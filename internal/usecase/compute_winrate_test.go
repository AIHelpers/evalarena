package usecase

import (
	"testing"

	"evalarena/internal/domain"
)

func TestTallyMatchup_Majority(t *testing.T) {
	cases := []struct {
		name  string
		votes []domain.Vote
		want  tally
	}{
		{"unanimous A", []domain.Vote{{Winner: domain.WinnerA}, {Winner: domain.WinnerA}}, tally{aWins: 1}},
		{"split 2-1 B", []domain.Vote{{Winner: domain.WinnerA}, {Winner: domain.WinnerB}, {Winner: domain.WinnerB}}, tally{bWins: 1}},
		{"even split is tie", []domain.Vote{{Winner: domain.WinnerA}, {Winner: domain.WinnerB}}, tally{ties: 1}},
		{"explicit ties dominate", []domain.Vote{{Winner: domain.WinnerTie}, {Winner: domain.WinnerTie}, {Winner: domain.WinnerA}}, tally{ties: 1}},
	}
	for _, c := range cases {
		got := tallyMatchup(c.votes)
		if got != c.want {
			t.Errorf("%s: tallyMatchup() = %+v, want %+v", c.name, got, c.want)
		}
	}
}

func TestHumanSplit(t *testing.T) {
	if humanSplit([]domain.Vote{{Winner: domain.WinnerA}}) {
		t.Error("single vote should never be a split")
	}
	if !humanSplit([]domain.Vote{{Winner: domain.WinnerA}, {Winner: domain.WinnerB}}) {
		t.Error("A vs B should be a split")
	}
	if humanSplit([]domain.Vote{{Winner: domain.WinnerA}, {Winner: domain.WinnerA}}) {
		t.Error("unanimous votes should not be a split")
	}
}
