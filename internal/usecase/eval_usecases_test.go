package usecase_test

import (
	"os"
	"path/filepath"
	"testing"

	"evalarena/internal/domain"
	"evalarena/internal/usecase"
)

const (
	testDatasetID = "ds-1"
	testItemMC    = "mc-1"
	testItemCode  = "code-1"
	testItemConc  = "concept-1"
)

// --- In-memory repos ---
type memRepos struct {
	ds []*domain.Dataset
	rs []*domain.EvalRun
}

func (m *memRepos) dsRepo() *memDSRepo   { return &memDSRepo{m} }
func (m *memRepos) runRepo() *memRunRepo { return &memRunRepo{m} }

type memDSRepo struct{ m *memRepos }

func (r *memDSRepo) Save(d *domain.Dataset) error { r.m.ds = append(r.m.ds, d); return nil }
func (r *memDSRepo) Get(id string) (*domain.Dataset, error) {
	for _, d := range r.m.ds {
		if d.ID == id {
			return d, nil
		}
	}
	return nil, os.ErrNotExist
}
func (r *memDSRepo) List() ([]*domain.Dataset, error) { return r.m.ds, nil }

type memRunRepo struct{ m *memRepos }

func (r *memRunRepo) Save(x *domain.EvalRun) error { r.m.rs = append(r.m.rs, x); return nil }
func (r *memRunRepo) Get(id string) (*domain.EvalRun, error) {
	for _, x := range r.m.rs {
		if x.ID == id {
			return x, nil
		}
	}
	return nil, os.ErrNotExist
}
func (r *memRunRepo) ListByDataset(id string) ([]*domain.EvalRun, error) {
	var out []*domain.EvalRun
	for _, x := range r.m.rs {
		if x.DatasetID == id {
			out = append(out, x)
		}
	}
	return out, nil
}
func (r *memRunRepo) ListAll() ([]*domain.EvalRun, error) { return r.m.rs, nil }

// --- Fakes ---
type fGofmt struct{ valid bool }

func (f fGofmt) Valid(string) (bool, string, error) { return f.valid, "", nil }

type fMC struct{ choice string }

func (f fMC) ParseChoice(string) (string, error) { return f.choice, nil }

type fRubric struct{ hits, total int }

func (f fRubric) Score(string, []string) (int, int) { return f.hits, f.total }

type fJudge struct {
	hits, total int
	rationale   string
	err         error
}

func (f fJudge) JudgeAgainstRubric(_, _ string, _ []string, _ string) (int, int, string, error) {
	return f.hits, f.total, f.rationale, f.err
}

// --- Helpers ---
func testDS() *domain.Dataset {
	return &domain.Dataset{ID: testDatasetID, Name: "t", Items: []domain.DatasetItem{
		{ID: testItemMC, Type: domain.ItemMultipleChoice, Category: "net", CorrectChoice: "A"},
		{ID: testItemCode, Type: domain.ItemCodeGeneration, Category: "conc", Rubric: []string{"mutex"}},
		{ID: testItemConc, Type: domain.ItemConceptual, Category: "theory", Rubric: []string{"atomicity"}},
	}}
}

