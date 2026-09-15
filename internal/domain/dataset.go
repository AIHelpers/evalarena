package domain

import "time"

// ItemType classifies what kind of task a dataset item represents.
type ItemType string

// Item type constants.
const (
	ItemCodeGeneration ItemType = "code_generation"
	ItemBugFix         ItemType = "bug_fix"
	ItemConceptual     ItemType = "conceptual"
	ItemMultipleChoice ItemType = "multiple_choice"
)

// DatasetItem is one golden-eval item: a question, a reference answer,
// and a rubric to grade responses against. Exactly the shape of
// golden_eval_set.jsonl.
type DatasetItem struct {
	ID              string            `json:"id"`
	Type            ItemType          `json:"type"`
	Category        string            `json:"category"`
	Difficulty      string            `json:"difficulty"`
	Question        string            `json:"question"`
	ReferenceAnswer string            `json:"reference_answer"`
	Rubric          []string          `json:"rubric"`
	Choices         map[string]string `json:"choices,omitempty"` // multiple_choice only
	CorrectChoice   string            `json:"correct_choice,omitempty"`
	Tags            []string          `json:"tags"`
}

// Dataset is an imported golden evaluation set, versioned and persisted.
type Dataset struct {
	ID         string        `json:"id"`
	Name       string        `json:"name"`
	Version    string        `json:"version"`
	SourcePath string        `json:"source_path"`
	ImportedAt time.Time     `json:"imported_at"`
	Items      []DatasetItem `json:"items"`
}
