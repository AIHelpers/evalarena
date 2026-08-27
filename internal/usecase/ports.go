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
