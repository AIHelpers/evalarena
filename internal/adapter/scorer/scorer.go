// Package scorer implements the scorer adapter interfaces from
// usecase/ports.go.
package scorer

import (
	"bytes"
	"errors"
	"os/exec"
	"regexp"
	"strings"

	"evalarena/internal/domain"
)

// ErrGofmtUnavailable is returned when gofmt isn't on PATH.
var ErrGofmtUnavailable = errors.New("gofmt not available on PATH")

// GofmtScorer checks whether code text is gofmt-valid.
type GofmtScorer struct{}

// Valid reports whether code is gofmt-valid.
func (GofmtScorer) Valid(code string) (bool, string, error) {
	// gofmt -d reads from stdin and writes a diff to stdout; it is
	// cross-platform and doesn't depend on /dev/stdin or temp files.
	cmd := exec.Command("gofmt", "-d")
	cmd.Stdin = bytes.NewReader([]byte(code))
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		var execErr *exec.Error
		if errors.As(err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound) {
			return false, "", ErrGofmtUnavailable
		}
		if _, ok := err.(*exec.ExitError); ok {
			return false, stderr.String(), nil
		}
		return false, stderr.String(), err
	}
	return out.Len() == 0, "", nil
}

var mcLetterRe = regexp.MustCompile(`(?i)\(?([A-D])\)?`)

// MCScorer extracts the letter a multiple-choice response chose.
type MCScorer struct{}

// ParseChoice extracts the letter a multiple-choice response chose.
func (MCScorer) ParseChoice(response string) (string, error) {
	m := mcLetterRe.FindStringSubmatch(strings.TrimSpace(response))
	if m == nil {
		return "", errors.New("no multiple-choice letter (A-D) found")
	}
	return strings.ToUpper(m[1]), nil
}

// RubricHeuristicScorer applies the rubric heuristic.
type RubricHeuristicScorer struct{}

// Score applies the rubric heuristic and returns hits/total.
func (RubricHeuristicScorer) Score(response string, rubric []string) (int, int) {
	return domain.RubricHeuristicScore(response, rubric)
}
