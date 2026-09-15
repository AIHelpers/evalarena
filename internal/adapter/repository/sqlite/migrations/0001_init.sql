-- 0001_init.sql — initial EvalArena schema.
--
-- Contains both the existing arena/pairwise tables (same shape as the
-- JSON-file adapter today) and the new dataset-evaluation tables. All
-- applied in one transaction by the migration runner.

-- === Existing arena/pairwise data, same shape as the JSON-file adapter ===
CREATE TABLE arenas (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    mode       TEXT NOT NULL,               -- pairwise | tournament
    created_at TIMESTAMP NOT NULL
);

CREATE TABLE matchups (
    id                    TEXT PRIMARY KEY,
    arena_id              TEXT NOT NULL REFERENCES arenas(id) ON DELETE CASCADE,
    prompt                TEXT NOT NULL,
    category              TEXT,
    candidate_a_label     TEXT,
    candidate_a_text      TEXT,
    candidate_b_label     TEXT,
    candidate_b_text     TEXT,
    candidates_json       TEXT,               -- tournament mode, JSON array
    judge_vote_json       TEXT,
    position_swapped_json TEXT,
    reviewer_order_json   TEXT
);
CREATE INDEX idx_matchups_arena    ON matchups(arena_id);
CREATE INDEX idx_matchups_category ON matchups(category);

CREATE TABLE votes (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    matchup_id  TEXT NOT NULL REFERENCES matchups(id) ON DELETE CASCADE,
    reviewer_id TEXT NOT NULL,
    winner      TEXT NOT NULL,               -- A | B | tie
    confidence  REAL,
    is_judge    BOOLEAN NOT NULL DEFAULT 0,
    cast_at     TIMESTAMP NOT NULL
);
CREATE INDEX idx_votes_matchup ON votes(matchup_id);

CREATE TABLE reviewers (
    id             TEXT PRIMARY KEY,
    name           TEXT NOT NULL,
    votes_cast     INTEGER NOT NULL DEFAULT 0,
    agreement_rate REAL NOT NULL DEFAULT 0,
    created_at     TIMESTAMP NOT NULL
);

CREATE TABLE datasets (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    version     TEXT,
    source_path TEXT,
    imported_at TIMESTAMP NOT NULL
);

CREATE TABLE dataset_items (
    dataset_id       TEXT NOT NULL REFERENCES datasets(id) ON DELETE CASCADE,
    id               TEXT NOT NULL,          -- golden item id, e.g. "eval-cg-001"
    type             TEXT NOT NULL,
    category         TEXT NOT NULL,
    difficulty       TEXT NOT NULL,
    question         TEXT NOT NULL,
    reference_answer TEXT NOT NULL,
    rubric_json      TEXT NOT NULL,          -- JSON array of strings
    choices_json     TEXT,                   -- multiple_choice only
    correct_choice   TEXT,
    tags_json        TEXT,
    PRIMARY KEY (dataset_id, id)
);
CREATE INDEX idx_dataset_items_category ON dataset_items(dataset_id, category);
CREATE INDEX idx_dataset_items_type     ON dataset_items(dataset_id, type);

CREATE TABLE eval_runs (
    id          TEXT PRIMARY KEY,
    dataset_id  TEXT NOT NULL REFERENCES datasets(id),
    model_label TEXT NOT NULL,
    scorer_mode TEXT NOT NULL,               -- automated | llm_judge | human
    created_at  TIMESTAMP NOT NULL,
    notes       TEXT
);
CREATE INDEX idx_eval_runs_dataset ON eval_runs(dataset_id);

CREATE TABLE eval_item_results (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id          TEXT NOT NULL REFERENCES eval_runs(id) ON DELETE CASCADE,
    item_id         TEXT NOT NULL,
    response        TEXT NOT NULL,
    gofmt_valid     BOOLEAN,                 -- NULL if not applicable
    mc_chosen       TEXT,
    mc_correct      BOOLEAN,
    rubric_hits     INTEGER NOT NULL,
    rubric_total    INTEGER NOT NULL,
    score           REAL NOT NULL,
    scorer_type     TEXT NOT NULL,
    judge_rationale TEXT,
    scored_at       TIMESTAMP NOT NULL,
    UNIQUE(run_id, item_id)
);
CREATE INDEX idx_eval_item_results_run  ON eval_item_results(run_id);
CREATE INDEX idx_eval_item_results_item ON eval_item_results(item_id);

