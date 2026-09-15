package importer_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"evalarena/internal/adapter/importer"
	"evalarena/internal/domain"
)

func tmpJSONL(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "golden.jsonl")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestImport_Success(t *testing.T) {
	c := `{"id":"eval-001","type":"code_generation","category":"concurrency","difficulty":"hard","question":"Q","reference_answer":"Use a mutex","rubric":["uses mutex"],"tags":["go"]}
{"id":"eval-002","type":"multiple_choice","category":"networking","difficulty":"easy","question":"Q2","reference_answer":"","rubric":[],"choices":{"A":"TCP","B":"UDP"},"correct_choice":"A"}`
	p := tmpJSONL(t, c)
	ds, err := (importer.GoldenSetImporter{}).Import("golden v1", p)
	if err != nil {
		t.Fatal(err)
	}
	if ds.Name != "golden v1" || ds.ID != "ds-golden-v1" || ds.Version != "1.0" {
		t.Fatalf("metadata mismatch: %+v", ds)
	}
	if ds.SourcePath != p {
		t.Errorf("SourcePath = %q", ds.SourcePath)
	}
	if len(ds.Items) != 2 {
		t.Fatalf("len items = %d, want 2", len(ds.Items))
	}
	it := ds.Items[0]
	if it.ID != "eval-001" || it.Type != domain.ItemCodeGeneration || it.Rubric[0] != "uses mutex" {
		t.Errorf("item1 mismatch: %+v", it)
	}
	it2 := ds.Items[1]
	if it2.Type != domain.ItemMultipleChoice || it2.CorrectChoice != "A" || it2.Choices["A"] != "TCP" {
		t.Errorf("item2 mismatch: %+v", it2)
	}
}

func TestImport_EmptyLinesAndEmptyFile(t *testing.T) {
	p := tmpJSONL(t, "\n\n{\"id\":\"x\"}\n\n")
	ds, err := (importer.GoldenSetImporter{}).Import("e", p)
	if err != nil {
		t.Fatal(err)
	}
	if len(ds.Items) != 1 {
		t.Errorf("items = %d, want 1", len(ds.Items))
	}
	p2 := tmpJSONL(t, "")
	ds2, err := (importer.GoldenSetImporter{}).Import("e2", p2)
	if err != nil {
		t.Fatal(err)
	}
	if len(ds2.Items) != 0 {
		t.Errorf("empty items = %d, want 0", len(ds2.Items))
	}
}

func TestImport_Errors(t *testing.T) {
	badJSON := tmpJSONL(t, "{\"id\":\"a\"}\nnot-json\n")
	if _, err := (importer.GoldenSetImporter{}).Import("b", badJSON); err == nil {
		t.Error("expected invalid JSON error")
	} else if !strings.Contains(err.Error(), "line") {
		t.Errorf("error should mention line, got %v", err)
	}

	missingID := tmpJSONL(t, `{"type":"code_generation"}`)
	if _, err := (importer.GoldenSetImporter{}).Import("m", missingID); err == nil {
		t.Error("expected missing ID error")
	}

	if _, err := (importer.GoldenSetImporter{}).Import("n", filepath.Join(t.TempDir(), "nope.jsonl")); err == nil {
		t.Error("expected file-not-found error")
	}
}

func TestImport_SpaceToDash(t *testing.T) {
	p := tmpJSONL(t, "")
	ds, err := (importer.GoldenSetImporter{}).Import("multi word name", p)
	if err != nil {
		t.Fatal(err)
	}
	if ds.ID != "ds-multi-word-name" {
		t.Errorf("ID = %q", ds.ID)
	}
}
