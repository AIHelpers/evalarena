# EvalArena

A side-by-side comparison tool: paste two (or more) model outputs for the
same prompt set, have human reviewers pick a winner blind, and get back a
win-rate score with confidence intervals — the "did the fine-tune actually
help?" step after a fine-tuning run.

This implements the plan in `08-evalarena.md`, in Go, with **zero external
dependencies** (see "Deviations from the plan" below for why, and what that
traded off).

## What's included

- **Domain model**: `Arena`, `Matchup`, `Vote`, `Reviewer`, plus pure-Go
  Wilson score interval and Elo/Bradley-Terry update math.
- **Use cases**: create arena, blind position-randomized review, cast vote,
  win-rate + category breakdown, Elo tournament ranking, LLM-judge
  auto-pilot pre-pass, reviewer calibration, static HTML report export,
  import from ModelBench-Local / Distillery / PromptVault export formats.
- **JSON-file repository** implementing the same `ArenaRepository` port a
  SQLite or Postgres adapter would (see below).
- **HTTP server** (`cmd/server`) exposing a REST API and serving a static
  browser reviewer UI + results dashboard — no install needed for
  distributed review teams, per the plan's "web server" requirement.
- **CLI** (`cmd/cli`) for headless import/create/results/report, for
  CI-triggered arena creation.
- Unit tests for Wilson interval and Elo math against reference values, a
  win-rate tallying test, a position-randomization bias test (500 simulated
  reviewers, checks the A/B split isn't skewed), and a full
  create→vote→compute-results integration test. All pass under
  `go test -race -cover ./...`.

## Running it

```
go run ./cmd/server                 # starts on :8080, data in ./data
# open http://localhost:8080
```

```
go run ./cmd/cli create --name "base vs finetune-v3" \
  --from-distillery path/to/distillery_job.json
go run ./cmd/cli results --id <arena-id>
go run ./cmd/cli report --id <arena-id> --out report.html
```

Env vars: `EVALARENA_ADDR`, `EVALARENA_DATA`, `EVALARENA_JUDGE_MODEL`,
`ANTHROPIC_API_KEY` (only needed if you want the LLM-judge pre-pass to
actually call a model instead of no-op'ing everything to human review).

### Import file formats

`cmd/cli create --from-modelbench/--from-distillery/--from-promptvault`
expect that tool's own export shape — see
`internal/adapter/importer/*.go` for the exact JSON fields each one reads.

## Architecture

Matches the plan's clean-architecture layout:
`domain` (entities + pure math) → `usecase` (application logic, behind
`ports.go` interfaces) → `adapter` (HTTP handlers, JSON-file repo,
importers, judge client) → `cmd` (server/CLI wiring). Swapping the
JSON-file repo for SQLite/Postgres later means writing one new adapter
package that implements `usecase.ArenaRepository` / `ReviewerRepository` —
no usecase or handler code changes.

## Deviations from the plan

This sandbox's network policy has no route to the Go module proxy (or to
module hosts like `modernc.org`), so anything requiring `go get` — `chi`,
`cobra`, a SQLite driver, Wails — wasn't fetchable. Rather than ship
something that only works with network access this environment doesn't
have, I implemented equivalent behavior with the standard library:

- **Storage**: JSON files on disk instead of SQLite, behind the exact same
  `ArenaRepository` interface the plan specifies. A real `sqlite`/`postgres`
  adapter is a drop-in later; nothing else changes.
- **HTTP routing**: Go 1.22's built-in `net/http.ServeMux` method+pattern
  routing (`"POST /api/arenas/{id}/vote"`) instead of `chi` — same
  capability, no dependency.
- **CLI**: standard `flag` package with a hand-rolled subcommand dispatch
  instead of `cobra` — same `evalarena <verb> --flag` surface.
- **Desktop app**: the plan's Wails reviewer UI isn't buildable in this
  headless sandbox (no windowing system, and `wails` itself isn't
  fetchable here either). I built the "also ships as a lightweight web
  server" half of that requirement instead: a full browser-based reviewer
  UI with keyboard shortcuts (←/→/Space), blind A/B display, and a results
  dashboard, served by `cmd/server`. Porting that UI into an actual Wails
  desktop shell once you have normal network/dependency access should be
  mechanical — the usecases and HTTP API underneath don't change.
- **Tournament-mode blinding**: extended beyond the plan's literal spec
  since pairwise position-swap doesn't generalize to N candidates —
  reviewers see a randomly-ordered, unlabeled list of options and vote by
  position; votes resolve back to the real candidate label server-side.

## Testing & CI

`go build ./...`, `go vet ./...`, and `go test -race -cover ./...` all pass
today. `golangci-lint` and a Wails build matrix from the plan's CI section
weren't run here for the same network-access reason as above.
