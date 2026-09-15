// Command evalarena is the headless CLI: import/create arenas and export
// reports without the web server, for CI-triggered arena creation
// (e.g. "evalarena create --from-modelbench run123.json").
//
// Implemented with the standard flag package rather than cobra, since this
// sandbox's network policy has no route to the Go module proxy. The command
// surface (evalarena <verb> [flags]) mirrors what cobra would have produced.
package main

import (
	"fmt"
	"os"

	"evalarena/internal/adapter/importer"
	"evalarena/internal/adapter/repository/jsonfile"
	"evalarena/internal/domain"
	"evalarena/internal/infra"
	"evalarena/internal/usecase"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	cmd, args := os.Args[1], os.Args[2:]

	dataDir := envOr("EVALARENA_DATA", "./data")
	arenaRepo, err := jsonfile.NewArenaRepository(dataDir + "/arenas")
	must(err)
	random := infra.SystemRandom{}

	dsRepo, erRepo, closeFn, err := openEvalRepos(dataDir)
	must(err)
	defer closeFn()

	switch cmd {
	case "create":
		runCreate(args, arenaRepo, random)
	case "list":
		runList(args, arenaRepo)
	case "report":
		runReport(args, arenaRepo)
	case "results":
		runResults(args, arenaRepo)
	case "dataset":
		if len(args) < 1 {
			fatal("dataset: subcommand required (import)")
		}
		runDatasetImport(args[1:], dsRepo)
	case "eval":
		runEval(args, dsRepo, erRepo)
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`evalarena <command> [flags]

Commands:
  create --name NAME [--mode pairwise|tournament]
         [--from-modelbench FILE | --from-distillery FILE | --from-promptvault FILE]
      Create an arena, importing candidate outputs from an external tool's
      export file.

  list
      List all arenas.

  results --id ARENA_ID
      Print the win-rate report (overall + by category) as JSON.

  report --id ARENA_ID --out FILE.html
      Export a shareable static HTML results report.

  dataset import --file FILE.jsonl --name NAME
      Import a golden evaluation dataset.

  eval run --dataset DS_ID --model-label LABEL --responses FILE.jsonl
      Run an evaluation, print summary.
  eval list [--dataset DS_ID]
      List evaluation runs.
  eval show --id RUN_ID [--category CAT] [--failed-only]
      Show per-item results.
  eval compare --ids RUN1,RUN2 [--out FILE]
      Compare runs; export JSON/CSV/HTML.
  eval trend --dataset DS_ID [--out trend.csv]
      Per-run category scores ordered by time.`)
}

func runCreate(args []string, repo usecase.ArenaRepository, random usecase.RandomSource) {
	fs := newFlagSet("create")
	name := fs.String("name", "", "arena name (required)")
	mode := fs.String("mode", "pairwise", "pairwise | tournament")
	fromModelBench := fs.String("from-modelbench", "", "path to a ModelBench-Local run export JSON")
	fromDistillery := fs.String("from-distillery", "", "path to a Distillery job result export JSON")
	fromPromptVault := fs.String("from-promptvault", "", "path to a PromptVault comparison export JSON")
	must(fs.Parse(args))

	if *name == "" {
		fatal("create: --name is required")
	}

	var imp usecase.Importer
	var srcFile string
	switch {
	case *fromModelBench != "":
		imp, srcFile = importer.ModelBenchImporter{}, *fromModelBench
	case *fromDistillery != "":
		imp, srcFile = importer.DistilleryImporter{}, *fromDistillery
	case *fromPromptVault != "":
		imp, srcFile = importer.PromptVaultImporter{}, *fromPromptVault
	default:
		fatal("create: one of --from-modelbench / --from-distillery / --from-promptvault is required")
	}

	raw, err := os.ReadFile(srcFile)
	must(err)

	uc := usecase.NewImportFromSourceUseCase(repo, random, imp)
	arena, err := uc.Execute(*name, domain.Mode(*mode), raw)
	must(err)

	fmt.Printf("created arena %q (id=%s) with %d matchups\n", arena.Name, arena.ID, len(arena.Matchups))
}

func runList(args []string, repo usecase.ArenaRepository) {
	arenas, err := repo.List()
	must(err)
	for _, a := range arenas {
		fmt.Printf("%s\t%s\t%s\t%d matchups\n", a.ID, a.Mode, a.Name, len(a.Matchups))
	}
}

func runResults(args []string, repo usecase.ArenaRepository) {
	fs := newFlagSet("results")
	id := fs.String("id", "", "arena id (required)")
	must(fs.Parse(args))
	if *id == "" {
		fatal("results: --id is required")
	}
	uc := usecase.NewComputeWinRateUseCase(repo)
	report, err := uc.Execute(*id)
	must(err)
	fmt.Printf("Overall: A=%d B=%d tie=%d  A win rate=%.1f%% (95%% CI %.1f-%.1f%%)\n",
		report.Overall.AWins, report.Overall.BWins, report.Overall.Ties,
		report.Overall.AWinRate*100, report.Overall.WilsonLow*100, report.Overall.WilsonHigh*100)
	for _, c := range report.ByCategory {
		fmt.Printf("  [%s] A=%d B=%d tie=%d  A win rate=%.1f%%\n", c.Label, c.AWins, c.BWins, c.Ties, c.AWinRate*100)
	}
}

func runReport(args []string, repo usecase.ArenaRepository) {
	fs := newFlagSet("report")
	id := fs.String("id", "", "arena id (required)")
	out := fs.String("out", "report.html", "output HTML file path")
	must(fs.Parse(args))
	if *id == "" {
		fatal("report: --id is required")
	}
	uc := usecase.NewExportReportUseCase(repo)
	html, err := uc.Execute(*id)
	must(err)
	must(os.WriteFile(*out, html, 0o600))
	fmt.Printf("wrote %s\n", *out)
}