func respFile(t *testing.T, s string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "r.jsonl")
	if err := os.WriteFile(p, []byte(s), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRunEval_MC_Correct(t *testing.T) {
	m := &memRepos{ds: []*domain.Dataset{testDS()}}
	uc := usecase.NewRunEvaluationUseCase(m.dsRepo(), m.runRepo(), fGofmt{}, fMC{choice: "A"}, fRubric{}, nil)
	run, err := uc.Execute(usecase.RunEvaluationInput{DatasetID: testDatasetID, ModelLabel: "m", ResponsesPath: respFile(t, `{"id":"`+testItemMC+`","response":"A"}`+"\n")})
	if err != nil {
		t.Fatal(err)
	}
	res := run.Results[0]
	if res.Score != 1 || res.MCChosen != "A" || res.MCCorrect == nil || !*res.MCCorrect {
		t.Errorf("MC correct mismatch: %+v", res)
	}
}

func TestRunEval_MC_Wrong(t *testing.T) {
	m := &memRepos{ds: []*domain.Dataset{testDS()}}
	uc := usecase.NewRunEvaluationUseCase(m.dsRepo(), m.runRepo(), fGofmt{}, fMC{choice: "B"}, fRubric{}, nil)
	run, err := uc.Execute(usecase.RunEvaluationInput{DatasetID: testDatasetID, ModelLabel: "m", ResponsesPath: respFile(t, `{"id":"`+testItemMC+`","response":"B"}`+"\n")})
	if err != nil {
		t.Fatal(err)
	}
	res := run.Results[0]
	if res.MCCorrect == nil || *res.MCCorrect || res.Score != 0 {
		t.Errorf("MC wrong mismatch: %+v", res)
	}
}

func TestRunEval_Code_CombinedScore(t *testing.T) {
	m := &memRepos{ds: []*domain.Dataset{testDS()}}
	uc := usecase.NewRunEvaluationUseCase(m.dsRepo(), m.runRepo(), fGofmt{valid: true}, fMC{}, fRubric{hits: 1, total: 1}, nil)
	run, err := uc.Execute(usecase.RunEvaluationInput{DatasetID: testDatasetID, ModelLabel: "m", ResponsesPath: respFile(t, `{"id":"`+testItemCode+`","response":"x"}`+"\n")})
	if err != nil {
		t.Fatal(err)
	}
	code := run.Results[1]
	// 0.4*1 + 0.6*(1/1) = 1.0
	if code.Score != 1.0 || code.GofmtValid == nil || !*code.GofmtValid {
		t.Errorf("code result: %+v", code)
	}
}

func TestRunEval_Conceptual_RubricOnly(t *testing.T) {
	m := &memRepos{ds: []*domain.Dataset{testDS()}}
	uc := usecase.NewRunEvaluationUseCase(m.dsRepo(), m.runRepo(), fGofmt{}, fMC{}, fRubric{hits: 1, total: 2}, nil)
	run, err := uc.Execute(usecase.RunEvaluationInput{DatasetID: testDatasetID, ModelLabel: "m", ResponsesPath: respFile(t, `{"id":"`+testItemConc+`","response":"x"}`+"\n")})
	if err != nil {
		t.Fatal(err)
	}
	c := run.Results[2]
	if c.Score != 0.5 || c.RubricHits != 1 || c.RubricTotal != 2 {
		t.Errorf("concept result: %+v", c)
	}
}

func TestRunEval_WithJudge(t *testing.T) {
	m := &memRepos{ds: []*domain.Dataset{testDS()}}
	uc := usecase.NewRunEvaluationUseCase(m.dsRepo(), m.runRepo(), fGofmt{}, fMC{}, fRubric{}, fJudge{hits: 2, total: 2, rationale: "g"})
	run, err := uc.Execute(usecase.RunEvaluationInput{DatasetID: testDatasetID, ModelLabel: "m", ResponsesPath: respFile(t, `{"id":"`+testItemCode+`","response":"x"}`+"\n"), UseRubricJudge: true})
	if err != nil {
		t.Fatal(err)
	}
	c := run.Results[1]
	if c.RubricHits != 2 || c.RubricTotal != 2 || c.JudgeRationale != "g" {
		t.Errorf("judge result: %+v", c)
	}
}

func TestRunEval_JudgeFallsBackToHeuristic(t *testing.T) {
	m := &memRepos{ds: []*domain.Dataset{testDS()}}
	uc := usecase.NewRunEvaluationUseCase(m.dsRepo(), m.runRepo(), fGofmt{}, fMC{}, fRubric{hits: 1, total: 1}, fJudge{err: os.ErrNotExist})
	run, err := uc.Execute(usecase.RunEvaluationInput{DatasetID: testDatasetID, ModelLabel: "m", ResponsesPath: respFile(t, `{"id":"`+testItemCode+`","response":"x"}`+"\n"), UseRubricJudge: true})
	if err != nil {
		t.Fatal(err)
	}
	c := run.Results[1]
	if c.RubricHits != 1 || c.RubricTotal != 1 {
		t.Errorf("heuristic fallback: %+v", c)
	}
}

func TestRunEval_UnknownTypeScoresZero(t *testing.T) {
	ds := testDS()
	ds.Items[0].Type = "bogus"
	m := &memRepos{ds: []*domain.Dataset{ds}}
	uc := usecase.NewRunEvaluationUseCase(m.dsRepo(), m.runRepo(), fGofmt{}, fMC{}, fRubric{}, nil)
	run, err := uc.Execute(usecase.RunEvaluationInput{DatasetID: testDatasetID, ModelLabel: "m", ResponsesPath: respFile(t, `{"id":"`+testItemMC+`","response":"x"}`+"\n")})
	if err != nil {
		t.Fatal(err)
	}
	if run.Results[0].Score != 0 {
		t.Errorf("unknown type score: %v", run.Results[0].Score)
	}
}

func TestRunEval_MissingResponseScoresZero(t *testing.T) {
	m := &memRepos{ds: []*domain.Dataset{testDS()}}
	uc := usecase.NewRunEvaluationUseCase(m.dsRepo(), m.runRepo(), fGofmt{}, fMC{}, fRubric{}, nil)
	run, err := uc.Execute(usecase.RunEvaluationInput{DatasetID: testDatasetID, ModelLabel: "m", ResponsesPath: respFile(t, "")})
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Results) != 3 || run.Results[0].Score != 0 {
		t.Errorf("missing response: %+v", run.Results)
	}
}

