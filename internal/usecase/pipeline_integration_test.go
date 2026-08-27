package usecase_test

import (
	"testing"

	"evalarena/internal/adapter/repository/jsonfile"
	"evalarena/internal/domain"
	"evalarena/internal/infra"
	"evalarena/internal/usecase"
)

func TestFullPipeline_CreateVoteResults(t *testing.T) {
	repo, err := jsonfile.NewArenaRepository(t.TempDir())
	if err != nil {
		t.Fatalf("repo init: %v", err)
	}
	random := infra.SystemRandom{}

	create := usecase.NewCreateArenaUseCase(repo, random)
	arena, err := create.Execute(usecase.CreateArenaInput{
		Name: "pipeline test",
		Mode: domain.ModePairwise,
		Matchups: []usecase.MatchupInput{
			{
				Prompt:     "p1",
				CandidateA: domain.CandidateOutput{SourceLabel: "a", Text: "text a"},
				CandidateB: domain.CandidateOutput{SourceLabel: "b", Text: "text b"},
			},
		},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(arena.Matchups) != 1 {
		t.Fatalf("expected 1 matchup, got %d", len(arena.Matchups))
	}
	matchupID := arena.Matchups[0].ID

	getForReview := usecase.NewGetMatchupForReviewUseCase(repo, random)
	bm, err := getForReview.Execute(arena.ID, matchupID, "reviewer1")
	if err != nil {
		t.Fatalf("get for review: %v", err)
	}
	if bm.LeftText == "" || bm.RightText == "" {
		t.Fatalf("expected non-empty blinded texts, got %+v", bm)
	}

	castVote := usecase.NewCastVoteUseCase(repo)
	if _, err := castVote.Execute(usecase.CastVoteInput{
		ArenaID: arena.ID, MatchupID: matchupID, ReviewerID: "reviewer1", Pick: domain.WinnerA,
	}); err != nil {
		t.Fatalf("cast vote: %v", err)
	}

	winRate := usecase.NewComputeWinRateUseCase(repo)
	report, err := winRate.Execute(arena.ID)
	if err != nil {
		t.Fatalf("compute win rate: %v", err)
	}
	if report.Overall.Total != 1 {
		t.Fatalf("expected 1 decided matchup, got %d", report.Overall.Total)
	}
	if report.Overall.AWins+report.Overall.BWins != 1 {
		t.Fatalf("expected exactly one of A/B to have won, got A=%d B=%d", report.Overall.AWins, report.Overall.BWins)
	}
}

// TestPositionRandomization_Uniform simulates many independent reviewers on
// the same matchup and checks the swap distribution isn't wildly skewed --
// guarding against the "reviewers favor the first option" bias the plan
// calls out.
func TestPositionRandomization_Uniform(t *testing.T) {
	repo, err := jsonfile.NewArenaRepository(t.TempDir())
	if err != nil {
		t.Fatalf("repo init: %v", err)
	}
	random := infra.SystemRandom{}
	create := usecase.NewCreateArenaUseCase(repo, random)
	arena, err := create.Execute(usecase.CreateArenaInput{
		Name: "bias test",
		Mode: domain.ModePairwise,
		Matchups: []usecase.MatchupInput{{
			Prompt:     "p1",
			CandidateA: domain.CandidateOutput{SourceLabel: "a", Text: "AAA"},
			CandidateB: domain.CandidateOutput{SourceLabel: "b", Text: "BBB"},
		}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	matchupID := arena.Matchups[0].ID

	getForReview := usecase.NewGetMatchupForReviewUseCase(repo, random)
	const n = 500
	aOnLeft := 0
	for i := 0; i < n; i++ {
		reviewerID := "reviewer" + string(rune('A'+i%26)) + string(rune(i))
		bm, err := getForReview.Execute(arena.ID, matchupID, reviewerID)
		if err != nil {
			t.Fatalf("get for review: %v", err)
		}
		if bm.LeftText == "AAA" {
			aOnLeft++
		}
	}
	// With n=500 independent coin flips, expect close to 50/50; allow a
	// generous band to keep this test non-flaky while still catching a
	// systematically broken (e.g. always-same-side) implementation.
	frac := float64(aOnLeft) / float64(n)
	if frac < 0.35 || frac > 0.65 {
		t.Errorf("position randomization looks skewed: A landed on left %.1f%% of the time (want ~50%%)", frac*100)
	}
}
