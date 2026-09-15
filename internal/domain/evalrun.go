package domain

import "time"

// ScorerType identifies which mechanism scored an item.
type ScorerType string

// Scorer type constants.
const (
	ScorerAutomated ScorerType = "automated"
	ScorerLLMJudge  ScorerType = "llm_judge"
	ScorerHuman     ScorerType = "human"
)

// ItemResult is one item's evaluation outcome within a run.
type ItemResult struct {
	ItemID         string     `json:"item_id"`
	Response       string     `json:"response"`
	GofmtValid     *bool      `json:"gofmt_valid,omitempty"` // nil if not applicable (non-code item)
	MCChosen       string     `json:"mc_chosen,omitempty"`
	MCCorrect      *bool      `json:"mc_correct,omitempty"`
	RubricHits     int        `json:"rubric_hits"`
	RubricTotal    int        `json:"rubric_total"`
	Score          float64    `json:"score"`
	ScorerType     ScorerType `json:"scorer_type"`
	JudgeRationale string     `json:"judge_rationale,omitempty"`
	ScoredAt       time.Time  `json:"scored_at"`
}

// EvalRun is one complete evaluation of a dataset by a model, persisted
// so runs can be compared over time.
type EvalRun struct {
	ID         string       `json:"id"`
	DatasetID  string       `json:"dataset_id"`
	ModelLabel string       `json:"model_label"`
	ScorerMode ScorerType   `json:"scorer_mode"`
	CreatedAt  time.Time    `json:"created_at"`
	Notes      string       `json:"notes,omitempty"`
	Results    []ItemResult `json:"results"`
}
