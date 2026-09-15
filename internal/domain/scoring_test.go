package domain

import "testing"

// TestRubricHeuristicScore ports evaluate.py's mock-run heuristic check:
// feed each item's own reference answer back as the response, and each
// rubric criterion should be "hit" because the reference answer is
// written to satisfy the rubric.
func TestRubricHeuristicScore_ReferenceAnswerHitsAll(t *testing.T) {
	rubric := []string{
		"function returns a channel",
		"closes the channel after send",
		"handles concurrent senders",
	}
	response := `func collect(ch chan<- int) {
	defer close(ch)
	ch <- 42
}
// concurrent senders are serialized by the channel`
	hits, total := RubricHeuristicScore(response, rubric)
	if hits != total {
		t.Errorf("expected all %d rubric criteria hit by reference answer, got %d", total, hits)
	}
}

func TestRubricHeuristicScore_PartialMatch(t *testing.T) {
	rubric := []string{
		"uses mutex",
		"implements retry",
		"logs error",
	}
	response := "the code uses a mutex and logs errors to stderr, but never retries"
	hits, total := RubricHeuristicScore(response, rubric)
	if total != 3 {
		t.Fatalf("expected total 3, got %d", total)
	}
	if hits != 2 {
		t.Errorf("expected 2 rubric hits, got %d", hits)
	}
}

func TestRubricHeuristicScore_NoMatch(t *testing.T) {
	hits, total := RubricHeuristicScore("nothing relevant here", []string{"uses mutex", "implements retry"})
	if hits != 0 || total != 2 {
		t.Errorf("expected 0/2, got %d/%d", hits, total)
	}
}

func TestRubricHeuristicScore_EmptyRubric(t *testing.T) {
	hits, total := RubricHeuristicScore("any text", nil)
	if hits != 0 || total != 0 {
		t.Errorf("expected 0/0 for empty rubric, got %d/%d", hits, total)
	}
}

func TestCombinedScore(t *testing.T) {
	cases := []struct {
		name        string
		gofmtValid  bool
		hits, total int
		want        float64
	}{
		{"perfect gofmt + full rubric", true, 3, 3, 1.0},
		{"no gofmt + full rubric", false, 3, 3, 0.6},
		{"perfect gofmt + no rubric", true, 0, 2, 0.4},
		{"nothing", false, 0, 3, 0.0},
		{"half rubric fraction", true, 1, 2, 0.7},
	}
	for _, c := range cases {
		got := CombinedScore(c.gofmtValid, c.hits, c.total)
		if got != c.want {
			t.Errorf("%s: CombinedScore(%v,%d,%d) = %v, want %v", c.name, c.gofmtValid, c.hits, c.total, got, c.want)
		}
	}
}

func TestSummarize(t *testing.T) {
	items := map[string]DatasetItem{
		"i1": {ID: "i1", Type: ItemCodeGeneration, Category: "concurrency", Difficulty: "hard"},
		"i2": {ID: "i2", Type: ItemCodeGeneration, Category: "concurrency", Difficulty: "easy"},
		"i3": {ID: "i3", Type: ItemConceptual, Category: "networking", Difficulty: "easy"},
		"i4": {ID: "i4", Type: ItemMultipleChoice, Category: "untyped", Difficulty: "medium"},
	}
	results := []ItemResult{
		{ItemID: "i1", Score: 1.0},
		{ItemID: "i2", Score: 0.6},
		{ItemID: "i3", Score: 0.8},
		{ItemID: "i4", Score: 0.5},
	}
	s := Summarize(results, items)

	if s.N != 4 {
		t.Errorf("expected N=4, got %d", s.N)
	}
	wantOverall := (1.0 + 0.6 + 0.8 + 0.5) / 4
	if abs(s.Overall-wantOverall) > 1e-9 {
		t.Errorf("overall = %v, want %v", s.Overall, wantOverall)
	}
	if got := s.ByType[string(ItemCodeGeneration)]; abs(got-0.8) > 1e-9 {
		t.Errorf("by_type code_generation = %v, want 0.8", got)
	}
	if got := s.ByCategory["concurrency"]; abs(got-0.8) > 1e-9 {
		t.Errorf("by_category concurrency = %v, want 0.8", got)
	}
	if got := s.ByDifficulty["easy"]; abs(got-0.7) > 1e-9 {
		t.Errorf("by_difficulty easy = %v, want 0.7", got)
	}
}
