// Package judge implements usecase.JudgeModel for the auto-pilot mode: an
// LLM votes first on each matchup, and only low-confidence or disagreement
// cases get routed to human reviewers.
package judge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"evalarena/internal/domain"
)

// AnthropicJudge calls the Anthropic Messages API with a structured
// judging prompt and parses a strict-JSON verdict back out. It requires
// ANTHROPIC_API_KEY to be set; if it isn't, NewAnthropicJudge returns a
// NoopJudge instead so the rest of the app still runs without a judge
// configured.
type AnthropicJudge struct {
	apiKey string
	model  string
	client *http.Client
}

func NewAnthropicJudge(model string) usecaseJudgeModel {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" || model == "" {
		return NoopJudge{}
	}
	if model == "" {
		model = "claude-sonnet-4-6"
	}
	return &AnthropicJudge{apiKey: key, model: model, client: &http.Client{Timeout: 60 * time.Second}}
}

// usecaseJudgeModel avoids an import cycle: adapter -> usecase is fine, but
// declaring the interface locally keeps this file self-documenting about
// exactly which two methods matter.
type usecaseJudgeModel interface {
	Judge(prompt, textA, textB string) (domain.Winner, float64, error)
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

type judgeVerdict struct {
	Winner     string  `json:"winner"` // "A" | "B" | "tie"
	Confidence float64 `json:"confidence"`
}

func (j *AnthropicJudge) Judge(prompt, textA, textB string) (domain.Winner, float64, error) {
	instruction := fmt.Sprintf(`You are judging a blind A/B comparison of two AI model responses to the same prompt.

Prompt:
%s

Response A:
%s

Response B:
%s

Decide which response better addresses the prompt (quality, correctness, helpfulness). Respond with ONLY a JSON object, no other text, in this exact shape:
{"winner": "A" | "B" | "tie", "confidence": <number 0-1>}`, prompt, textA, textB)

	reqBody, err := json.Marshal(anthropicRequest{
		Model:     j.model,
		MaxTokens: 200,
		Messages:  []anthropicMessage{{Role: "user", Content: instruction}},
	})
	if err != nil {
		return domain.WinnerTie, 0, err
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(reqBody))
	if err != nil {
		return domain.WinnerTie, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", j.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := j.client.Do(req)
	if err != nil {
		return domain.WinnerTie, 0, err
	}
	defer resp.Body.Close()

	var ar anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return domain.WinnerTie, 0, err
	}
	if len(ar.Content) == 0 {
		return domain.WinnerTie, 0, fmt.Errorf("empty judge response")
	}

	text := strings.TrimSpace(ar.Content[0].Text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")

	var verdict judgeVerdict
	if err := json.Unmarshal([]byte(strings.TrimSpace(text)), &verdict); err != nil {
		return domain.WinnerTie, 0, fmt.Errorf("parsing judge verdict: %w", err)
	}

	switch strings.ToUpper(verdict.Winner) {
	case "A":
		return domain.WinnerA, verdict.Confidence, nil
	case "B":
		return domain.WinnerB, verdict.Confidence, nil
	default:
		return domain.WinnerTie, verdict.Confidence, nil
	}
}

// NoopJudge always routes to human review (confidence 0), used when no
// judge model is configured.
type NoopJudge struct{}

func (NoopJudge) Judge(prompt, textA, textB string) (domain.Winner, float64, error) {
	return domain.WinnerTie, 0, nil
}
