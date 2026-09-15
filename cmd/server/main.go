// Command server runs EvalArena's web mode: the REST API plus the static
// reviewer/dashboard frontend, for distributed teams reviewing via browser
// without installing anything.
package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	adapterhttp "evalarena/internal/adapter/http"
	"evalarena/internal/adapter/judge"
	"evalarena/internal/adapter/repository/jsonfile"
	"evalarena/internal/adapter/repository/sqlite"
	"evalarena/internal/adapter/scorer"
	"evalarena/internal/infra"
	"evalarena/internal/usecase"
)

func main() {
	addr := flag.String("addr", envOr("EVALARENA_ADDR", ":8080"), "listen address")
	dataDir := flag.String("data", envOr("EVALARENA_DATA", "./data"), "data directory")
	judgeModel := flag.String("judge-model", envOr("EVALARENA_JUDGE_MODEL", ""), "judge model id (empty disables the LLM-judge pre-pass)")
	flag.Parse()

	arenaRepo, err := jsonfile.NewArenaRepository(*dataDir + "/arenas")
	if err != nil {
		log.Fatalf("init arena repo: %v", err)
	}
	reviewerRepo, err := jsonfile.NewReviewerRepository(*dataDir + "/reviewers")
	if err != nil {
		log.Fatalf("init reviewer repo: %v", err)
	}
	dsRepo, erRepo, closeEvalDB, err := openEvalRepos(*dataDir)
	if err != nil {
		log.Fatalf("init eval repos: %v", err)
	}
	defer closeEvalDB()

	random := infra.SystemRandom{}
	judgeModelImpl := judge.NewAnthropicJudge(*judgeModel)

	srv := &adapterhttp.Server{
		Repo:         arenaRepo,
		CreateArena:  usecase.NewCreateArenaUseCase(arenaRepo, random),
		CastVote:     usecase.NewCastVoteUseCase(arenaRepo),
		GetForReview: usecase.NewGetMatchupForReviewUseCase(arenaRepo, random),
		WinRate:      usecase.NewComputeWinRateUseCase(arenaRepo),
		Elo:          usecase.NewComputeEloRankingUseCase(arenaRepo),
		JudgePrepass: usecase.NewRunJudgePrepassUseCase(arenaRepo, judgeModelImpl),
		Calibrate:    usecase.NewCalibrateReviewersUseCase(arenaRepo, reviewerRepo),
		ExportReport: usecase.NewExportReportUseCase(arenaRepo),

		// Dataset evaluation (Phase 6 dashboard).
		Datasets:    dsRepo,
		EvalRuns:    erRepo,
		EvalSummary: usecase.NewComputeEvalSummaryUseCase(dsRepo, erRepo),
		EvalCompare: usecase.NewCompareEvalRunsUseCase(dsRepo, erRepo),
		RunEval: usecase.NewRunEvaluationUseCase(
			dsRepo, erRepo,
			scorer.GofmtScorer{}, scorer.MCScorer{}, scorer.RubricHeuristicScorer{},
			judge.NewAnthropicRubricJudge(envOr("EVALARENA_JUDGE_MODEL", "claude-sonnet-4-6")),
		),
	}

	srvHTTP := &http.Server{
		Addr:              *addr,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("EvalArena server listening on %s (data dir: %s)", *addr, *dataDir)
	log.Fatal(srvHTTP.ListenAndServe())
}

// openEvalRepos constructs dataset + evalrun repositories from the backend
// env var (default jsonfile; sqlite if EVALARENA_DB_BACKEND=sqlite). Mirrors
// the CLI's wiring so the server dashboard reads the same data.
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

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
