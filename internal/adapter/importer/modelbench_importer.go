// Package importer implements usecase.Importer for each upstream tool in
// the vibe-coding-hub suite. Each importer expects that tool's own export
// JSON shape and maps it onto usecase.MatchupInput; nothing outside this
// package needs to know those shapes.
package importer

import (
	"encoding/json"
	"fmt"

	"evalarena/internal/domain"
	"evalarena/internal/usecase"
)

// modelBenchRun is the expected shape of a ModelBench-Local run export:
// one prompt set run against two model configs, with per-prompt outputs.
type modelBenchRun struct {
	RunID   string `json:"run_id"`
	ModelA  string `json:"model_a"`
	ModelB  string `json:"model_b"`
	Results []struct {
		Prompt   string  `json:"prompt"`
		Category *string `json:"category,omitempty"`
		OutputA  string  `json:"output_a"`
		OutputB  string  `json:"output_b"`
	} `json:"results"`
}

type ModelBenchImporter struct{}

func (ModelBenchImporter) Import(raw []byte) ([]usecase.MatchupInput, error) {
	var run modelBenchRun
	if err := json.Unmarshal(raw, &run); err != nil {
		return nil, fmt.Errorf("modelbench import: %w", err)
	}
	if len(run.Results) == 0 {
		return nil, fmt.Errorf("modelbench import: run %q has no results", run.RunID)
	}
	modelA, modelB := run.ModelA, run.ModelB
	if modelA == "" {
		modelA = "model-a"
	}
	if modelB == "" {
		modelB = "model-b"
	}
	out := make([]usecase.MatchupInput, 0, len(run.Results))
	for _, res := range run.Results {
		out = append(out, usecase.MatchupInput{
			Prompt:     res.Prompt,
			Category:   res.Category,
			CandidateA: domain.CandidateOutput{SourceLabel: modelA, Text: res.OutputA},
			CandidateB: domain.CandidateOutput{SourceLabel: modelB, Text: res.OutputB},
		})
	}
	return out, nil
}
