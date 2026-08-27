package usecase

import "evalarena/internal/domain"

// Importer parses an external tool's export format (ModelBench-Local run,
// Distillery job result, PromptVault eval output) into arena-ready matchup
// inputs. Concrete implementations live in internal/adapter/importer.
type Importer interface {
	Import(raw []byte) ([]MatchupInput, error)
}

// ImportFromSourceUseCase wraps CreateArenaUseCase with a specific Importer,
// so "evalarena create --from-modelbench run123" and friends share one code
// path that only differs by which Importer is plugged in.
type ImportFromSourceUseCase struct {
	Repo     ArenaRepository
	Random   RandomSource
	Importer Importer
}

func NewImportFromSourceUseCase(repo ArenaRepository, random RandomSource, importer Importer) *ImportFromSourceUseCase {
	return &ImportFromSourceUseCase{Repo: repo, Random: random, Importer: importer}
}

func (uc *ImportFromSourceUseCase) Execute(name string, mode domain.Mode, raw []byte) (*domain.Arena, error) {
	matchups, err := uc.Importer.Import(raw)
	if err != nil {
		return nil, err
	}
	create := NewCreateArenaUseCase(uc.Repo, uc.Random)
	return create.Execute(CreateArenaInput{Name: name, Mode: mode, Matchups: matchups})
}
