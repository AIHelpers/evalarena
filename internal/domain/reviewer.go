package domain

// Reviewer tracks a human reviewer's identity and calibration stats.
// AgreementRate is the fraction of the reviewer's votes that matched
// the eventual consensus (majority human vote, or judge vote if no
// consensus is available) -- useful for flagging unreliable reviewers
// on larger review teams.
type Reviewer struct {
	ID            string  `json:"id"`
	DisplayName   string  `json:"display_name"`
	VotesCast     int     `json:"votes_cast"`
	AgreementRate float64 `json:"agreement_rate"`
}
