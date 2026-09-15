package repository_test

import (
	"testing"
	"time"

	"evalarena/internal/adapter/repository/jsonfile"
	"evalarena/internal/adapter/repository/sqlite"
	"evalarena/internal/domain"
)

// TestRepositoryConformance runs a table of behavioral checks against both
// the jsonfile and sqlite backend implementations of each repository port,
// asserting both satisfy literally the same contract.
func TestRepositoryConformance(t *testing.T) {
	t.Run("jsonfile", func(t *testing.T) {
		dir := t.TempDir()
		arenaRepo, err := jsonfile.NewArenaRepository(dir + "/arenas")
		must(t, err)
		reviewerRepo, err := jsonfile.NewReviewerRepository(dir + "/reviewers")
		must(t, err)
		datasetRepo, err := jsonfile.NewDatasetRepository(dir + "/datasets")
		must(t, err)
		evalRunRepo, err := jsonfile.NewEvalRunRepository(dir + "/evalruns")
		must(t, err)

		testArenaRepo(t, arenaRepo)
		testReviewerRepo(t, reviewerRepo)
		testDatasetRepo(t, datasetRepo)
		testEvalRunRepo(t, evalRunRepo)
	})

	t.Run("sqlite", func(t *testing.T) {
		db, err := sqlite.Open(t.TempDir() + "/test.db")
		must(t, err)
		defer func() { _ = db.Close() }()

		arenaRepo := sqlite.NewArenaRepository(db)
		reviewerRepo := sqlite.NewReviewerRepository(db)
		datasetRepo := sqlite.NewDatasetRepository(db)
		evalRunRepo := sqlite.NewEvalRunRepository(db)

		testArenaRepo(t, arenaRepo)
		testReviewerRepo(t, reviewerRepo)
		testDatasetRepo(t, datasetRepo)
		testEvalRunRepo(t, evalRunRepo)
	})
}

func testArenaRepo(t *testing.T, repo interface {
	Save(*domain.Arena) error
	Get(string) (*domain.Arena, error)
	List() ([]*domain.Arena, error)
	Delete(string) error
}) {
	t.Helper()

	a1 := &domain.Arena{
		ID:        "arena-1",
		Name:      "test arena",
		Mode:      domain.ModePairwise,
		CreatedAt: time.Now().UTC(),
		Matchups: []domain.Matchup{{
			ID:         "m1",
			Prompt:     "prompt",
			CandidateA: domain.CandidateOutput{SourceLabel: "a", Text: "text a"},
			CandidateB: domain.CandidateOutput{SourceLabel: "b", Text: "text b"},
			HumanVotes: []domain.Vote{{
				ReviewerID: "reviewer1",
				Winner:     domain.WinnerA,
				IsJudge:    false,
				CastAt:     time.Now().UTC(),
			}},
		}},
	}
	must(t, repo.Save(a1))

	got, err := repo.Get("arena-1")
	must(t, err)
	if got.ID != "arena-1" || got.Name != "test arena" || len(got.Matchups) != 1 {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if got.Matchups[0].HumanVotes[0].Winner != domain.WinnerA {
		t.Errorf("vote round-trip mismatch: got %v, want %v", got.Matchups[0].HumanVotes[0].Winner, domain.WinnerA)
	}

	list, err := repo.List()
	must(t, err)
	if len(list) != 1 {
		t.Errorf("List() len = %d, want 1", len(list))
	}

	must(t, repo.Delete("arena-1"))
	if _, err := repo.Get("arena-1"); err == nil {
		t.Error("Get after Delete should error")
	}
}

func testReviewerRepo(t *testing.T, repo interface {
	Save(*domain.Reviewer) error
	Get(string) (*domain.Reviewer, error)
	List() ([]*domain.Reviewer, error)
}) {
	t.Helper()

	r1 := &domain.Reviewer{ID: "rev-1", DisplayName: "Alice", VotesCast: 3, AgreementRate: 0.8}
	must(t, repo.Save(r1))

	got, err := repo.Get("rev-1")
	must(t, err)
	if got.DisplayName != "Alice" || got.VotesCast != 3 {
		t.Errorf("reviewer round-trip mismatch: %+v", got)
	}

	list, err := repo.List()
	must(t, err)
	if len(list) != 1 {
		t.Errorf("reviewer List() len = %d, want 1", len(list))
	}
}

func testDatasetRepo(t *testing.T, repo interface {
	Save(*domain.Dataset) error
	Get(string) (*domain.Dataset, error)
	List() ([]*domain.Dataset, error)
}) {
	t.Helper()

	ds := &domain.Dataset{
		ID:         "ds-1",
		Name:       "golden-v1",
		Version:    "1.0",
		SourcePath: "golden_eval_set.jsonl",
		ImportedAt: time.Now().UTC(),
		Items: []domain.DatasetItem{{
			ID:              "eval-001",
			Type:            domain.ItemCodeGeneration,
			Category:        "concurrency",
			Difficulty:      "hard",
			Question:        "Write a goroutine-safe counter.",
			ReferenceAnswer: "Use a mutex.",
			Rubric:          []string{"uses mutex"},
		}},
	}
	must(t, repo.Save(ds))

	got, err := repo.Get("ds-1")
	must(t, err)
	if got.Name != "golden-v1" || len(got.Items) != 1 {
		t.Fatalf("dataset round-trip mismatch: %+v", got)
	}
	if got.Items[0].Rubric[0] != "uses mutex" {
		t.Errorf("rubric round-trip mismatch: %v", got.Items[0].Rubric)
	}

	list, err := repo.List()
	must(t, err)
	if len(list) != 1 {
		t.Errorf("dataset List() len = %d, want 1", len(list))
	}
}

func testEvalRunRepo(t *testing.T, repo interface {
	Save(*domain.EvalRun) error
	Get(string) (*domain.EvalRun, error)
	ListByDataset(string) ([]*domain.EvalRun, error)
	ListAll() ([]*domain.EvalRun, error)
}) {
	t.Helper()

	run := &domain.EvalRun{
		ID:         "run-1",
		DatasetID:  "ds-1",
		ModelLabel: "finetune-v3",
		ScorerMode: domain.ScorerAutomated,
		CreatedAt:  time.Now().UTC(),
		Results: []domain.ItemResult{{
			ItemID:      "eval-001",
			Response:    "use a mutex",
			RubricHits:  1,
			RubricTotal: 1,
			Score:       1.0,
			ScorerType:  domain.ScorerAutomated,
			ScoredAt:    time.Now().UTC(),
		}},
	}
	must(t, repo.Save(run))

	got, err := repo.Get("run-1")
	must(t, err)
	if got.ModelLabel != "finetune-v3" || len(got.Results) != 1 {
		t.Fatalf("eval run round-trip mismatch: %+v", got)
	}
	if got.Results[0].Score != 1.0 {
		t.Errorf("score round-trip = %v, want 1.0", got.Results[0].Score)
	}

	byDataset, err := repo.ListByDataset("ds-1")
	must(t, err)
	if len(byDataset) != 1 {
		t.Errorf("ListByDataset len = %d, want 1", len(byDataset))
	}

	all, err := repo.ListAll()
	must(t, err)
	if len(all) != 1 {
		t.Errorf("ListAll len = %d, want 1", len(all))
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
