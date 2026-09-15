package usecase

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"evalarena/internal/domain"
)

// ImportDatasetUseCase imports a golden_eval_set.jsonl into a Dataset.
type ImportDatasetUseCase struct {
	Datasets DatasetRepository
}

func NewImportDatasetUseCase(d DatasetRepository) *ImportDatasetUseCase {
	return &ImportDatasetUseCase{Datasets: d}
}

func (uc *ImportDatasetUseCase) Execute(name, path string) (*domain.Dataset, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var items []domain.DatasetItem
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var it domain.DatasetItem
		if err := json.Unmarshal([]byte(line), &it); err != nil {
			return nil, fmt.Errorf("parse item: %w", err)
		}
		if it.ID == "" {
			return nil, fmt.Errorf("item missing id")
		}
		items = append(items, it)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	ds := &domain.Dataset{
		ID:         "ds-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		Name:       name,
		Version:    "1.0",
		SourcePath: path,
		ImportedAt: time.Now().UTC(),
		Items:      items,
	}
	if err := uc.Datasets.Save(ds); err != nil {
		return nil, err
	}
	return ds, nil
}

// RunEvaluationInput configures an evaluation run.
type RunEvaluationInput struct {
	DatasetID      string
	ModelLabel     string
	ResponsesPath  string            // {id, response} JSONL (CLI)
	Responses      map[string]string // inline itemID -> response (web UI)
	UseRubricJudge bool
	Notes          string
}

// RunEvaluationUseCase is the core new capability: grade a model's outputs
// against a golden dataset.
type RunEvaluationUseCase struct {
	Datasets DatasetRepository
	Runs     EvalRunRepository
	Gofmt    GofmtScorer
	MC       MCScorer
	Rubric   RubricScorer
	Judge    RubricJudgeModel // optional, may be nil
}

func NewRunEvaluationUseCase(d DatasetRepository, r EvalRunRepository, g GofmtScorer, m MCScorer, rb RubricScorer, j RubricJudgeModel) *RunEvaluationUseCase {
	return &RunEvaluationUseCase{Datasets: d, Runs: r, Gofmt: g, MC: m, Rubric: rb, Judge: j}
}

type responsePair struct {
	ID       string `json:"id"`
	Response string `json:"response"`
}

func (uc *RunEvaluationUseCase) Execute(in RunEvaluationInput) (*domain.EvalRun, error) {
	ds, err := uc.Datasets.Get(in.DatasetID)
	if err != nil {
		return nil, err
	}
	// Two ways to supply responses: inline map (web UI) or JSONL file (CLI).
	var byID map[string]string
	if in.Responses != nil {
		byID = in.Responses
	} else {
		responses, err := loadResponses(in.ResponsesPath)
		if err != nil {
			return nil, err
		}
		byID = map[string]string{}
		for _, r := range responses {
			byID[r.ID] = r.Response
		}
	}

	itemsByID := map[string]domain.DatasetItem{}
	for _, it := range ds.Items {
		itemsByID[it.ID] = it
	}

	scorerMode := domain.ScorerAutomated
	run := &domain.EvalRun{
		ID:         "run-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		DatasetID:  ds.ID,
		ModelLabel: in.ModelLabel,
		ScorerMode: scorerMode,
		CreatedAt:  time.Now().UTC(),
		Notes:      in.Notes,
	}

	now := time.Now().UTC()
	for _, it := range ds.Items {
		response := byID[it.ID]
		res := domain.ItemResult{
			ItemID:     it.ID,
			Response:   response,
			ScorerType: scorerMode,
			ScoredAt:   now,
		}

		switch it.Type {
		case domain.ItemMultipleChoice:
			chosen, perr := uc.MC.ParseChoice(response)
			res.MCChosen = chosen
			if perr != nil {
				res.Score = 0
				run.Results = append(run.Results, res)
				continue
			}
			correct := chosen == strings.ToUpper(it.CorrectChoice)
			res.MCCorrect = &correct
			if correct {
				res.Score = 1
				res.RubricHits, res.RubricTotal = 1, 1
			}
		case domain.ItemCodeGeneration, domain.ItemBugFix:
			valid, _, gerr := uc.Gofmt.Valid(response)
			if gerr != nil {
				res.GofmtValid = nil // unavailable
			} else {
				g := valid
				res.GofmtValid = &g
			}
			applyRubric(uc, &res, it, response, in.UseRubricJudge)
			res.Score = domain.CombinedScore(valid, res.RubricHits, res.RubricTotal)
		case domain.ItemConceptual:
			applyRubric(uc, &res, it, response, in.UseRubricJudge)
			if res.RubricTotal > 0 {
				res.Score = float64(res.RubricHits) / float64(res.RubricTotal)
			}
		default:
			res.Score = 0
		}
		run.Results = append(run.Results, res)
	}

	if err := uc.Runs.Save(run); err != nil {
		return nil, err
	}
	return run, nil
}

func applyRubric(uc *RunEvaluationUseCase, res *domain.ItemResult, it domain.DatasetItem, response string, useJudge bool) {
	if useJudge && uc.Judge != nil {
		hits, total, rationale, err := uc.Judge.JudgeAgainstRubric(it.Question, it.ReferenceAnswer, it.Rubric, response)
		if err == nil {
			res.RubricHits, res.RubricTotal, res.JudgeRationale = hits, total, rationale
			return
		}
		// fall through to heuristic on judge error
	}
	res.RubricHits, res.RubricTotal = uc.Rubric.Score(response, it.Rubric)
}

