package domain

import "testing"

func TestWilsonInterval(t *testing.T) {
	// Reference values cross-checked against standard Wilson score calculators.
	cases := []struct {
		successes, total int
		wantLow, wantHigh float64
		tol               float64
	}{
		{0, 0, 0, 0, 0.001},
		{5, 10, 0.237, 0.763, 0.01},
		{10, 10, 0.723, 1.0, 0.01},
		{0, 10, 0.0, 0.278, 0.01},
	}
	for _, c := range cases {
		low, high := WilsonInterval(c.successes, c.total)
		if abs(low-c.wantLow) > c.tol {
			t.Errorf("WilsonInterval(%d,%d) low = %v, want ~%v", c.successes, c.total, low, c.wantLow)
		}
		if abs(high-c.wantHigh) > c.tol {
			t.Errorf("WilsonInterval(%d,%d) high = %v, want ~%v", c.successes, c.total, high, c.wantHigh)
		}
	}
}

func TestEloUpdate_EqualRatingsWinnerGainsHalfK(t *testing.T) {
	newA, newB := EloUpdate(1500, 1500, 1.0)
	wantA := 1500 + EloKFactor*0.5
	wantB := 1500 - EloKFactor*0.5
	if abs(newA-wantA) > 0.001 || abs(newB-wantB) > 0.001 {
		t.Errorf("EloUpdate(1500,1500,1.0) = (%v,%v), want (%v,%v)", newA, newB, wantA, wantB)
	}
}

func TestEloUpdate_TieAtEqualRatingsIsNoOp(t *testing.T) {
	newA, newB := EloUpdate(1500, 1500, 0.5)
	if abs(newA-1500) > 0.001 || abs(newB-1500) > 0.001 {
		t.Errorf("EloUpdate(1500,1500,0.5) = (%v,%v), want (1500,1500)", newA, newB)
	}
}

func TestEloUpdate_UnderdogWinGainsMoreThanFavoriteWin(t *testing.T) {
	// A is a big underdog (1200 vs 1800). If A wins, A should gain much more
	// than the symmetric case of equal ratings.
	newA, _ := EloUpdate(1200, 1800, 1.0)
	gain := newA - 1200
	if gain <= EloKFactor*0.5 {
		t.Errorf("expected underdog win to gain more than %v, got %v", EloKFactor*0.5, gain)
	}
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
