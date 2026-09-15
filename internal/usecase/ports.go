package usecase

import "evalarena/internal/domain"

// ArenaRepository is the persistence boundary. The JSON-file adapter
// implements this today; a sqlite or postgres adapter can be swapped in
// without touching any usecase code.
type ArenaRepository interface {
	Save(arena *domain.Arena) error
	Get(id string) (*domain.Arena, error)
	List() ([]*domain.Arena, error)
	Delete(id string) error
}

// ReviewerRepository persists reviewer calibration stats across arenas.
type ReviewerRepository interface {
	Save(r *domain.Reviewer) error
	Get(id string) (*domain.Reviewer, error)
	List() ([]*domain.Reviewer, error)
}

// JudgeModel is the LLM-judge auto-pilot boundary: given a prompt and two
// candidate outputs, return a vote and a 0-1 confidence score. Low
// confidence routes the matchup to human review instead.
type JudgeModel interface {
	Judge(prompt, textA, textB string) (winner domain.Winner, confidence float64, err error)
}

// RandomSource abstracts randomness so position-bias randomization is
// deterministic and testable.
type RandomSource interface {
	// Bool returns true/false with ~50/50 probability.
	Bool() bool
	// ID returns a new unique identifier.
	ID() string
	// Perm returns a random permutation of [0,n), used to randomize
	// candidate display order in tournament-mode reviews.
	Perm(n int) []int
}

// DatasetRepository persists golden evaluation datasets.
type DatasetRepository interface {
	Save(ds *domain.Dataset) error
	Get(id string) (*domain.Dataset, error)
	List() ([]*domain.Dataset, error)
}

// EvalRunRepository persists evaluation runs for comparison over time.
type EvalRunRepository interface {
	Save(run *domain.EvalRun) error
	Get(id string) (*domain.EvalRun, error)
	ListByDataset(datasetID string) ([]*domain.EvalRun, error)
	ListAll() ([]*domain.EvalRun, error)
}

// RubricJudgeModel checks a single response against a reference answer
// and rubric — an absolute correctness judgment, unlike JudgeModel's
// blind A-vs-B preference judgment. Deliberately a separate interface:
// the existing pairwise judge stays untouched.
type RubricJudgeModel interface {
	JudgeAgainstRubric(question, referenceAnswer string, rubric []string, response string) (
		hits int, total int, rationale string, err error)
}

// GofmtScorer checks whether code text is gofmt-valid. If gofmt isn't
// available on PATH, it reports (false, err) and the caller should treat
// it as "unavailable" rather than "invalid".
type GofmtScorer interface {
	Valid(code string) (bool, string, error)
}

// MCScorer extracts the letter a multiple-choice response chose.
type MCScorer interface {
	ParseChoice(response string) (string, error)
}

// RubricScorer applies the rubric heuristic to a response. Always
// available — this is the fallback when no LLM judge is configured.
type RubricScorer interface {
	Score(response string, rubric []string) (hits, total int)
}
