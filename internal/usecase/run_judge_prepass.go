package usecase

import (
	"time"

	"evalarena/internal/domain"
)

// RunJudgePrepassUseCase has a configurable judge model vote on every
// un-judged matchup first. Matchups where the judge's confidence is at or
// above ConfidenceThreshold are considered judge-resolved (no human needed
// unless a human later overrides); matchups below threshold are left for
// human review. This is what cuts human review volume in the plan's
// "auto-pilot mode".
type RunJudgePrepassUseCase struct {
	Repo  ArenaRepository
	Judge JudgeModel
}

func NewRunJudgePrepassUseCase(repo ArenaRepository, judge JudgeModel) *RunJudgePrepassUseCase {
	return &RunJudgePrepassUseCase{Repo: repo, Judge: judge}
}

type JudgePrepassResult struct {
	JudgedHighConfidence int `json:"judged_high_confidence"` // resolved by judge alone
	RoutedToHuman        int `json:"routed_to_human"`        // low confidence, needs a human
}

func (uc *RunJudgePrepassUseCase) Execute(arenaID string, confidenceThreshold float64) (*JudgePrepassResult, error) {
	arena, err := uc.Repo.Get(arenaID)
	if err != nil {
		return nil, err
	}
	result := &JudgePrepassResult{}

	for i := range arena.Matchups {
		m := &arena.Matchups[i]
		if m.JudgeVote != nil {
			continue // already judged
		}
		winner, confidence, err := uc.Judge.Judge(m.Prompt, m.CandidateA.Text, m.CandidateB.Text)
		if err != nil {
			continue // skip on judge failure; leave for human review
		}
		m.JudgeVote = &domain.Vote{
			ReviewerID: "llm-judge",
			Winner:     winner,
			Confidence: confidence,
			IsJudge:    true,
			CastAt:     time.Now().UTC(),
		}
		if confidence >= confidenceThreshold {
			result.JudgedHighConfidence++
		} else {
			result.RoutedToHuman++
		}
	}

	if err := uc.Repo.Save(arena); err != nil {
		return nil, err
	}
	return result, nil
}

// NeedsHumanReview reports whether a matchup still needs a human verdict:
// either no judge ran, or the judge's confidence was below threshold and no
// human vote exists yet.
func NeedsHumanReview(m domain.Matchup, confidenceThreshold float64) bool {
	if len(m.HumanVotes) > 0 {
		return false
	}
	if m.JudgeVote == nil {
		return true
	}
	return m.JudgeVote.Confidence < confidenceThreshold
}
