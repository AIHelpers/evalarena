package domain

import "time"

// Winner identifies which candidate a reviewer (or judge) picked.
// For tournament matchups with 3+ candidates, Winner holds the
// candidate's SourceLabel instead of "A"/"B".
type Winner string

const (
	WinnerA   Winner = "A"
	WinnerB   Winner = "B"
	WinnerTie Winner = "tie"
)

// Vote is a single reviewer's (or judge's) verdict on a matchup.
type Vote struct {
	ReviewerID string    `json:"reviewer_id"`
	Winner     Winner    `json:"winner"`
	Confidence float64   `json:"confidence,omitempty"` // judge pre-pass only, 0-1
	Note       *string   `json:"note,omitempty"`
	IsJudge    bool      `json:"is_judge"`
	CastAt     time.Time `json:"cast_at"`
}
