// Package domain contains the core value types and pure domain logic.
package domain

import "time"

// Mode determines how an Arena's matchups are scored.
type Mode string

const (
	// ModePairwise is a two-candidate comparison arena.
	ModePairwise Mode = "pairwise"
	// ModeTournament is a three-or-more-candidate comparison arena.
	ModeTournament Mode = "tournament"
)

// Arena is a single comparison session: a set of prompts, each judged
// across two (pairwise) or more (tournament) candidate outputs.
type Arena struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Mode      Mode      `json:"mode"`
	Matchups  []Matchup `json:"matchups"`
	CreatedAt time.Time `json:"created_at"`
}

// FindMatchup returns a pointer to the matchup with the given ID, or nil.
func (a *Arena) FindMatchup(id string) *Matchup {
	for i := range a.Matchups {
		if a.Matchups[i].ID == id {
			return &a.Matchups[i]
		}
	}
	return nil
}

// Categories returns the distinct set of categories present in the arena,
// in first-seen order. Matchups with no category are grouped as "uncategorized".
func (a *Arena) Categories() []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range a.Matchups {
		cat := "uncategorized"
		if m.Category != nil && *m.Category != "" {
			cat = *m.Category
		}
		if !seen[cat] {
			seen[cat] = true
			out = append(out, cat)
		}
	}
	return out
}
