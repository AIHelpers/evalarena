package importer

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"evalarena/internal/domain"
)

// GoldenSetImporter parses golden_eval_set.jsonl (one JSON object per line,
// each matching domain.DatasetItem) into a domain.Dataset.
type GoldenSetImporter struct{}

func (GoldenSetImporter) Import(name, path string) (*domain.Dataset, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var items []domain.DatasetItem
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var it domain.DatasetItem
		if err := json.Unmarshal([]byte(line), &it); err != nil {
			return nil, fmt.Errorf("line %q: %w", line, err)
		}
		if it.ID == "" {
			return nil, fmt.Errorf("item missing id: %q", line)
		}
		items = append(items, it)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	return &domain.Dataset{
		ID:         "ds-" + strings.ReplaceAll(name, " ", "-"),
		Name:       name,
		Version:    "1.0",
		SourcePath: path,
		Items:      items,
	}, nil
}

// Ensure GoldenSetImporter satisfies the importer contract used by CLI.
var _ = GoldenSetImporter{}
