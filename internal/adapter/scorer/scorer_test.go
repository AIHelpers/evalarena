package scorer_test

import (
	"testing"

	"evalarena/internal/adapter/scorer"
)

func TestMCScorer_ParseChoice_ValidLetters(t *testing.T) {
	s := scorer.MCScorer{}
	cases := []struct {
		name     string
		response string
		want     string
	}{
		{"plain A", "A", "A"},
		{"plain b lowercase", "b", "B"},
		{"with period", "C.", "C"},
		{"with parens", "(D)", "D"},
		{"inside sentence", "The answer is A.", "A"},
		{"leading/trailing spaces", "  B  ", "B"},
		{"mixed case", "d", "D"},
	}
	for _, c := range cases {
		got, err := s.ParseChoice(c.response)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", c.name, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s: ParseChoice(%q) = %q, want %q", c.name, c.response, got, c.want)
		}
	}
}

func TestMCScorer_ParseChoice_NoLetter(t *testing.T) {
	s := scorer.MCScorer{}
	cases := []string{"", "none", "not sure", "42", "E", "F", "Z"}
	for _, resp := range cases {
		if _, err := s.ParseChoice(resp); err == nil {
			t.Errorf("ParseChoice(%q): expected error for no A-D letter, got nil", resp)
		}
	}
}

func TestRubricHeuristicScorer_Score(t *testing.T) {
	s := scorer.RubricHeuristicScorer{}
	hits, total := s.Score("the code uses a mutex", []string{"uses mutex", "implements retry"})
	if total != 2 {
		t.Fatalf("expected total 2, got %d", total)
	}
	if hits != 1 {
		t.Errorf("expected 1 hit, got %d", hits)
	}
}

func TestRubricHeuristicScorer_Score_EmptyRubric(t *testing.T) {
	s := scorer.RubricHeuristicScorer{}
	hits, total := s.Score("any text", nil)
	if hits != 0 || total != 0 {
		t.Errorf("expected 0/0 for empty rubric, got %d/%d", hits, total)
	}
}

// TestGofmtScorer_Valid checks that properly formatted Go code is judged
// gofmt-valid.
func TestGofmtScorer_Valid(t *testing.T) {
	s := scorer.GofmtScorer{}
	valid, _, err := s.Valid("package main\n\nfunc main() {\n}\n")
	if err != nil {
		if err == scorer.ErrGofmtUnavailable {
			t.Skip("gofmt not on PATH, skipping")
		}
		t.Skipf("gofmt unavailable/non-functional on this platform: %v", err)
	}
	if !valid {
		t.Error("expected formatted code to be gofmt-valid")
	}
}

func TestGofmtScorer_InvalidFormatting(t *testing.T) {
	s := scorer.GofmtScorer{}
	code := "package main\nfunc main() {\n\t\tx := 1\n}\n"
	valid, _, err := s.Valid(code)
	if err != nil {
		if err == scorer.ErrGofmtUnavailable {
			t.Skip("gofmt not on PATH, skipping")
		}
		t.Skipf("gofmt unavailable/non-functional on this platform: %v", err)
	}
	if valid {
		t.Error("expected mis-formatted code to be invalid")
	}
}

func TestGofmtScorer_InvalidGoSyntax(t *testing.T) {
	s := scorer.GofmtScorer{}
	valid, _, err := s.Valid("package main\nfunc main( {\n")
	if err != nil {
		if err == scorer.ErrGofmtUnavailable {
			t.Skip("gofmt not on PATH, skipping")
		}
		t.Skipf("gofmt unavailable/non-functional on this platform: %v", err)
	}
	if valid {
		t.Error("expected syntax-invalid code to be not gofmt-valid")
	}
}

func TestGofmtScorer_EmptyCodeIsValid(t *testing.T) {
	s := scorer.GofmtScorer{}
	valid, _, err := s.Valid("")
	if err != nil {
		if err == scorer.ErrGofmtUnavailable {
			t.Skip("gofmt not on PATH, skipping")
		}
		t.Skipf("gofmt unavailable/non-functional on this platform: %v", err)
	}
	if !valid {
		t.Error("expected empty code to be gofmt-valid")
	}
}
