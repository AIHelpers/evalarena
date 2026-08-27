// Package jsonfile implements the usecase.ArenaRepository and
// usecase.ReviewerRepository ports with plain JSON files on disk.
//
// The development plan specifies SQLite for solo/small-team use and
// Postgres for shared server deployments, both behind the same repository
// interface. This adapter satisfies that same interface with zero external
// dependencies (no cgo, no driver download), which matters in network-
// restricted build environments. Swapping in a real sqlite/postgres
// implementation later is a matter of writing another adapter package that
// implements usecase.ArenaRepository/ReviewerRepository -- no usecase or
// handler code changes.
package jsonfile

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"evalarena/internal/domain"
)

var ErrNotFound = errors.New("not found")

type ArenaRepository struct {
	dir string
	mu  sync.Mutex
}

func NewArenaRepository(dir string) (*ArenaRepository, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &ArenaRepository{dir: dir}, nil
}

func (r *ArenaRepository) path(id string) string {
	return filepath.Join(r.dir, id+".json")
}

func (r *ArenaRepository) Save(arena *domain.Arena) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, err := json.MarshalIndent(arena, "", "  ")
	if err != nil {
		return err
	}
	tmp := r.path(arena.ID) + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, r.path(arena.ID))
}

func (r *ArenaRepository) Get(id string) (*domain.Arena, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, err := os.ReadFile(r.path(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var arena domain.Arena
	if err := json.Unmarshal(b, &arena); err != nil {
		return nil, err
	}
	return &arena, nil
}

func (r *ArenaRepository) List() ([]*domain.Arena, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return nil, err
	}
	var out []*domain.Arena
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(r.dir, e.Name()))
		if err != nil {
			continue
		}
		var arena domain.Arena
		if err := json.Unmarshal(b, &arena); err != nil {
			continue
		}
		out = append(out, &arena)
	}
	return out, nil
}

func (r *ArenaRepository) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	err := os.Remove(r.path(id))
	if os.IsNotExist(err) {
		return ErrNotFound
	}
	return err
}
