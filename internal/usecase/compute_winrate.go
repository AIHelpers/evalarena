package usecase

import "evalarena/internal/domain"

type ComputeWinRateUseCase struct {
	Repo ArenaRepository
}

func NewComputeWinRateUseCase(repo ArenaRepository) *ComputeWinRateUseCase {
	return &ComputeWinRateUseCase{Repo: repo}
}

// WinRateReport is the overall result plus a per-category breakdown, so
// callers can see *where* one model wins vs. loses, not just an aggregate.
type WinRateReport struct {
	Overall    domain.WinRateResult   `json:"overall"`
	ByCategory []domain.WinRateResult `json:"by_category"`
}

func (uc *ComputeWinRateUseCase) Execute(arenaID string) (*WinRateReport, error) {
	arena, err := uc.Repo.Get(arenaID)
	if err != nil {
		return nil, err
	}

	byCategory := map[string]*domain.WinRateResult{}
	overall := &domain.WinRateResult{Label: "overall"}

	for _, m := range arena.Matchups {
		votes := m.EffectiveVotes()
		if len(votes) == 0 {
			continue
		}
		result := tallyMatchup(votes)

		cat := "uncategorized"
		if m.Category != nil && *m.Category != "" {
			cat = *m.Category
		}
		cr, ok := byCategory[cat]
		if !ok {
			cr = &domain.WinRateResult{Label: cat}
			byCategory[cat] = cr
		}
		applyTally(overall, result)
		applyTally(cr, result)

		if humanSplit(m.HumanVotes) {
			overall.Disagreement++
			cr.Disagreement++
		}
	}

	finalize(overall)
	report := &WinRateReport{Overall: *overall}
	for _, cat := range arena.Categories() {
		if cr, ok := byCategory[cat]; ok {
			finalize(cr)
			report.ByCategory = append(report.ByCategory, *cr)
		}
	}
	return report, nil
}

type tally struct {
	aWins, bWins, ties int
}

// tallyMatchup collapses a matchup's votes into a single verdict: majority
// vote among (human or judge) votes, tie if evenly split.
func tallyMatchup(votes []domain.Vote) tally {
	var a, b, t int
	for _, v := range votes {
		switch v.Winner {
		case domain.WinnerA:
			a++
		case domain.WinnerB:
			b++
		default:
			t++
		}
	}
	switch {
	case a > b && a > t:
		return tally{aWins: 1}
	case b > a && b > t:
		return tally{bWins: 1}
	default:
		return tally{ties: 1}
	}
}

func applyTally(r *domain.WinRateResult, t tally) {
	r.AWins += t.aWins
	r.BWins += t.bWins
	r.Ties += t.ties
	r.Total++
}

func humanSplit(votes []domain.Vote) bool {
	if len(votes) < 2 {
		return false
	}
	seen := map[domain.Winner]bool{}
	for _, v := range votes {
		seen[v.Winner] = true
	}
	return len(seen) > 1
}

func finalize(r *domain.WinRateResult) {
	decided := r.AWins + r.BWins
	if decided > 0 {
		r.AWinRate = float64(r.AWins) / float64(decided)
	}
	r.WilsonLow, r.WilsonHigh = domain.WilsonInterval(r.AWins, decided)
}
