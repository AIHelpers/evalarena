package domain

// CandidateOutput is one model's response to a matchup's prompt.
// SourceLabel identifies the underlying model/version pre-blinding
// (e.g. "base-model", "finetune-v3"); it is never shown to reviewers
// while voting, only in results/exports.
type CandidateOutput struct {
	SourceLabel string `json:"source_label"`
	Text        string `json:"text"`
}

// Matchup is a single prompt plus the candidate outputs being compared.
// Pairwise arenas use CandidateA/CandidateB; tournament arenas use
// Candidates for 3+ entrants.
type Matchup struct {
	ID         string            `json:"id"`
	Prompt     string            `json:"prompt"`
	Category   *string           `json:"category,omitempty"`
	CandidateA CandidateOutput   `json:"candidate_a"`
	CandidateB CandidateOutput   `json:"candidate_b"`
	Candidates []CandidateOutput `json:"candidates,omitempty"` // tournament mode, 3+
	JudgeVote  *Vote             `json:"judge_vote,omitempty"` // optional LLM pre-pass
	HumanVotes []Vote            `json:"human_votes"`

	// PositionSwapped records, per reviewer session, whether A/B were
	// flipped on display to control for position bias. Keyed by
	// ReviewerID so results can un-blind correctly at aggregation time.
	// Pairwise mode only.
	PositionSwapped map[string]bool `json:"position_swapped,omitempty"`

	// ReviewerOrder records, per reviewer, the display permutation of
	// Candidates (a permutation of indices into Candidates) so a
	// reviewer's blind position pick can be resolved back to the real
	// candidate. Tournament mode only.
	ReviewerOrder map[string][]int `json:"reviewer_order,omitempty"`
}

// EffectiveVotes returns human votes plus the judge pre-pass vote (if any
// and if no human vote exists yet), matching the "judge votes fast cases,
// humans handle the rest" auto-pilot flow.
func (m *Matchup) EffectiveVotes() []Vote {
	votes := append([]Vote{}, m.HumanVotes...)
	if len(votes) == 0 && m.JudgeVote != nil {
		votes = append(votes, *m.JudgeVote)
	}
	return votes
}
