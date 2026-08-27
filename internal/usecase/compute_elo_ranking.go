package usecase

import (
	"errors"
	"sort"

	"evalarena/internal/domain"
)

type ComputeEloRankingUseCase struct {
	Repo ArenaRepository
}

func NewComputeEloRankingUseCase(repo ArenaRepository) *ComputeEloRankingUseCase {
	return &ComputeEloRankingUseCase{Repo: repo}
}

// Execute computes Elo ratings across all matchups in a tournament-mode
// arena. Every matchup's candidates are compared pairwise against every
// other candidate in that same matchup, in vote order, applying the
// standard logistic Elo update after each recorded verdict.
func (uc *ComputeEloRankingUseCase) Execute(arenaID string) ([]domain.EloRating, error) {
	arena, err := uc.Repo.Get(arenaID)
	if err != nil {
		return nil, err
	}
	if arena.Mode != domain.ModeTournament {
		return nil, errors.New("elo ranking only applies to tournament-mode arenas")
	}

	ratings := map[string]*domain.EloRating{}
	get := func(label string) *domain.EloRating {
		r, ok := ratings[label]
		if !ok {
			r = &domain.EloRating{SourceLabel: label, Rating: domain.EloInitialRating}
			ratings[label] = r
		}
		return r
	}

	for _, m := range arena.Matchups {
		if len(m.Candidates) < 2 {
			continue
		}
		for _, v := range m.EffectiveVotes() {
			// In tournament mode, Winner holds the winning candidate's
			// SourceLabel directly (or "tie"). Apply that verdict against
			// every other candidate in the matchup.
			winnerLabel := string(v.Winner)
			for _, c := range m.Candidates {
				if c.SourceLabel == winnerLabel {
					continue
				}
				a := get(winnerLabel)
				b := get(c.SourceLabel)
				score := 1.0
				if v.Winner == domain.WinnerTie {
					score = 0.5
					a = get(m.Candidates[0].SourceLabel)
					b = get(m.Candidates[1].SourceLabel)
				}
				newA, newB := domain.EloUpdate(a.Rating, b.Rating, score)
				a.Rating, b.Rating = newA, newB
				a.Matches++
				b.Matches++
				if score == 1.0 {
					a.Wins++
					b.Losses++
				} else {
					a.Ties++
					b.Ties++
				}
			}
		}
	}

	out := make([]domain.EloRating, 0, len(ratings))
	for _, r := range ratings {
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Rating > out[j].Rating })
	return out, nil
}
