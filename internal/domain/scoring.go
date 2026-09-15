package domain

import "strings"

// RubricHeuristicScore ports evaluate.py's score_rubric_heuristic: crude
// keyword-presence check per rubric criterion. Same caveats apply as in
// the Python version's docstring — this estimates coverage, it does not
// verify correctness.
//
// Each rubric criterion is a short phrase like "function returns a
// channel". We split it into significant words (drop stopwords, <=2 chars,
// and words already checked), and count the criterion as "hit" if any
// significant keyword appears in the response text. Returns (hits, total).
func RubricHeuristicScore(responseText string, rubric []string) (hits, total int) {
	norm := strings.ToLower(strings.TrimSpace(responseText))
	total = len(rubric)
	if total == 0 {
		return 0, 0
	}
	seen := map[string]bool{}
	for _, crit := range rubric {
		for _, word := range significantWords(crit) {
			if seen[word] {
				continue
			}
			seen[word] = true
			if strings.Contains(norm, word) {
				hits++
				break
			}
		}
	}
	return hits, total
}

// significantWords drops obvious stopwords/particles and words <=2 chars,
// then lowercases and returns the rest. Mirrors evaluate.py's crude
// keyword extraction.
func significantWords(s string) []string {
	stopwords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"of": true, "to": true, "for": true, "in": true, "on": true,
		"with": true, "that": true, "is": true, "are": true, "be": true,
		"should": true, "must": true, "can": true, "use": true, "using": true,
		"return": true, "returns": true, "function": true, "code": true,
	}
	var out []string
	for _, w := range strings.Fields(strings.ToLower(s)) {
		w = strings.Trim(w, ".,;:!?()[]{}\"'`")
		if len(w) <= 2 || stopwords[w] {
			continue
		}
		out = append(out, w)
	}
	return out
}

// CombinedScore ports evaluate.py's 0.4*gofmt + 0.6*rubric_fraction
// weighting for code_generation/bug_fix items. For non-code items the
// caller decides the score directly (e.g. 1.0 for a correct MC answer).
func CombinedScore(gofmtValid bool, rubricHits, rubricTotal int) float64 {
	gofmtPart := 0.0
	if gofmtValid {
		gofmtPart = 1.0
	}
	rubricPart := 0.0
	if rubricTotal > 0 {
		rubricPart = float64(rubricHits) / float64(rubricTotal)
	}
	return 0.4*gofmtPart + 0.6*rubricPart
}

// RunSummary is the aggregate of an evaluation run's scores, broken down
// overall and by type / category / difficulty.
type RunSummary struct {
	Overall      float64            `json:"overall"`
	ByType       map[string]float64 `json:"by_type"`
	ByCategory   map[string]float64 `json:"by_category"`
	ByDifficulty map[string]float64 `json:"by_difficulty"`
	N            int                `json:"n"`
}

// Summarize aggregates a run's ItemResults into a RunSummary, using the
// item metadata map to group by type/category/difficulty. Items missing
// from the map are still counted in the overall average.
func Summarize(results []ItemResult, items map[string]DatasetItem) RunSummary {
	byType := map[string][]float64{}
	byCat := map[string][]float64{}
	byDiff := map[string][]float64{}

	var total float64
	for _, r := range results {
		total += r.Score
		it, ok := items[r.ItemID]
		if !ok {
			// Unknown item: still counts in overall, grouped under "unknown".
			byType["unknown"] = append(byType["unknown"], r.Score)
			byCat["unknown"] = append(byCat["unknown"], r.Score)
			byDiff["unknown"] = append(byDiff["unknown"], r.Score)
			continue
		}
		if it.Type != "" {
			byType[string(it.Type)] = append(byType[string(it.Type)], r.Score)
		}
		if it.Category != "" {
			byCat[it.Category] = append(byCat[it.Category], r.Score)
		} else {
			byCat["uncategorized"] = append(byCat["uncategorized"], r.Score)
		}
		if it.Difficulty != "" {
			byDiff[it.Difficulty] = append(byDiff[it.Difficulty], r.Score)
		}
	}

	s := RunSummary{
		ByType:       avgMap(byType),
		ByCategory:   avgMap(byCat),
		ByDifficulty: avgMap(byDiff),
		N:            len(results),
	}
	if s.N > 0 {
		s.Overall = total / float64(s.N)
	}
	return s
}

func avgMap(m map[string][]float64) map[string]float64 {
	out := make(map[string]float64, len(m))
	for k, vals := range m {
		var sum float64
		for _, v := range vals {
			sum += v
		}
		out[k] = sum / float64(len(vals))
	}
	return out
}
