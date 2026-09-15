package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"evalarena/internal/adapter/judge"
	"evalarena/internal/adapter/repository/jsonfile"
	"evalarena/internal/adapter/repository/sqlite"
	"evalarena/internal/adapter/scorer"
	"evalarena/internal/domain"
	"evalarena/internal/usecase"
)

// openEvalRepos constructs dataset + evalrun repositories from the backend
// env var (default jsonfile; sqlite if EVALARENA_DB_BACKEND=sqlite).
func openEvalRepos(dataDir string) (usecase.DatasetRepository, usecase.EvalRunRepository, func(), error) {
	backend := envOr("EVALARENA_DB_BACKEND", "jsonfile")
	if backend == "sqlite" {
		db, err := sqlite.Open(dataDir + "/evalarena.db")
		if err != nil {
			return nil, nil, nil, err
		}
		return sqlite.NewDatasetRepository(db), sqlite.NewEvalRunRepository(db), func() { _ = db.Close() }, nil
	}
	ds, err := jsonfile.NewDatasetRepository(dataDir + "/datasets")
	if err != nil {
		return nil, nil, nil, err
	}
	er, err := jsonfile.NewEvalRunRepository(dataDir + "/evalruns")
	if err != nil {
		return nil, nil, nil, err
	}
	return ds, er, func() {}, nil
}

func runEval(args []string, ds usecase.DatasetRepository, er usecase.EvalRunRepository) {
	if len(args) < 1 {
		fatal("eval: subcommand required (run|list|show|compare|trend)")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "run":
		runEvalRun(rest, ds, er)
	case "list":
		runEvalList(rest, er)
	case "show":
		runEvalShow(rest, ds, er)
	case "compare":
		runEvalCompare(rest, ds, er)
	case "trend":
		runEvalTrend(rest, ds, er)
	default:
		fatal("eval: unknown subcommand " + sub)
	}
}

func runDatasetImport(args []string, ds usecase.DatasetRepository) {
	fs := newFlagSet("dataset import")
	file := fs.String("file", "", "path to golden_eval_set.jsonl (required)")
	name := fs.String("name", "", "dataset name (required)")
	must(fs.Parse(args))
	if *file == "" || *name == "" {
		fatal("dataset import: --file and --name are required")
	}
	uc := usecase.NewImportDatasetUseCase(ds)
	d, err := uc.Execute(*name, *file)
	must(err)
	fmt.Printf("imported dataset %q (id=%s) with %d items\n", d.Name, d.ID, len(d.Items))
}

func runEvalRun(args []string, ds usecase.DatasetRepository, er usecase.EvalRunRepository) {
	fs := newFlagSet("eval run")
	dataset := fs.String("dataset", "", "dataset id (required)")
	label := fs.String("model-label", "", "model label (required)")
	responses := fs.String("responses", "", "path to {id,response} JSONL (required)")
	useJudge := fs.Bool("rubric-judge", false, "use LLM rubric judge if configured")
	notes := fs.String("notes", "", "optional run notes")
	must(fs.Parse(args))
	if *dataset == "" || *label == "" || *responses == "" {
		fatal("eval run: --dataset, --model-label, --responses are required")
	}
	j := judge.NewAnthropicRubricJudge(envOr("ANTHROPIC_MODEL", "claude-sonnet-4-6"))
	uc := usecase.NewRunEvaluationUseCase(ds, er, scorer.GofmtScorer{}, scorer.MCScorer{}, scorer.RubricHeuristicScorer{}, j)
	run, err := uc.Execute(usecase.RunEvaluationInput{
		DatasetID:      *dataset,
		ModelLabel:     *label,
		ResponsesPath:  *responses,
		UseRubricJudge: *useJudge,
		Notes:          *notes,
	})
	must(err)
	su := usecase.NewComputeEvalSummaryUseCase(ds, er)
	summary, err := su.Execute(run.ID)
	must(err)
	fmt.Printf("evaluation run %q (id=%s) overall=%.2f n=%d\n", run.ModelLabel, run.ID, summary.Overall, summary.N)
	for cat, sc := range summary.ByCategory {
		fmt.Printf("  [%s] %.2f\n", cat, sc)
	}
}

func runEvalList(args []string, er usecase.EvalRunRepository) {
	fs := newFlagSet("eval list")
	dataset := fs.String("dataset", "", "filter by dataset id")
	must(fs.Parse(args))
	var runs []*domain.EvalRun
	var err error
	if *dataset != "" {
		runs, err = er.ListByDataset(*dataset)
	} else {
		runs, err = er.ListAll()
	}
	must(err)
	for _, r := range runs {
		fmt.Printf("%s\t%s\t%s\t%s\t%s\n", r.ID, r.ModelLabel, r.DatasetID, r.CreatedAt.Format("2006-01-02 15:04"), r.ScorerMode)
	}
}

func runEvalShow(args []string, ds usecase.DatasetRepository, er usecase.EvalRunRepository) {
	fs := newFlagSet("eval show")
	id := fs.String("id", "", "run id (required)")
	category := fs.String("category", "", "filter by category")
	failedOnly := fs.Bool("failed-only", false, "show only failing items")
	threshold := fs.Float64("threshold", 0.5, "failure threshold for --failed-only")
	must(fs.Parse(args))
	if *id == "" {
		fatal("eval show: --id is required")
	}
	run, err := er.Get(*id)
	must(err)
	dsObj, err := ds.Get(run.DatasetID)
	must(err)
	items := map[string]domain.DatasetItem{}
	for _, it := range dsObj.Items {
		items[it.ID] = it
	}
	for _, res := range run.Results {
		it := items[res.ItemID]
		if *category != "" && it.Category != *category {
			continue
		}
		if *failedOnly && res.Score >= *threshold {
			continue
		}
		fmt.Printf("%s\t%s\t%.2f\t%s\n", res.ItemID, it.Category, res.Score, res.Response)
	}
}

