package judge_test

import (
	"testing"

	"evalarena/internal/adapter/judge"
	"evalarena/internal/domain"
)

func TestAnthropicRubricJudge_NoopWhenNoKeyOrModel(t *testing.T) {
	// NewAnthropicRubricJudge with no ANTHROPIC_API_KEY env returns a no-op.
	t.Setenv("ANTHROPIC_API_KEY", "")
	j := judge.NewAnthropicRubricJudge("claude-sonnet-4-6")
	hits, total, rationale, err := j.JudgeAgainstRubric("q", "ref", []string{"c1", "c2"}, "resp")
	if err != nil {
		t.Fatalf("no-op judge should not error, got %v", err)
	}
	if hits != 0 || total != 2 {
		t.Errorf("no-op judge hit/total = %d/%d, want 0/2", hits, total)
	}
	if rationale != "" {
		t.Errorf("no-op judge rationale = %q, want empty", rationale)
	}
}

func TestAnthropicRubricJudge_NoopWhenEmptyModel(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "fake-key")
	j := judge.NewAnthropicRubricJudge("")
	hits, total, _, err := j.JudgeAgainstRubric("q", "ref", []string{"c1"}, "resp")
	if err != nil {
		t.Fatal(err)
	}
	if hits != 0 || total != 1 {
		t.Errorf("empty-model no-op hit/total = %d/%d, want 0/1", hits, total)
	}
}

func TestAnthropicRubricJudge_ConfiguredReturnsError(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "fake-key")
	j := judge.NewAnthropicRubricJudge("claude-sonnet-4-6")
	// A fully configured judge with an invalid key: total still equals the
	// rubric length, but the call itself errors because the API call fails.
	_, total, _, err := j.JudgeAgainstRubric("q", "ref", []string{"c1"}, "resp")
	if err == nil {
		t.Error("expected error calling API with fake key")
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
}

func TestAnthropicJudge_NoopWhenNoKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	j := judge.NewAnthropicJudge("claude-sonnet-4-6")
	if _, ok := j.(judge.NoopJudge); !ok {
		t.Errorf("expected NoopJudge when no API key, got %T", j)
	}
}

func TestNoopJudge_Judge(t *testing.T) {
	j := judge.NoopJudge{}
	w, conf, err := j.Judge("p", "a", "b")
	if err != nil {
		t.Fatal(err)
	}
	if w != domain.WinnerTie || conf != 0 {
		t.Errorf("NoopJudge = %v, %.1f; want tie, 0", w, conf)
	}
}