func loadResponses(path string) ([]responsePair, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var out []responsePair
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var p responsePair
		if err := json.Unmarshal([]byte(line), &p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, sc.Err()
}

// ComputeEvalSummaryUseCase returns the summary for a run.
type ComputeEvalSummaryUseCase struct {
	Datasets DatasetRepository
	Runs     EvalRunRepository
}

func NewComputeEvalSummaryUseCase(d DatasetRepository, r EvalRunRepository) *ComputeEvalSummaryUseCase {
	return &ComputeEvalSummaryUseCase{Datasets: d, Runs: r}
}

func (uc *ComputeEvalSummaryUseCase) Execute(runID string) (*domain.RunSummary, error) {
	run, err := uc.Runs.Get(runID)
	if err != nil {
		return nil, err
	}
	ds, err := uc.Datasets.Get(run.DatasetID)
	if err != nil {
		return nil, err
	}
	items := map[string]domain.DatasetItem{}
	for _, it := range ds.Items {
		items[it.ID] = it
	}
	s := domain.Summarize(run.Results, items)
	return &s, nil
}

// RunComparison combines results across multiple evaluation runs.
type RunComparison struct {
	RunIDs        []string                      `json:"run_ids"`
	ModelLabels   []string                      `json:"model_labels"`
	OverallByRun  map[string]float64            `json:"overall_by_run"`
	CategoryDelta map[string]map[string]float64 `json:"category_delta"`
	Regressions   []ItemDelta                   `json:"regressions"`
	Improvements  []ItemDelta                   `json:"improvements"`
}

type ItemDelta struct {
	ItemID   string  `json:"item_id"`
	Category string  `json:"category"`
	ScoreA   float64 `json:"score_a"`
	ScoreB   float64 `json:"score_b"`
	Delta    float64 `json:"delta"`
}

// CompareEvalRunsUseCase compares run[0] against run[last].
type CompareEvalRunsUseCase struct {
	Datasets DatasetRepository
	Runs     EvalRunRepository
}

func NewCompareEvalRunsUseCase(d DatasetRepository, r EvalRunRepository) *CompareEvalRunsUseCase {
	return &CompareEvalRunsUseCase{Datasets: d, Runs: r}
}

func (uc *CompareEvalRunsUseCase) Execute(runIDs []string) (*RunComparison, error) {
	if len(runIDs) < 2 {
		return nil, fmt.Errorf("compare needs at least 2 run ids, got %d", len(runIDs))
	}

	runs := make([]*domain.EvalRun, 0, len(runIDs))
	var runA, runB *domain.EvalRun
	for i, id := range runIDs {
		run, err := uc.Runs.Get(id)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
		if i == 0 {
			runA = run
		}
		if i == len(runIDs)-1 {
			runB = run
		}
	}

	// Summarize each run's overall and by-category for the delta view.
	overallByRun := map[string]float64{}
	categoryDelta := map[string]map[string]float64{}
	labels := make([]string, 0, len(runs))
	for _, run := range runs {
		labels = append(labels, run.ModelLabel)
		ds, err := uc.Datasets.Get(run.DatasetID)
		if err != nil {
			return nil, err
		}
		items := map[string]domain.DatasetItem{}
		for _, it := range ds.Items {
			items[it.ID] = it
		}
		s := domain.Summarize(run.Results, items)
		overallByRun[run.ID] = s.Overall
		for cat, score := range s.ByCategory {
			if categoryDelta[cat] == nil {
				categoryDelta[cat] = map[string]float64{}
			}
			categoryDelta[cat][run.ID] = score
		}
	}

	// Per-item deltas between runA (first) and runB (last).
	scoresA := map[string]float64{}
	catA := map[string]string{}
	for _, res := range runA.Results {
		scoresA[res.ItemID] = res.Score
		it := lookupItem(uc, runA.DatasetID, res.ItemID)
		catA[res.ItemID] = it.Category
	}

	var regressions, improvements []ItemDelta
	for _, res := range runB.Results {
		a, ok := scoresA[res.ItemID]
		if !ok {
			continue
		}
		delta := ItemDelta{
			ItemID:   res.ItemID,
			Category: catA[res.ItemID],
			ScoreA:   a,
			ScoreB:   res.Score,
			Delta:    res.Score - a,
		}
		if delta.Delta < 0 {
			regressions = append(regressions, delta)
		} else if delta.Delta > 0 {
			improvements = append(improvements, delta)
		}
	}
	sort.Slice(regressions, func(i, j int) bool { return regressions[i].Delta < regressions[j].Delta })
	sort.Slice(improvements, func(i, j int) bool { return improvements[i].Delta > improvements[j].Delta })

	return &RunComparison{
		RunIDs:        runIDs,
		ModelLabels:   labels,
		OverallByRun:  overallByRun,
		CategoryDelta: categoryDelta,
		Regressions:   regressions,
		Improvements:  improvements,
	}, nil
}

func lookupItem(uc *CompareEvalRunsUseCase, datasetID, itemID string) domain.DatasetItem {
	ds, err := uc.Datasets.Get(datasetID)
	if err != nil {
		return domain.DatasetItem{}
	}
	for _, it := range ds.Items {
		if it.ID == itemID {
			return it
		}
	}
	return domain.DatasetItem{}
}
