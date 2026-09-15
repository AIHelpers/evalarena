package jsonfile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"evalarena/internal/domain"
)

// DatasetRepository persists golden evaluation datasets as JSON files.
// Implements usecase.DatasetRepository.
type DatasetRepository struct {
	dir string
	mu  sync.Mutex
}

// NewDatasetRepository creates a DatasetRepository rooted at dir.
func NewDatasetRepository(dir string) (*DatasetRepository, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	return &DatasetRepository{dir: dir}, nil
}

func (r *DatasetRepository) path(id string) string {
	return filepath.Join(r.dir, "dataset_"+id+jsonExt)
}

// Save writes ds to its JSON file atomically.
func (r *DatasetRepository) Save(ds *domain.Dataset) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, err := json.MarshalIndent(ds, "", "  ")
	if err != nil {
		return err
	}
	tmp := r.path(ds.ID) + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, r.path(ds.ID))
}

// Get returns a dataset by id, or ErrNotFound.
func (r *DatasetRepository) Get(id string) (*domain.Dataset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, err := os.ReadFile(r.path(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var ds domain.Dataset
	if err := json.Unmarshal(b, &ds); err != nil {
		return nil, err
	}
	return &ds, nil
}

// List returns all datasets.
func (r *DatasetRepository) List() ([]*domain.Dataset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return nil, err
	}
	var out []*domain.Dataset
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != jsonExt {
			continue
		}
		b, err := os.ReadFile(filepath.Join(r.dir, e.Name()))
		if err != nil {
			continue
		}
		var ds domain.Dataset
		if err := json.Unmarshal(b, &ds); err == nil && ds.ID != "" {
			out = append(out, &ds)
		}
	}
	return out, nil
}

// EvalRunRepository persists evaluation runs as JSON files.
// Implements usecase.EvalRunRepository.
type EvalRunRepository struct {
	dir string
	mu  sync.Mutex
}

// NewEvalRunRepository creates an EvalRunRepository rooted at dir.
func NewEvalRunRepository(dir string) (*EvalRunRepository, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	return &EvalRunRepository{dir: dir}, nil
}

func (r *EvalRunRepository) path(id string) string {
	return filepath.Join(r.dir, "run_"+id+jsonExt)
}

// Save writes run to its JSON file atomically.
func (r *EvalRunRepository) Save(run *domain.EvalRun) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return err
	}
	tmp := r.path(run.ID) + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, r.path(run.ID))
}

// Get returns an eval run by id, or ErrNotFound.
func (r *EvalRunRepository) Get(id string) (*domain.EvalRun, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, err := os.ReadFile(r.path(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var run domain.EvalRun
	if err := json.Unmarshal(b, &run); err != nil {
		return nil, err
	}
	return &run, nil
}

// ListByDataset returns eval runs filtered by dataset id.
func (r *EvalRunRepository) ListByDataset(datasetID string) ([]*domain.EvalRun, error) {
	all, err := r.ListAll()
	if err != nil {
		return nil, err
	}
	var out []*domain.EvalRun
	for _, run := range all {
		if run.DatasetID == datasetID {
			out = append(out, run)
		}
	}
	return out, nil
}

// ListAll returns all eval runs.
func (r *EvalRunRepository) ListAll() ([]*domain.EvalRun, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return nil, err
	}
	var out []*domain.EvalRun
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != jsonExt {
			continue
		}
		b, err := os.ReadFile(filepath.Join(r.dir, e.Name()))
		if err != nil {
			continue
		}
		var run domain.EvalRun
		if err := json.Unmarshal(b, &run); err == nil && run.ID != "" {
			out = append(out, &run)
		}
	}
	return out, nil
}
