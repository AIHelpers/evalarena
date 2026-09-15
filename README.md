# EvalArena

A side-by-side comparison tool: paste two (or more) model outputs for the
same prompt set, have human reviewers pick a winner blind, and get back a
win-rate score with confidence intervals — the "did the fine-tune actually
help?" step after a fine-tuning run.

It also evaluates a model's outputs against a golden dataset (question +
reference answer + rubric) for an absolute correctness signal, persisted
and comparable across runs over time.

## What's included

- **Domain model**: `Arena`, `Matchup`, `Vote`, `Reviewer`, `Dataset`,
  `DatasetItem`, `EvalRun`, `ItemResult`, `RunSummary`, plus pure-Go Wilson
  score interval, Elo/Bradley-Terry update math, and rubric-scoring math.
- **Use cases**: create arena, blind position-randomized review, cast vote,
  win-rate + category breakdown, Elo tournament ranking, LLM-judge
  auto-pilot pre-pass, reviewer calibration, static HTML report export,
  import from ModelBench-Local / Distillery / PromptVault export formats;
  plus dataset import, run evaluation, run summary, and run comparison.
- **Repositories**: both a **JSON-file** adapter and a **SQLite** adapter
  (pure-Go `modernc.org/sqlite`, no cgo) implementing the same
  `ArenaRepository` / `ReviewerRepository` / `DatasetRepository` /
  `EvalRunRepository` ports — swap by setting `EVALARENA_DB_BACKEND`.
- **Dataset evaluation**: grade a model's outputs against a golden dataset
  with automated scorers (gofmt-validity, multiple-choice, rubric
  heuristic) and an optional LLM rubric judge — an absolute correctness
  signal alongside the pairwise arena, persisted and comparable across runs.
- **HTTP server** (`cmd/server`) exposing a REST API and serving a static
  browser reviewer UI + results dashboard, plus a dataset-evaluation
  dashboard at `/eval-dashboard.html` — no install needed for distributed
  review teams.
- **CLI** (`cmd/cli`) for headless import/create/results/report, and for
  dataset import + eval run/list/show/compare/trend.
- Unit tests for Wilson interval and Elo math against reference values, a
  win-rate tallying test, a position-randomization bias test (500 simulated
  reviewers, checks the A/B split isn't skewed), a full
  create→vote→compute-results integration test, and a dataset
  import→run→run→compare integration test. All pass under
  `go test -race -cover ./...`.

## Running it — web

```
go run ./cmd/server                 # starts on :8080, data in ./data
# open http://localhost:8080         → arena reviewer + results dashboard
# open http://localhost:8080/eval-dashboard.html  → dataset-eval dashboard
```

## Running it — CLI

```
go run ./cmd/cli create --name "base vs finetune-v3" \
  --from-distillery path/to/distillery_job.json
go run ./cmd/cli results --id <arena-id>
go run ./cmd/cli report --id <arena-id> --out report.html

# Dataset evaluation
go run ./cmd/cli dataset import --file golden_eval_set.jsonl --name "go-golden-v1"
go run ./cmd/cli eval run --dataset <dataset-id> --model-label "finetune-v3" \
  --responses finetune_outputs.jsonl [--rubric-judge]
go run ./cmd/cli eval list [--dataset <dataset-id>]
go run ./cmd/cli eval show --id <run-id> [--category concurrency] [--failed-only]
go run ./cmd/cli eval compare --ids <run1>,<run2>[,...] [--out FILE.json|.csv|.html]
go run ./cmd/cli eval trend --dataset <dataset-id> [--out trend.csv]
```

Env vars: `EVALARENA_ADDR`, `EVALARENA_DATA`, `EVALARENA_JUDGE_MODEL`,
`EVALARENA_DB_BACKEND` (`jsonfile` default, or `sqlite`),
`ANTHROPIC_API_KEY` (only needed if you want the LLM-judge pre-pass or the
LLM rubric judge to actually call a model instead of no-op'ing everything).

### Import file formats

`cmd/cli create --from-modelbench/--from-distillery/--from-promptvault`
expect that tool's own export shape — see
`internal/adapter/importer/*.go` for the exact JSON fields each one reads.

`dataset import` and `eval run` accept the golden-set JSONL (one JSON object
per line matching `DatasetItem`) and responses JSONL (`{"id","response"}`
per line) respectively — the same shapes the Python `evaluate.py` already
uses, so existing model-output scripts feed EvalArena unchanged.

## Architecture

Matches the plan's clean-architecture layout:
`domain` (entities + pure math) → `usecase` (application logic, behind
`ports.go` interfaces) → `adapter` (HTTP handlers, JSON-file/SQLite repos,
importers, scorers, judge clients) → `cmd` (server/CLI wiring). The JSON-file
repo and the SQLite repo implement the same ports, so swapping the backend
(and later adding Postgres) means writing a new adapter package — no usecase
or handler code changes.

## SQLite backend

SQLite is opt-in via `EVALARENA_DB_BACKEND=sqlite` (default remains
`jsonfile` so existing setups aren't broken). It uses `modernc.org/sqlite`
— a pure-Go transpilation of SQLite with no cgo and no system library,
consistent with the project's "minimize what the build environment must
provide" stance. See `docs/adr/0001-sqlite-backend.md` for the driver
decision and known risks (the first run of `go get` / `go mod download`
needs network access to the Go module proxy; `go mod vendor` is recommended
for offline builds).

## Deviations from the plan

This repo was originally built in a sandbox whose network policy had no
route to the Go module proxy, so anything requiring `go get` — `chi`,
`cobra`, a SQLite driver, Wails — wasn't fetchable at the time. Where that
constraint applied, I implemented equivalent behavior with the standard
library; the SQLite backend and dataset-evaluation feature were added later
once a network-capable environment was available:

- **HTTP routing**: Go 1.22's built-in `net/http.ServeMux` method+pattern
  routing (`"POST /api/arenas/{id}/vote"`) instead of `chi` — same
  capability, no dependency.
- **CLI**: standard `flag` package with a hand-rolled subcommand dispatch
  instead of `cobra` — same `evalarena <verb> --flag` surface.
- **Desktop app**: the plan's Wails reviewer UI wasn't buildable in a
  headless sandbox (no windowing system). I built the "also ships as a
  lightweight web server" half of that requirement instead: a full
  browser-based reviewer UI with keyboard shortcuts (←/→/Space), blind A/B
  display, a results dashboard, and a dataset-eval dashboard, all served by
  `cmd/server`. Porting that UI into an actual Wails desktop shell once
  network/dependency access is available should be mechanical — the usecases
  and HTTP API underneath don't change.
- **Tournament-mode blinding**: extended beyond the plan's literal spec
  since pairwise position-swap doesn't generalize to N candidates —
  reviewers see a randomly-ordered, unlabeled list of options and vote by
  position; votes resolve back to the real candidate label server-side.
- **Storage / dataset evaluation**: rather than only JSON files as a stopgap,
  this repo now ships a real SQLite backend (`EVALARENA_DB_BACKEND=sqlite`)
  and the dataset-evaluation capability (import golden dataset → run eval →
  compare runs over time) — the plan's intended SQLite adapter and
  dataset-eval feature, implemented directly.

## Testing & CI

`go build ./...`, `go vet ./...`, and `go test -race -cover ./...` all pass.