package judge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// AnthropicRubricJudge implements usecase.RubricJudgeModel: checks a single
// response against a reference answer and rubric — an absolute correctness
// judgment, unlike the pairwise JudgeModel. Requires ANTHROPIC_API_KEY and a
// model id; if unset, NewAnthropicRubricJudge returns a no-op that reports 0
// hits so it never inflates scores.
type AnthropicRubricJudge struct {
	apiKey string
	model  string
	client *http.Client
}

func NewAnthropicRubricJudge(model string) *AnthropicRubricJudge {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" || model == "" {
		return &AnthropicRubricJudge{} // no-op without apiKey/model
	}
	return &AnthropicRubricJudge{apiKey: key, model: model, client: &http.Client{Timeout: 60 * time.Second}}
}

type rubricCriteriaResponse struct {
	Criteria []struct {
		Met bool   `json:"met"`
		Why string `json:"why"`
	} `json:"criteria"`
}

// JudgeAgainstRubric asks the model to evaluate each rubric criterion
// against the response and reference answer, returning strict JSON.
// No key/model -> reports 0 hits so it never silently inflates scores.
func (j *AnthropicRubricJudge) JudgeAgainstRubric(question, referenceAnswer string, rubric []string, response string) (int, int, string, error) {
	if j.apiKey == "" || j.model == "" {
		return 0, len(rubric), "", nil
	}

	var sb strings.Builder
	sb.WriteString("You are grading a single AI response against a reference answer using a rubric.\n\n")
	fmt.Fprintf(&sb, "Question:\n%s\n\n", question)
	fmt.Fprintf(&sb, "Reference answer:\n%s\n\n", referenceAnswer)
	sb.WriteString("Rubric criteria:\n")
	for i, c := range rubric {
		fmt.Fprintf(&sb, "%d. %s\n", i+1, c)
	}
	fmt.Fprintf(&sb, "\nResponse to grade:\n%s\n\n", response)
	sb.WriteString(`For each rubric criterion, decide whether the response met it. Respond with ONLY a JSON object, no other text, in this exact shape:
{"criteria": [{"met": true|false, "why": "short explanation"}, ...]}`)

	reqBody, err := json.Marshal(anthropicRequest{
		Model:     j.model,
		MaxTokens: 1000,
		Messages:  []anthropicMessage{{Role: "user", Content: sb.String()}},
	})
	if err != nil {
		return 0, len(rubric), "", err
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(reqBody))
	if err != nil {
		return 0, len(rubric), "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", j.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := j.client.Do(req)
	if err != nil {
		return 0, len(rubric), "", err
	}
	defer func() { _ = resp.Body.Close() }()

	var ar anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return 0, len(rubric), "", err
	}
	if len(ar.Content) == 0 {
		return 0, len(rubric), "", fmt.Errorf("empty judge response")
	}

	text := strings.TrimSpace(ar.Content[0].Text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")

	var verdict rubricCriteriaResponse
	if err := json.Unmarshal([]byte(strings.TrimSpace(text)), &verdict); err != nil {
		return 0, len(rubric), "", fmt.Errorf("parsing rubric verdict: %w", err)
	}

	hits := 0
	var rationales []string
	for _, c := range verdict.Criteria {
		if c.Met {
			hits++
		}
		if c.Why != "" {
			rationales = append(rationales, c.Why)
		}
	}
	return hits, len(rubric), strings.Join(rationales, "; "), nil
}
