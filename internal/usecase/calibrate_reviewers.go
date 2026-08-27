package usecase

import "evalarena/internal/domain"

type CalibrateReviewersUseCase struct {
	Repo         ArenaRepository
	ReviewerRepo ReviewerRepository
}

func NewCalibrateReviewersUseCase(repo ArenaRepository, reviewerRepo ReviewerRepository) *CalibrateReviewersUseCase {
	return &CalibrateReviewersUseCase{Repo: repo, ReviewerRepo: reviewerRepo}
}

// Execute walks every matchup with 2+ human votes (or a judge vote to use as
// a tiebreaker reference when humans are split), determines the consensus
// verdict, and tracks how often each reviewer agreed with it. Results are
// persisted per-reviewer so calibration accumulates across arenas.
func (uc *CalibrateReviewersUseCase) Execute(arenaID string) ([]domain.Reviewer, error) {
	arena, err := uc.Repo.Get(arenaID)
	if err != nil {
		return nil, err
	}

	agree := map[string]int{}
	total := map[string]int{}

	for _, m := range arena.Matchups {
		if len(m.HumanVotes) == 0 {
			continue
		}
		var consensus domain.Winner
		if len(m.HumanVotes) >= 2 {
			consensus = tallyToWinner(tallyMatchup(m.HumanVotes))
		} else if m.JudgeVote != nil {
			consensus = m.JudgeVote.Winner
		} else {
			continue // single human vote, no reference to calibrate against
		}
		for _, v := range m.HumanVotes {
			total[v.ReviewerID]++
			if v.Winner == consensus {
				agree[v.ReviewerID]++
			}
		}
	}

	var out []domain.Reviewer
	for reviewerID, t := range total {
		rate := 0.0
		if t > 0 {
			rate = float64(agree[reviewerID]) / float64(t)
		}
		r := domain.Reviewer{ID: reviewerID, VotesCast: t, AgreementRate: rate}
		if uc.ReviewerRepo != nil {
			_ = uc.ReviewerRepo.Save(&r)
		}
		out = append(out, r)
	}
	return out, nil
}

func tallyToWinner(t tally) domain.Winner {
	switch {
	case t.aWins == 1:
		return domain.WinnerA
	case t.bWins == 1:
		return domain.WinnerB
	default:
		return domain.WinnerTie
	}
}
