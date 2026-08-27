package usecase

import (
	"errors"
	"fmt"

	"evalarena/internal/domain"
)

// BlindedOption is one anonymized candidate shown to a reviewer in
// tournament mode. PositionKey is what the reviewer picks (e.g. "opt0"),
// never the real candidate label -- CastVoteUseCase resolves it back to the
// true label server-side via the matchup's stored ReviewerOrder.
type BlindedOption struct {
	PositionKey string
	Text        string
}

// BlindedMatchup is what a reviewer actually sees: no source labels, and
// display order randomized per-reviewer to control for position bias.
// Pairwise mode populates LeftText/RightText; tournament mode populates
// Options instead.
type BlindedMatchup struct {
	MatchupID    string
	Prompt       string
	Category     *string
	LeftText     string
	RightText    string
	Options      []BlindedOption
	AlreadyVoted bool
}

type GetMatchupForReviewUseCase struct {
	Repo   ArenaRepository
	Random RandomSource
}

func NewGetMatchupForReviewUseCase(repo ArenaRepository, random RandomSource) *GetMatchupForReviewUseCase {
	return &GetMatchupForReviewUseCase{Repo: repo, Random: random}
}

func (uc *GetMatchupForReviewUseCase) Execute(arenaID, matchupID, reviewerID string) (*BlindedMatchup, error) {
	arena, err := uc.Repo.Get(arenaID)
	if err != nil {
		return nil, err
	}
	m := arena.FindMatchup(matchupID)
	if m == nil {
		return nil, errors.New("matchup not found")
	}

	alreadyVoted := false
	for _, v := range m.HumanVotes {
		if v.ReviewerID == reviewerID {
			alreadyVoted = true
			break
		}
	}

	if arena.Mode == domain.ModeTournament {
		if m.ReviewerOrder == nil {
			m.ReviewerOrder = map[string][]int{}
		}
		order, assigned := m.ReviewerOrder[reviewerID]
		if !assigned {
			order = uc.Random.Perm(len(m.Candidates))
			m.ReviewerOrder[reviewerID] = order
			if err := uc.Repo.Save(arena); err != nil {
				return nil, err
			}
		}
		options := make([]BlindedOption, len(order))
		for pos, candidateIdx := range order {
			options[pos] = BlindedOption{
				PositionKey: fmt.Sprintf("opt%d", pos),
				Text:        m.Candidates[candidateIdx].Text,
			}
		}
		return &BlindedMatchup{
			MatchupID:    m.ID,
			Prompt:       m.Prompt,
			Category:     m.Category,
			Options:      options,
			AlreadyVoted: alreadyVoted,
		}, nil
	}

	if m.PositionSwapped == nil {
		m.PositionSwapped = map[string]bool{}
	}
	swapped, assigned := m.PositionSwapped[reviewerID]
	if !assigned {
		swapped = uc.Random.Bool()
		m.PositionSwapped[reviewerID] = swapped
		if err := uc.Repo.Save(arena); err != nil {
			return nil, err
		}
	}

	left, right := m.CandidateA.Text, m.CandidateB.Text
	if swapped {
		left, right = right, left
	}

	return &BlindedMatchup{
		MatchupID:    m.ID,
		Prompt:       m.Prompt,
		Category:     m.Category,
		LeftText:     left,
		RightText:    right,
		AlreadyVoted: alreadyVoted,
	}, nil
}

// unswap converts a reviewer's left/right/tie pick back into the true A/B/tie
// winner, given whether that reviewer's view was position-swapped.
func unswap(pick domain.Winner, swapped bool) domain.Winner {
	if !swapped || pick == domain.WinnerTie {
		return pick
	}
	if pick == domain.WinnerA {
		return domain.WinnerB
	}
	return domain.WinnerA
}
