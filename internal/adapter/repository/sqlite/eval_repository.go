package sqlite

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"evalarena/internal/domain"
)

// DatasetRepository implements usecase.DatasetRepository against the
// SQLite datasets/dataset_items tables.
type DatasetRepository struct {
	db *sql.DB
}

// NewDatasetRepository creates a DatasetRepository backed by db.
func NewDatasetRepository(db *sql.DB) *DatasetRepository {
	return &DatasetRepository{db: db}
}

// Save inserts a dataset and its items in a transaction.
func (r *DatasetRepository) Save(ds *domain.Dataset) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`INSERT INTO datasets(id, name, version, source_path, imported_at) VALUES (?, ?, ?, ?, ?)`,
		ds.ID, ds.Name, ds.Version, ds.SourcePath, ds.ImportedAt.UTC().Format(time.RFC3339Nano)); err != nil {
		return fmtErr("insert dataset: %w", err)
	}
	for _, it := range ds.Items {
		rubricJSON, err := json.Marshal(it.Rubric)
		if err != nil {
			return err
		}
		var choicesJSON []byte
		if it.Choices != nil {
			choicesJSON, err = json.Marshal(it.Choices)
			if err != nil {
				return err
			}
		}
		tagsJSON, err := json.Marshal(it.Tags)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO dataset_items(
			dataset_id, id, type, category, difficulty, question, reference_answer,
			rubric_json, choices_json, correct_choice, tags_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			ds.ID, it.ID, string(it.Type), it.Category, it.Difficulty,
			it.Question, it.ReferenceAnswer,
			rubricJSON, choicesJSON, it.CorrectChoice, tagsJSON); err != nil {
			return fmtErr("insert dataset item: %w", err)
		}
	}
	return tx.Commit()
}

