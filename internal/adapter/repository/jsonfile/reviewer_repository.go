package jsonfile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"evalarena/internal/domain"
)

// ReviewerRepository persists reviewers as JSON files.
type ReviewerRepository struct {
	dir string
	mu  sync.Mutex
}

// NewReviewerRepository creates a ReviewerRepository rooted at dir.
func NewReviewerRepository(dir string) (*ReviewerRepository, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	return &ReviewerRepository{dir: dir}, nil
}

func (r *ReviewerRepository) path(id string) string {
	return filepath.Join(r.dir, "reviewer_"+id+jsonExt)
}

// Save writes rev to its JSON file.
func (r *ReviewerRepository) Save(rev *domain.Reviewer) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, err := json.MarshalIndent(rev, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.path(rev.ID), b, 0o600)
}

// Get returns a reviewer by id, or ErrNotFound.
func (r *ReviewerRepository) Get(id string) (*domain.Reviewer, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, err := os.ReadFile(r.path(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var rev domain.Reviewer
	if err := json.Unmarshal(b, &rev); err != nil {
		return nil, err
	}
	return &rev, nil
}

// List returns all reviewers.
func (r *ReviewerRepository) List() ([]*domain.Reviewer, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return nil, err
	}
	var out []*domain.Reviewer
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != jsonExt {
			continue
		}
		b, err := os.ReadFile(filepath.Join(r.dir, e.Name()))
		if err != nil {
			continue
		}
		var rev domain.Reviewer
		if err := json.Unmarshal(b, &rev); err == nil && rev.ID != "" {
			out = append(out, &rev)
		}
	}
	return out, nil
}