func runEvalCompare(args []string, ds usecase.DatasetRepository, er usecase.EvalRunRepository) {
	fs := newFlagSet("eval compare")
	ids := fs.String("ids", "", "comma-separated run ids (required)")
	out := fs.String("out", "", "export to FILE.json/.csv/.html")
	must(fs.Parse(args))
	if *ids == "" {
		fatal("eval compare: --ids is required")
	}
	runIDs := strings.Split(*ids, ",")
	for i := range runIDs {
		runIDs[i] = strings.TrimSpace(runIDs[i])
	}
	uc := usecase.NewCompareEvalRunsUseCase(ds, er)
	cmp, err := uc.Execute(runIDs)
	must(err)

	if *out == "" {
		b, _ := json.MarshalIndent(cmp, "", "  ")
		fmt.Println(string(b))
		return
	}
	switch {
	case strings.HasSuffix(*out, ".json"):
		b, _ := json.MarshalIndent(cmp, "", "  ")
		must(os.WriteFile(*out, b, 0o600))
	case strings.HasSuffix(*out, ".csv"):
		must(writeCompareCSV(*out, cmp))
	case strings.HasSuffix(*out, ".html"):
		must(writeCompareHTML(*out, cmp))
	default:
		fatal("eval compare: --out must end in .json, .csv, or .html")
	}
	fmt.Printf("wrote %s\n", *out)
}

func writeCompareCSV(path string, cmp *usecase.RunComparison) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	w := csv.NewWriter(f)
	defer w.Flush()
	_ = w.Write([]string{"category", "run_id", "score"})
	var cats []string
	for c := range cmp.CategoryDelta {
		cats = append(cats, c)
	}
	sort.Strings(cats)
	for _, c := range cats {
		for _, rid := range cmp.RunIDs {
			sc := cmp.CategoryDelta[c][rid]
			_ = w.Write([]string{c, rid, fmt.Sprintf("%.4f", sc)})
		}
	}
	return w.Error()
}

func writeCompareHTML(path string, cmp *usecase.RunComparison) error {
	var sb strings.Builder
	sb.WriteString("<!DOCTYPE html><html><head><meta charset='utf-8'><title>Eval Compare</title></head><body><h1>EvalArena Run Comparison</h1>")
	sb.WriteString("<h2>Overall</h2><table border=1><tr><th>Run</th><th>Label</th><th>Overall</th></tr>")
	for i, rid := range cmp.RunIDs {
		label := ""
		if i < len(cmp.ModelLabels) {
			label = cmp.ModelLabels[i]
		}
		fmt.Fprintf(&sb, "<tr><td>%s</td><td>%s</td><td>%.4f</td></tr>", rid, label, cmp.OverallByRun[rid])
	}
	sb.WriteString("</table><h2>Regressions</h2><table border=1><tr><th>Item</th><th>Category</th><th>Score A</th><th>Score B</th><th>Delta</th></tr>")
	for _, d := range cmp.Regressions {
		fmt.Fprintf(&sb, "<tr><td>%s</td><td>%s</td><td>%.4f</td><td>%.4f</td><td style='color:red'>%.4f</td></tr>", d.ItemID, d.Category, d.ScoreA, d.ScoreB, d.Delta)
	}
	sb.WriteString("</table><h2>Improvements</h2><table border=1><tr><th>Item</th><th>Category</th><th>Score A</th><th>Score B</th><th>Delta</th></tr>")
	for _, d := range cmp.Improvements {
		fmt.Fprintf(&sb, "<tr><td>%s</td><td>%s</td><td>%.4f</td><td>%.4f</td><td style='color:green'>%.4f</td></tr>", d.ItemID, d.Category, d.ScoreA, d.ScoreB, d.Delta)
	}
	sb.WriteString("</table></body></html>")
	return os.WriteFile(path, []byte(sb.String()), 0o600)
}

func runEvalTrend(args []string, ds usecase.DatasetRepository, er usecase.EvalRunRepository) {
	fs := newFlagSet("eval trend")
	dataset := fs.String("dataset", "", "dataset id (required)")
	out := fs.String("out", "trend.csv", "output CSV path")
	must(fs.Parse(args))
	if *dataset == "" {
		fatal("eval trend: --dataset is required")
	}
	runs, err := er.ListByDataset(*dataset)
	must(err)
	sort.Slice(runs, func(i, j int) bool { return runs[i].CreatedAt.Before(runs[j].CreatedAt) })

	f, err := os.Create(*out)
	must(err)
	defer func() { _ = f.Close() }()
	w := csv.NewWriter(f)
	defer w.Flush()

	dsObj, err := ds.Get(*dataset)
	must(err)
	var cats []string
	for _, it := range dsObj.Items {
		if !contains(cats, it.Category) {
			cats = append(cats, it.Category)
		}
	}
	sort.Strings(cats)

	header := make([]string, 0, 3+len(cats))
	header = append(header, "model_label", "created_at", "overall")
	header = append(header, cats...)
	_ = w.Write(header)

	su := usecase.NewComputeEvalSummaryUseCase(ds, er)
	for _, r := range runs {
		s, err := su.Execute(r.ID)
		must(err)
		row := make([]string, 0, 3+len(cats))
		row = append(row, r.ModelLabel, r.CreatedAt.Format("2006-01-02 15:04"), fmt.Sprintf("%.4f", s.Overall))
		for _, c := range cats {
			row = append(row, fmt.Sprintf("%.4f", s.ByCategory[c]))
		}
		_ = w.Write(row)
	}
	must(w.Error())
	fmt.Printf("wrote %s\n", *out)
}

func contains(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}
