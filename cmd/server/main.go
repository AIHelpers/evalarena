// Command server runs EvalArena's web mode: the REST API plus the static
// reviewer/dashboard frontend, for distributed teams reviewing via browser
// without installing anything.
package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	adapterhttp "evalarena/internal/adapter/http"
	"evalarena/internal/adapter/judge"
	"evalarena/internal/adapter/repository/jsonfile"
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
	}

	log.Printf("EvalArena server listening on %s (data dir: %s)", *addr, *dataDir)
	log.Fatal(http.ListenAndServe(*addr, srv.Routes()))
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
