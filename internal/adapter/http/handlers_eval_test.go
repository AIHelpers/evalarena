package http_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	h "evalarena/internal/adapter/http"
	"evalarena/internal/adapter/scorer"
	"evalarena/internal/domain"
	"evalarena/internal/usecase"
)

const (
	testDSID  = "ds-1"
	testRunID = "run-1"
	testRun2  = "run-2"
)

type fakeEvalDS struct{ m map[string]*domain.Dataset }

func (f *fakeEvalDS) Save(d *domain.Dataset) error { f.m[d.ID] = d; return nil }
func (f *fakeEvalDS) Get(id string) (*domain.Dataset, error) {
	if d, ok := f.m[id]; ok {
		return d, nil
	}
	return nil, errors.New("not found")
}
func (f *fakeEvalDS) List() ([]*domain.Dataset, error) {
	out := make([]*domain.Dataset, 0, len(f.m))
	for _, d := range f.m {
		out = append(out, d)
	}
	return out, nil
}

type fakeEvalRun struct{ m map[string]*domain.EvalRun }

func (f *fakeEvalRun) Save(r *domain.EvalRun) error { f.m[r.ID] = r; return nil }
func (f *fakeEvalRun) Get(id string) (*domain.EvalRun, error) {
	if r, ok := f.m[id]; ok {
		return r, nil
	}
	return nil, errors.New("not found")
}
func (f *fakeEvalRun) ListByDataset(_ string) ([]*domain.EvalRun, error) { return nil, nil }
func (f *fakeEvalRun) ListAll() ([]*domain.EvalRun, error) {
	out := make([]*domain.EvalRun, 0, len(f.m))
	for _, r := range f.m {
		out = append(out, r)
	}
	return out, nil
}

func newTestServer() *h.Server {
	ds := &fakeEvalDS{m: map[string]*domain.Dataset{
		testDSID: {ID: testDSID, Name: "d", Items: []domain.DatasetItem{
			{ID: "i1", Type: domain.ItemCodeGeneration, Category: "c", Rubric: []string{"mutex"}},
		}},
	}}
	rs := &fakeEvalRun{m: map[string]*domain.EvalRun{
		testRunID: {
			ID: testRunID, DatasetID: testDSID, ModelLabel: "m1",
			ScorerMode: domain.ScorerAutomated,
			Results:    []domain.ItemResult{{ItemID: "i1", Score: 0.8}},
		},
		testRun2: {
			ID: testRun2, DatasetID: testDSID, ModelLabel: "m2",
			ScorerMode: domain.ScorerAutomated,
			Results:    []domain.ItemResult{{ItemID: "i1", Score: 0.2}},
		},
	}}
	return &h.Server{
		Datasets:    ds,
		EvalRuns:    rs,
		EvalSummary: usecase.NewComputeEvalSummaryUseCase(ds, rs),
		EvalCompare: usecase.NewCompareEvalRunsUseCase(ds, rs),
		RunEval: usecase.NewRunEvaluationUseCase(
			ds, rs,
			scorer.GofmtScorer{}, scorer.MCScorer{}, scorer.RubricHeuristicScorer{}, nil,
		),
	}
}

func TestHandleListEvalRuns(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest("GET", "/api/eval-runs", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var runs []struct {
		ID         string   `json:"id"`
		ModelLabel string   `json:"model_label"`
		Overall    *float64 `json:"overall"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &runs); err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 {
		t.Fatalf("runs = %d, want 2", len(runs))
	}
	found := false
	for _, r := range runs {
		if r.ID == testRunID && r.Overall != nil && *r.Overall == 0.8 {
			found = true
		}
	}
	if !found {
		t.Errorf("expected %s with overall 0.8, got %+v", testRunID, runs)
	}
}

func TestHandleGetEvalRun(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest("GET", "/api/eval-runs/"+testRunID, nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var out struct {
		ID         string             `json:"id"`
		ModelLabel string             `json:"model_label"`
		Summary    *domain.RunSummary `json:"summary"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Summary == nil || out.Summary.Overall != 0.8 {
		t.Errorf("summary = %+v, want overall 0.8", out.Summary)
	}
}

func TestHandleGetEvalRun_NotFound(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest("GET", "/api/eval-runs/nope", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestHandleCompareEvalRuns(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest("GET", "/api/eval-runs/compare?ids="+testRunID+","+testRun2, nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var out usecase.RunComparison
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Regressions) != 1 {
		t.Fatalf("regressions = %+v, want 1", out.Regressions)
	}
	if d := out.Regressions[0].Delta; d < -0.600001 || d > -0.599999 {
		t.Errorf("delta = %v, want ~-0.6", d)
	}
}

func TestHandleCompareEvalRuns_MissingIDs(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest("GET", "/api/eval-runs/compare", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandleCompareEvalRuns_TooFew(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest("GET", "/api/eval-runs/compare?ids="+testRunID, nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandleListDatasets(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest("GET", "/api/datasets", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var out []struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		ItemCount int    `json:"item_count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].ID != testDSID || out[0].ItemCount != 1 {
		t.Errorf("datasets = %+v, want 1 dataset %s with 1 item", out, testDSID)
	}
}

func TestHandleGetDataset(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest("GET", "/api/datasets/"+testDSID, nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var ds domain.Dataset
	if err := json.Unmarshal(rec.Body.Bytes(), &ds); err != nil {
		t.Fatal(err)
	}
	if len(ds.Items) != 1 || ds.Items[0].ID != "i1" {
		t.Errorf("dataset items = %+v, want 1 item i1", ds.Items)
	}
}

func TestHandleGetDataset_NotFound(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest("GET", "/api/datasets/nope", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestHandleCreateEvalRun(t *testing.T) {
	srv := newTestServer()
	payload := `{"dataset_id":"` + testDSID + `","model_label":"model-x","responses":{"i1":"package main"},"notes":"from UI"}`
	req := httptest.NewRequest("POST", "/api/eval-runs", bytes.NewReader([]byte(payload)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body=%s", rec.Code, rec.Body.String())
	}
	var out struct {
		Run     *domain.EvalRun    `json:"run"`
		Summary *domain.RunSummary `json:"summary"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Run == nil || out.Run.DatasetID != testDSID || out.Run.ModelLabel != "model-x" {
		t.Errorf("run = %+v", out.Run)
	}
	if out.Summary == nil {
		t.Error("summary missing")
	}
	// Ensure the run was persisted.
	if got, err := srv.EvalRuns.Get(out.Run.ID); err != nil || got == nil {
		t.Errorf("run not saved: err=%v", err)
	}
}

func TestHandleCreateEvalRun_Validation(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest("POST", "/api/eval-runs", bytes.NewReader([]byte(`{"dataset_id":"","model_label":"x"}`)))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandleCreateEvalRun_EmptyResponses(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest("POST", "/api/eval-runs", bytes.NewReader([]byte(`{"dataset_id":"`+testDSID+`","model_label":"x"}`)))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}
