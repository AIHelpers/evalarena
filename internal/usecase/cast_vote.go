package usecase

import (
	"errors"
	"fmt"
	"time"

	"evalarena/internal/domain"
)

// CastVoteInput carries the reviewer's pick. In pairwise mode this is the
// *displayed-position* pick ("A" meaning "left", "B" meaning "right", or
// "tie") -- CastVoteUseCase un-swaps it back to the true candidate before
// storing, so results are never biased by which side a reviewer happened to
// see a candidate on. In tournament mode (3+ candidates), Pick must instead
// be one of the matchup's candidate SourceLabels, or "tie" -- there's no
// fixed "A/B" to un-swap since there can be more than two entrants.
type CastVoteInput struct {
	ArenaID    string
	MatchupID  string
	ReviewerID string
	Pick       domain.Winner // "A"/"B"/"tie" (pairwise) or a candidate label/"tie" (tournament)
	Note       *string
}

type CastVoteUseCase struct {
	Repo ArenaRepository
}

func NewCastVoteUseCase(repo ArenaRepository) *CastVoteUseCase {
	return &CastVoteUseCase{Repo: repo}
}

func (uc *CastVoteUseCase) Execute(in CastVoteInput) (*domain.Arena, error) {
	arena, err := uc.Repo.Get(in.ArenaID)
	if err != nil {
		return nil, err
	}
	m := arena.FindMatchup(in.MatchupID)
	if m == nil {
		return nil, errors.New("matchup not found")
	}

	var trueWinner domain.Winner
	if arena.Mode == domain.ModeTournament {
		trueWinner, err = validateTournamentPick(m, in.ReviewerID, in.Pick)
		if err != nil {
			return nil, err
		}
	} else {
		if in.Pick != domain.WinnerA && in.Pick != domain.WinnerB && in.Pick != domain.WinnerTie {
			return nil, errors.New("pick must be A, B, or tie")
		}
		swapped := false
		if m.PositionSwapped != nil {
			swapped = m.PositionSwapped[in.ReviewerID]
		}
		trueWinner = unswap(in.Pick, swapped)
	}

	// Replace any prior vote by this reviewer (revision, not duplication).
	filtered := m.HumanVotes[:0]
	for _, v := range m.HumanVotes {
		if v.ReviewerID != in.ReviewerID {
			filtered = append(filtered, v)
		}
	}
	m.HumanVotes = append(filtered, domain.Vote{
		ReviewerID: in.ReviewerID,
		Winner:     trueWinner,
		Note:       in.Note,
		IsJudge:    false,
		CastAt:     time.Now().UTC(),
	})

	if err := uc.Repo.Save(arena); err != nil {
		return nil, err
	}
	return arena, nil
}

// validateTournamentPick resolves a tournament pick to the real candidate
// label. Accepts either:
//   - a blind PositionKey from a GetMatchupForReview response (e.g. "opt1"),
//     resolved via this reviewer's stored ReviewerOrder -- the browser
//     review UI never sees real labels, so it always sends this form; or
//   - "tie"; or
//   - a real candidate SourceLabel directly, for programmatic/CLI callers
//     that already know which model they mean and skip the blind flow.
func validateTournamentPick(m *domain.Matchup, reviewerID string, pick domain.Winner) (domain.Winner, error) {
	if pick == domain.WinnerTie {
		return pick, nil
	}
	if order, ok := m.ReviewerOrder[reviewerID]; ok {
		for pos, candidateIdx := range order {
			if fmt.Sprintf("opt%d", pos) == string(pick) {
				return domain.Winner(m.Candidates[candidateIdx].SourceLabel), nil
			}
		}
	}
	for _, c := range m.Candidates {
		if c.SourceLabel == string(pick) {
			return pick, nil
		}
	}
	return "", errors.New("pick must be \"tie\", a position from your review (e.g. \"opt0\"), or one of this matchup's candidate labels")
}
