package domain

import "math"

// WinRateResult summarizes A-vs-B (or category-scoped) outcomes.
type WinRateResult struct {
	Label        string  `json:"label"` // "overall" or a category name
	AWins        int     `json:"a_wins"`
	BWins        int     `json:"b_wins"`
	Ties         int     `json:"ties"`
	Total        int     `json:"total"`
	AWinRate     float64 `json:"a_win_rate"`   // A wins / total, ties excluded from numerator
	WilsonLow    float64 `json:"wilson_low"`   // 95% Wilson score interval lower bound
	WilsonHigh   float64 `json:"wilson_high"`  // 95% Wilson score interval upper bound
	Disagreement int     `json:"disagreement"` // matchups where human reviewers split
}

// WilsonInterval computes the 95% Wilson score confidence interval for a
// binomial proportion successes/total. Ties are excluded from both
// successes and total before calling this (see ComputeWinRate).
// z=1.96 for 95% confidence. Pure implementation, no external stats lib.
func WilsonInterval(successes, total int) (low, high float64) {
	if total == 0 {
		return 0, 0
	}
	z := 1.96
	n := float64(total)
	p := float64(successes) / n
	z2 := z * z
	denom := 1 + z2/n
	centre := p + z2/(2*n)
	margin := z * math.Sqrt(p*(1-p)/n+z2/(4*n*n))
	low = (centre - margin) / denom
	high = (centre + margin) / denom
	if low < 0 {
		low = 0
	}
	if high > 1 {
		high = 1
	}
	return low, high
}

// EloRating is a single candidate's rating within a tournament arena.
type EloRating struct {
	SourceLabel string  `json:"source_label"`
	Rating      float64 `json:"rating"`
	Wins        int     `json:"wins"`
	Losses      int     `json:"losses"`
	Ties        int     `json:"ties"`
	Matches     int     `json:"matches"`
}

const (
	EloInitialRating = 1500.0
	EloKFactor       = 32.0
)

// EloUpdate applies one pairwise-comparison outcome (standard logistic Elo
// update, matching Bradley-Terry's win-probability model) and returns the
// new ratings for both participants. score is 1.0 for an A win, 0.0 for a
// B win, 0.5 for a tie.
func EloUpdate(ratingA, ratingB, score float64) (newA, newB float64) {
	expectedA := 1.0 / (1.0 + math.Pow(10, (ratingB-ratingA)/400.0))
	newA = ratingA + EloKFactor*(score-expectedA)
	expectedB := 1.0 - expectedA
	newB = ratingB + EloKFactor*((1-score)-expectedB)
	return newA, newB
}