// Get returns a dataset by id, or ErrNotFound.
func (r *DatasetRepository) Get(id string) (*domain.Dataset, error) {
	row := r.db.QueryRow(`SELECT id, name, version, source_path, imported_at FROM datasets WHERE id = ?`, id)
	var ds domain.Dataset
	var version, sourcePath sql.NullString
	var importedAtStr string
	if err := row.Scan(&ds.ID, &ds.Name, &version, &sourcePath, &importedAtStr); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	ds.Version = version.String
	ds.SourcePath = sourcePath.String
	ds.ImportedAt, _ = time.Parse(time.RFC3339Nano, importedAtStr)

	rows, err := r.db.Query(`SELECT
		id, type, category, difficulty, question, reference_answer,
		rubric_json, choices_json, correct_choice, tags_json
		FROM dataset_items WHERE dataset_id = ?`, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var it domain.DatasetItem
		var itType, choicesJSON, correctChoice, tagsJSON sql.NullString
		var rubricJSON string
		if err := rows.Scan(&it.ID, &itType, &it.Category, &it.Difficulty,
			&it.Question, &it.ReferenceAnswer,
			&rubricJSON, &choicesJSON, &correctChoice, &tagsJSON); err != nil {
			return nil, err
		}
		it.Type = domain.ItemType(itType.String)
		if err := json.Unmarshal([]byte(rubricJSON), &it.Rubric); err != nil {
			return nil, err
		}
		if choicesJSON.Valid {
			if err := json.Unmarshal([]byte(choicesJSON.String), &it.Choices); err != nil {
				return nil, err
			}
		}
		it.CorrectChoice = correctChoice.String
		if tagsJSON.Valid {
			if err := json.Unmarshal([]byte(tagsJSON.String), &it.Tags); err != nil {
				return nil, err
			}
		}
		ds.Items = append(ds.Items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &ds, nil
}

// List returns all datasets, newest first.
func (r *DatasetRepository) List() ([]*domain.Dataset, error) {
	rows, err := r.db.Query(`SELECT id FROM datasets ORDER BY imported_at DESC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]*domain.Dataset, 0, len(ids))
	for _, id := range ids {
		ds, err := r.Get(id)
		if err != nil {
			return nil, err
		}
		out = append(out, ds)
	}
	return out, nil
}

// EvalRunRepository implements usecase.EvalRunRepository against the
// SQLite eval_runs/eval_item_results tables.
type EvalRunRepository struct {
	db *sql.DB
}

// NewEvalRunRepository creates an EvalRunRepository backed by db.
func NewEvalRunRepository(db *sql.DB) *EvalRunRepository {
	return &EvalRunRepository{db: db}
}

// Save inserts an eval run and its item results in a transaction.
func (r *EvalRunRepository) Save(run *domain.EvalRun) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`INSERT INTO eval_runs(id, dataset_id, model_label, scorer_mode, created_at, notes) VALUES (?, ?, ?, ?, ?, ?)`,
		run.ID, run.DatasetID, run.ModelLabel, string(run.ScorerMode), run.CreatedAt.UTC().Format(time.RFC3339Nano), run.Notes); err != nil {
		return fmtErr("insert eval run: %w", err)
	}
	for _, res := range run.Results {
		if err := insertItemResult(tx, run.ID, &res); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func insertItemResult(tx *sql.Tx, runID string, res *domain.ItemResult) error {
	gofmtValid := sql.NullBool{}
	if res.GofmtValid != nil {
		gofmtValid.Bool = *res.GofmtValid
		gofmtValid.Valid = true
	}
	mcCorrect := sql.NullBool{}
	if res.MCCorrect != nil {
		mcCorrect.Bool = *res.MCCorrect
		mcCorrect.Valid = true
	}
	rationale := sql.NullString{}
	if res.JudgeRationale != "" {
		rationale.String = res.JudgeRationale
		rationale.Valid = true
	}
	_, err := tx.Exec(`INSERT INTO eval_item_results(
		run_id, item_id, response, gofmt_valid, mc_chosen, mc_correct,
		rubric_hits, rubric_total, score, scorer_type, judge_rationale, scored_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		runID, res.ItemID, res.Response, gofmtValid, res.MCChosen, mcCorrect,
		res.RubricHits, res.RubricTotal, res.Score, string(res.ScorerType), rationale,
		res.ScoredAt.UTC().Format(time.RFC3339Nano))
	return err
}

// Get returns an eval run by id, or ErrNotFound.
func (r *EvalRunRepository) Get(id string) (*domain.EvalRun, error) {
	row := r.db.QueryRow(`SELECT id, dataset_id, model_label, scorer_mode, created_at, notes FROM eval_runs WHERE id = ?`, id)
	var run domain.EvalRun
	var scorerMode, createdAtStr string
	var notes sql.NullString
	if err := row.Scan(&run.ID, &run.DatasetID, &run.ModelLabel, &scorerMode, &createdAtStr, &notes); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	run.ScorerMode = domain.ScorerType(scorerMode)
	run.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAtStr)
	run.Notes = notes.String

	results, err := r.loadResults(run.ID)
	if err != nil {
		return nil, err
	}
	run.Results = results
	return &run, nil
}

func (r *EvalRunRepository) loadResults(runID string) ([]domain.ItemResult, error) {
	rows, err := r.db.Query(`SELECT
		item_id, response, gofmt_valid, mc_chosen, mc_correct,
		rubric_hits, rubric_total, score, scorer_type, judge_rationale, scored_at
		FROM eval_item_results WHERE run_id = ? ORDER BY item_id`, runID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []domain.ItemResult
	for rows.Next() {
		var res domain.ItemResult
		var gofmtValid, mcCorrect sql.NullBool
		var mcChosen, scorerType, rationale sql.NullString
		var scoredAtStr string
		if err := rows.Scan(&res.ItemID, &res.Response, &gofmtValid, &mcChosen, &mcCorrect,
			&res.RubricHits, &res.RubricTotal, &res.Score, &scorerType, &rationale, &scoredAtStr); err != nil {
			return nil, err
		}
		if gofmtValid.Valid {
			v := gofmtValid.Bool
			res.GofmtValid = &v
		}
		res.MCChosen = mcChosen.String
		if mcCorrect.Valid {
			v := mcCorrect.Bool
			res.MCCorrect = &v
		}
		res.ScorerType = domain.ScorerType(scorerType.String)
		res.JudgeRationale = rationale.String
		res.ScoredAt, _ = time.Parse(time.RFC3339Nano, scoredAtStr)
		out = append(out, res)
	}
	return out, rows.Err()
}

// ListByDataset returns eval runs filtered by dataset id.
func (r *EvalRunRepository) ListByDataset(datasetID string) ([]*domain.EvalRun, error) {
	rows, err := r.db.Query(`SELECT id FROM eval_runs WHERE dataset_id = ? ORDER BY created_at DESC`, datasetID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]*domain.EvalRun, 0, len(ids))
	for _, id := range ids {
		run, err := r.Get(id)
		if err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, nil
}

// ListAll returns all eval runs, newest first.
func (r *EvalRunRepository) ListAll() ([]*domain.EvalRun, error) {
	rows, err := r.db.Query(`SELECT id FROM eval_runs ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]*domain.EvalRun, 0, len(ids))
	for _, id := range ids {
		run, err := r.Get(id)
		if err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, nil
}
