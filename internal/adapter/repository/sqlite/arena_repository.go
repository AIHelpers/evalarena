// Package sqlite implements repository adapters backed by a SQLite database.
package sqlite

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"evalarena/internal/domain"
)

// ArenaRepository implements usecase.ArenaRepository against the
// arenas/matchups/votes SQLite tables. The JSON-file adapter satisfies
// the same interface; usecase code can't tell them apart.
type ArenaRepository struct {
	db *sql.DB
}

// NewArenaRepository creates an ArenaRepository backed by db.
func NewArenaRepository(db *sql.DB) *ArenaRepository {
	return &ArenaRepository{db: db}
}

func (r *ArenaRepository) saveMatchup(tx *sql.Tx, arenaID string, m domain.Matchup) error {
	candidatesJSON, err := marshalIf(m.Candidates)
	if err != nil {
		return err
	}
	judgeVoteJSON, err := marshalVote(m.JudgeVote)
	if err != nil {
		return err
	}
	posSwappedJSON, err := marshalIf(m.PositionSwapped)
	if err != nil {
		return err
	}
	reviewerOrderJSON, err := marshalIf(m.ReviewerOrder)
	if err != nil {
		return err
	}

	var category sql.NullString
	if m.Category != nil {
		category.String = *m.Category
		category.Valid = true
	}

	if _, err := tx.Exec(`INSERT INTO matchups(
		id, arena_id, prompt, category,
		candidate_a_label, candidate_a_text, candidate_b_label, candidate_b_text,
		candidates_json, judge_vote_json, position_swapped_json, reviewer_order_json
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, arenaID, m.Prompt, category,
		m.CandidateA.SourceLabel, m.CandidateA.Text,
		m.CandidateB.SourceLabel, m.CandidateB.Text,
		candidatesJSON, judgeVoteJSON, posSwappedJSON, reviewerOrderJSON); err != nil {
		return fmtErr("insert matchup: %w", err)
	}

	for _, v := range m.HumanVotes {
		if _, err := tx.Exec(`INSERT INTO votes(
			matchup_id, reviewer_id, winner, confidence, is_judge, cast_at
		) VALUES (?, ?, ?, ?, ?, ?)`,
			m.ID, v.ReviewerID, string(v.Winner), v.Confidence, v.IsJudge,
			v.CastAt.UTC().Format(time.RFC3339Nano)); err != nil {
			return fmtErr("insert vote: %w", err)
		}
	}
	return nil
}

func (r *ArenaRepository) scanMatchup(scan func(dest ...any) error) (domain.Matchup, error) {
	var m domain.Matchup
	var category, aLabel, aText, bLabel, bText, candidatesJSON, judgeVoteJSON, posSwappedJSON, reviewerOrderJSON sql.NullString
	if err := scan(&m.ID, &m.Prompt, &category,
		&aLabel, &aText, &bLabel, &bText,
		&candidatesJSON, &judgeVoteJSON, &posSwappedJSON, &reviewerOrderJSON); err != nil {
		return m, err
	}
	if category.Valid {
		c := category.String
		m.Category = &c
	}
	m.CandidateA = domain.CandidateOutput{SourceLabel: aLabel.String, Text: aText.String}
	m.CandidateB = domain.CandidateOutput{SourceLabel: bLabel.String, Text: bText.String}
	if err := unmarshalJSON(candidatesJSON, &m.Candidates); err != nil {
		return m, err
	}
	if judgeVoteJSON.Valid {
		var v domain.Vote
		if err := json.Unmarshal([]byte(judgeVoteJSON.String), &v); err != nil {
			return m, err
		}
		m.JudgeVote = &v
	}
	if err := unmarshalJSON(posSwappedJSON, &m.PositionSwapped); err != nil {
		return m, err
	}
	if err := unmarshalJSON(reviewerOrderJSON, &m.ReviewerOrder); err != nil {
		return m, err
	}
	return m, nil
}

// Save persists an arena (insert or update) and returns an error if the
// record could not be written.
func (r *ArenaRepository) Save(arena *domain.Arena) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`INSERT INTO arenas(id, name, mode, created_at) VALUES (?, ?, ?, ?)`,
		arena.ID, arena.Name, string(arena.Mode), arena.CreatedAt.UTC().Format(time.RFC3339Nano)); err != nil {
		return fmtErr("insert arena: %w", err)
	}

	for _, m := range arena.Matchups {
		if err := r.saveMatchup(tx, arena.ID, m); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *ArenaRepository) loadMatchups(arena *domain.Arena) error {
	rows, err := r.db.Query(`SELECT
		id, prompt, category,
		candidate_a_label, candidate_a_text, candidate_b_label, candidate_b_text,
		candidates_json, judge_vote_json, position_swapped_json, reviewer_order_json
		FROM matchups WHERE arena_id = ? ORDER BY rowid`, arena.ID)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		m, err := r.scanMatchup(rows.Scan)
		if err != nil {
			return err
		}
		votes, err := r.loadVotes(m.ID)
		if err != nil {
			return err
		}
		m.HumanVotes = votes
		arena.Matchups = append(arena.Matchups, m)
	}
	return rows.Err()
}

// Get returns the arena with the given id, or ErrNotFound.
func (r *ArenaRepository) Get(id string) (*domain.Arena, error) {
	row := r.db.QueryRow(`SELECT id, name, mode, created_at FROM arenas WHERE id = ?`, id)
	var arena domain.Arena
	var mode, createdAtStr string
	if err := row.Scan(&arena.ID, &arena.Name, &mode, &createdAtStr); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	arena.Mode = domain.Mode(mode)
	arena.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAtStr)

	if err := r.loadMatchups(&arena); err != nil {
		return nil, err
	}
	return &arena, nil
}

func (r *ArenaRepository) loadVotes(matchupID string) ([]domain.Vote, error) {
	rows, err := r.db.Query(`SELECT reviewer_id, winner, confidence, is_judge, cast_at FROM votes WHERE matchup_id = ? ORDER BY id`, matchupID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []domain.Vote
	for rows.Next() {
		var v domain.Vote
		var winner, castAtStr string
		var confidence sql.NullFloat64
		if err := rows.Scan(&v.ReviewerID, &winner, &confidence, &v.IsJudge, &castAtStr); err != nil {
			return nil, err
		}
		v.Winner = domain.Winner(winner)
		v.Confidence = confidence.Float64
		v.CastAt, _ = time.Parse(time.RFC3339Nano, castAtStr)
		out = append(out, v)
	}
	return out, rows.Err()
}

// List returns all arenas sorted by creation time, newest first.
func (r *ArenaRepository) List() ([]*domain.Arena, error) {
	rows, err := r.db.Query(`SELECT id FROM arenas ORDER BY created_at DESC`)
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
	out := make([]*domain.Arena, 0, len(ids))
	for _, id := range ids {
		a, err := r.Get(id)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

// Delete removes an arena by id, returning ErrNotFound if it does not exist.
func (r *ArenaRepository) Delete(id string) error {
	res, err := r.db.Exec(`DELETE FROM arenas WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ReviewerRepository implements usecase.ReviewerRepository against the
// reviewers SQLite table.
type ReviewerRepository struct {
	db *sql.DB
}

// NewReviewerRepository creates a ReviewerRepository backed by db.
func NewReviewerRepository(db *sql.DB) *ReviewerRepository {
	return &ReviewerRepository{db: db}
}

// Save inserts or updates a reviewer.
func (r *ReviewerRepository) Save(rev *domain.Reviewer) error {
	_, err := r.db.Exec(`INSERT INTO reviewers(id, name, votes_cast, agreement_rate, created_at) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET name = excluded.name, votes_cast = excluded.votes_cast, agreement_rate = excluded.agreement_rate`,
		rev.ID, rev.DisplayName, rev.VotesCast, rev.AgreementRate, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

// Get returns a reviewer by id, or ErrNotFound.
func (r *ReviewerRepository) Get(id string) (*domain.Reviewer, error) {
	row := r.db.QueryRow(`SELECT id, name, votes_cast, agreement_rate FROM reviewers WHERE id = ?`, id)
	var rev domain.Reviewer
	if err := row.Scan(&rev.ID, &rev.DisplayName, &rev.VotesCast, &rev.AgreementRate); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &rev, nil
}

// List returns all reviewers sorted by name.
func (r *ReviewerRepository) List() ([]*domain.Reviewer, error) {
	rows, err := r.db.Query(`SELECT id, name, votes_cast, agreement_rate FROM reviewers ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*domain.Reviewer
	for rows.Next() {
		var rev domain.Reviewer
		if err := rows.Scan(&rev.ID, &rev.DisplayName, &rev.VotesCast, &rev.AgreementRate); err != nil {
			return nil, err
		}
		out = append(out, &rev)
	}
	return out, rows.Err()
}
