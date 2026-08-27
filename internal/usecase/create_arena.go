package usecase

import (
	"errors"
	"time"

	"evalarena/internal/domain"
)

// CreateArenaInput is one prompt's worth of raw material for an arena.
// For pairwise mode, populate CandidateA/CandidateB. For tournament mode
// (3+ candidates), populate Candidates instead.
type MatchupInput struct {
	Prompt     string
	Category   *string
	CandidateA domain.CandidateOutput
	CandidateB domain.CandidateOutput
	Candidates []domain.CandidateOutput
}

type CreateArenaInput struct {
	Name     string
	Mode     domain.Mode
	Matchups []MatchupInput
}

type CreateArenaUseCase struct {
	Repo   ArenaRepository
	Random RandomSource
}

func NewCreateArenaUseCase(repo ArenaRepository, random RandomSource) *CreateArenaUseCase {
	return &CreateArenaUseCase{Repo: repo, Random: random}
}

func (uc *CreateArenaUseCase) Execute(in CreateArenaInput) (*domain.Arena, error) {
	if in.Name == "" {
		return nil, errors.New("arena name is required")
	}
	if len(in.Matchups) == 0 {
		return nil, errors.New("at least one matchup (prompt + candidates) is required")
	}
	mode := in.Mode
	if mode == "" {
		mode = domain.ModePairwise
	}

	arena := &domain.Arena{
		ID:        uc.Random.ID(),
		Name:      in.Name,
		Mode:      mode,
		CreatedAt: time.Now().UTC(),
	}

	for _, mi := range in.Matchups {
		if mi.Prompt == "" {
			return nil, errors.New("every matchup requires a prompt")
		}
		if mode == domain.ModeTournament {
			if len(mi.Candidates) < 3 {
				return nil, errors.New("tournament mode matchups require 3+ candidates")
			}
		} else if mi.CandidateA.Text == "" || mi.CandidateB.Text == "" {
			return nil, errors.New("pairwise matchups require both candidate A and B text")
		}
		arena.Matchups = append(arena.Matchups, domain.Matchup{
			ID:         uc.Random.ID(),
			Prompt:     mi.Prompt,
			Category:   mi.Category,
			CandidateA: mi.CandidateA,
			CandidateB: mi.CandidateB,
			Candidates: mi.Candidates,
		})
	}

	if err := uc.Repo.Save(arena); err != nil {
		return nil, err
	}
	return arena, nil
}
