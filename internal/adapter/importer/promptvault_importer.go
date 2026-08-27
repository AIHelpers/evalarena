package importer

import (
	"encoding/json"
	"fmt"

	"evalarena/internal/domain"
	"evalarena/internal/usecase"
)

// promptVaultComparison is the expected shape of two PromptVault prompt
// *versions'* eval outputs being compared -- closing the loop between
// prompt iteration and human validation.
type promptVaultComparison struct {
	VersionA string `json:"version_a"`
	VersionB string `json:"version_b"`
	Cases    []struct {
		Input    string  `json:"input"`
		Category *string `json:"category,omitempty"`
		OutputA  string  `json:"output_a"`
		OutputB  string  `json:"output_b"`
	} `json:"cases"`
}

type PromptVaultImporter struct{}

func (PromptVaultImporter) Import(raw []byte) ([]usecase.MatchupInput, error) {
	var cmp promptVaultComparison
	if err := json.Unmarshal(raw, &cmp); err != nil {
		return nil, fmt.Errorf("promptvault import: %w", err)
	}
	if len(cmp.Cases) == 0 {
		return nil, fmt.Errorf("promptvault import: no eval cases found")
	}
	va, vb := cmp.VersionA, cmp.VersionB
	if va == "" {
		va = "version-a"
	}
	if vb == "" {
		vb = "version-b"
	}
	out := make([]usecase.MatchupInput, 0, len(cmp.Cases))
	for _, c := range cmp.Cases {
		out = append(out, usecase.MatchupInput{
			Prompt:     c.Input,
			Category:   c.Category,
			CandidateA: domain.CandidateOutput{SourceLabel: va, Text: c.OutputA},
			CandidateB: domain.CandidateOutput{SourceLabel: vb, Text: c.OutputB},
		})
	}
	return out, nil
}