func TestRunEval_MissingResponsesFile(t *testing.T) {
	m := &memRepos{ds: []*domain.Dataset{testDS()}}
	uc := usecase.NewRunEvaluationUseCase(m.dsRepo(), m.runRepo(), fGofmt{}, fMC{}, fRubric{}, nil)
	if _, err := uc.Execute(usecase.RunEvaluationInput{DatasetID: testDatasetID, ModelLabel: "m", ResponsesPath: filepath.Join(t.TempDir(), "none.jsonl")}); err == nil {
		t.Error("expected error for missing responses file")
	}
}

func TestCompareRuns_Deltas(t *testing.T) {
	m := &memRepos{ds: []*domain.Dataset{testDS()}, rs: []*domain.EvalRun{
		{ID: "a", DatasetID: testDatasetID, ModelLabel: "v1", Results: []domain.ItemResult{
			{ItemID: testItemMC, Score: 1.0}, {ItemID: testItemCode, Score: 0.9}, {ItemID: testItemConc, Score: 0.5}}},
		{ID: "b", DatasetID: testDatasetID, ModelLabel: "v2", Results: []domain.ItemResult{
			{ItemID: testItemMC, Score: 1.0}, {ItemID: testItemCode, Score: 0.4}, {ItemID: testItemConc, Score: 1.0}}},
	}}
	uc := usecase.NewCompareEvalRunsUseCase(m.dsRepo(), m.runRepo())
	cmp, err := uc.Execute([]string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if len(cmp.Regressions) != 1 || cmp.Regressions[0].ItemID != testItemCode || cmp.Regressions[0].Delta != -0.5 {
		t.Errorf("regressions: %+v", cmp.Regressions)
	}
	if len(cmp.Improvements) != 1 || cmp.Improvements[0].ItemID != testItemConc {
		t.Errorf("improvements: %+v", cmp.Improvements)
	}
	if len(cmp.RunIDs) != 2 || cmp.ModelLabels[0] != "v1" || cmp.ModelLabels[1] != "v2" {
		t.Errorf("labels: %+v", cmp.ModelLabels)
	}
}

func TestCompareRuns_TooFew(t *testing.T) {
	uc := usecase.NewCompareEvalRunsUseCase(&memDSRepo{}, &memRunRepo{})
	if _, err := uc.Execute([]string{"only"}); err == nil {
		t.Error("expected error for single run")
	}
}
