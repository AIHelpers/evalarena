package importer

import (
	"encoding/json"
	"fmt"

	"evalarena/internal/domain"
	"evalarena/internal/usecase"
)

// distilleryJobResult is the expected shape of a Distillery fine-tune job
// export: base model vs. the resulting fine-tune, evaluated over a held-out
// prompt set. This is the plan's flagship use case: "did the fine-tune
// actually help?"
type distilleryJobResult struct {
	JobID       string `json:"job_id"`
	BaseModel   string `json:"base_model"`
	FinetuneTag string `json:"finetune_tag"`
	EvalPrompts []struct {
		Prompt         string  `json:"prompt"`
		Category       *string `json:"category,omitempty"`
		BaseOutput     string  `json:"base_output"`
		FinetuneOutput string  `json:"finetune_output"`
	} `json:"eval_prompts"`
}

type DistilleryImporter struct{}

func (DistilleryImporter) Import(raw []byte) ([]usecase.MatchupInput, error) {
	var job distilleryJobResult
	if err := json.Unmarshal(raw, &job); err != nil {
		return nil, fmt.Errorf("distillery import: %w", err)
	}
	if len(job.EvalPrompts) == 0 {
		return nil, fmt.Errorf("distillery import: job %q has no eval prompts", job.JobID)
	}
	base, tune := job.BaseModel, job.FinetuneTag
	if base == "" {
		base = "base-model"
	}
	if tune == "" {
		tune = "finetune"
	}
	out := make([]usecase.MatchupInput, 0, len(job.EvalPrompts))
	for _, ep := range job.EvalPrompts {
		out = append(out, usecase.MatchupInput{
			Prompt:     ep.Prompt,
			Category:   ep.Category,
			CandidateA: domain.CandidateOutput{SourceLabel: base, Text: ep.BaseOutput},
			CandidateB: domain.CandidateOutput{SourceLabel: tune, Text: ep.FinetuneOutput},
		})
	}
	return out, nil
}
