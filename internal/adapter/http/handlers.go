// Package http wires the usecase layer to a REST API using the standard
// library's net/http (Go 1.22+ pattern-based ServeMux), avoiding a router
// dependency so the whole app builds with zero external modules.
package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"evalarena/internal/domain"
	"evalarena/internal/usecase"
)

// Server exposes the arena review/evaluation API over HTTP.
type Server struct {
	CreateArena  *usecase.CreateArenaUseCase
	CastVote     *usecase.CastVoteUseCase
	GetForReview *usecase.GetMatchupForReviewUseCase
	WinRate      *usecase.ComputeWinRateUseCase
	Elo          *usecase.ComputeEloRankingUseCase
	JudgePrepass *usecase.RunJudgePrepassUseCase
	Calibrate    *usecase.CalibrateReviewersUseCase
	ExportReport *usecase.ExportReportUseCase
	Repo         usecase.ArenaRepository

	// Dataset evaluation (Phase 6 dashboard).
	Datasets    usecase.DatasetRepository
	EvalRuns    usecase.EvalRunRepository
	EvalSummary *usecase.ComputeEvalSummaryUseCase
	EvalCompare *usecase.CompareEvalRunsUseCase
	RunEval     *usecase.RunEvaluationUseCase
}

// Routes returns the configured HTTP routes, wrapped in permissive CORS
// middleware so the frontend can be served from a different origin (e.g.
// file:// or a separate dev server) without the browser blocking fetch
// calls with "Failed to fetch".
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/arenas", s.handleCreateArena)
	mux.HandleFunc("GET /api/arenas", s.handleListArenas)
	mux.HandleFunc("GET /api/arenas/{id}", s.handleGetArena)
	mux.HandleFunc("DELETE /api/arenas/{id}", s.handleDeleteArena)

	mux.HandleFunc("GET /api/arenas/{id}/matchups/{mid}/review", s.handleGetForReview)
	mux.HandleFunc("POST /api/arenas/{id}/vote", s.handleCastVote)

	mux.HandleFunc("GET /api/arenas/{id}/results", s.handleResults)
	mux.HandleFunc("GET /api/arenas/{id}/elo", s.handleElo)
	mux.HandleFunc("GET /api/arenas/{id}/calibration", s.handleCalibration)
	mux.HandleFunc("POST /api/arenas/{id}/judge-prepass", s.handleJudgePrepass)
	mux.HandleFunc("GET /api/arenas/{id}/report", s.handleReport)

	// Dataset evaluation dashboard (Phase 6).
	mux.HandleFunc("GET /api/datasets", s.handleListDatasets)
	mux.HandleFunc("GET /api/datasets/{id}", s.handleGetDataset)
	mux.HandleFunc("POST /api/eval-runs", s.handleCreateEvalRun)
	mux.HandleFunc("GET /api/eval-runs", s.handleListEvalRuns)
	mux.HandleFunc("GET /api/eval-runs/{id}", s.handleGetEvalRun)
	mux.HandleFunc("GET /api/eval-runs/compare", s.handleCompareEvalRuns)

	// Static frontend.
	mux.Handle("/", http.FileServer(http.Dir("frontend")))

	return corsMiddleware(mux)
}

// corsMiddleware adds permissive CORS headers to every response and answers
// preflight OPTIONS requests so the browser allows cross-origin API calls.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

// --- Arenas ---

type createArenaRequest struct {
	Name     string           `json:"name"`
	Mode     domain.Mode      `json:"mode"`
	Matchups []matchupRequest `json:"matchups"`
}

type matchupRequest struct {
	Prompt     string                   `json:"prompt"`
	Category   *string                  `json:"category,omitempty"`
	CandidateA domain.CandidateOutput   `json:"candidate_a"`
	CandidateB domain.CandidateOutput   `json:"candidate_b"`
	Candidates []domain.CandidateOutput `json:"candidates,omitempty"`
}

func (s *Server) handleCreateArena(w http.ResponseWriter, r *http.Request) {
	var req createArenaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	in := usecase.CreateArenaInput{Name: req.Name, Mode: req.Mode}
	for _, m := range req.Matchups {
		in.Matchups = append(in.Matchups, usecase.MatchupInput{
			Prompt: m.Prompt, Category: m.Category,
			CandidateA: m.CandidateA, CandidateB: m.CandidateB, Candidates: m.Candidates,
		})
	}
	arena, err := s.CreateArena.Execute(in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, arena)
}

func (s *Server) handleListArenas(w http.ResponseWriter, _ *http.Request) {
	arenas, err := s.Repo.List()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, arenas)
}

func (s *Server) handleGetArena(w http.ResponseWriter, r *http.Request) {
	arena, err := s.Repo.Get(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	// Strip source labels so the raw arena endpoint doesn't leak blinding
	// info to a reviewer client; results/report endpoints show labels.
	sanitized := *arena
	sanitized.Matchups = append([]domain.Matchup{}, arena.Matchups...)
	for i := range sanitized.Matchups {
		sanitized.Matchups[i].CandidateA.SourceLabel = ""
		sanitized.Matchups[i].CandidateB.SourceLabel = ""
	}
	writeJSON(w, http.StatusOK, sanitized)
}

func (s *Server) handleDeleteArena(w http.ResponseWriter, r *http.Request) {
	if err := s.Repo.Delete(r.PathValue("id")); err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Review flow ---

func (s *Server) handleGetForReview(w http.ResponseWriter, r *http.Request) {
	reviewerID := r.URL.Query().Get("reviewer")
	if reviewerID == "" {
		writeErr(w, http.StatusBadRequest, errString("reviewer query param is required"))
		return
	}
	bm, err := s.GetForReview.Execute(r.PathValue("id"), r.PathValue("mid"), reviewerID)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, bm)
}

type castVoteRequest struct {
	MatchupID  string        `json:"matchup_id"`
	ReviewerID string        `json:"reviewer_id"`
	Pick       domain.Winner `json:"pick"`
	Note       *string       `json:"note,omitempty"`
}

func (s *Server) handleCastVote(w http.ResponseWriter, r *http.Request) {
	var req castVoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	arena, err := s.CastVote.Execute(usecase.CastVoteInput{
		ArenaID: r.PathValue("id"), MatchupID: req.MatchupID,
		ReviewerID: req.ReviewerID, Pick: req.Pick, Note: req.Note,
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, arena)
}

// --- Results ---

func (s *Server) handleResults(w http.ResponseWriter, r *http.Request) {
	report, err := s.WinRate.Execute(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) handleElo(w http.ResponseWriter, r *http.Request) {
	ratings, err := s.Elo.Execute(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, ratings)
}

func (s *Server) handleCalibration(w http.ResponseWriter, r *http.Request) {
	reviewers, err := s.Calibrate.Execute(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, reviewers)
}

func (s *Server) handleJudgePrepass(w http.ResponseWriter, r *http.Request) {
	threshold := 0.75
	if v := r.URL.Query().Get("threshold"); v != "" {
		if parsed, err := parseFloat(v); err == nil {
			threshold = parsed
		}
	}
	result, err := s.JudgePrepass.Execute(r.PathValue("id"), threshold)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	html, err := s.ExportReport.Execute(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(html)
}

// --- Dataset evaluation (Phase 6 dashboard) ---

// handleListDatasets returns all imported golden evaluation datasets
// (without their full items, to keep payload light for the run form).
func (s *Server) handleListDatasets(w http.ResponseWriter, _ *http.Request) {
	datasets, err := s.Datasets.List()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	type dsView struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Version   string `json:"version"`
		Imported  string `json:"imported_at"`
		ItemCount int    `json:"item_count"`
	}
	out := make([]dsView, 0, len(datasets))
	for _, d := range datasets {
		out = append(out, dsView{
			ID:        d.ID,
			Name:      d.Name,
			Version:   d.Version,
			Imported:  d.ImportedAt.Format("2006-01-02 15:04"),
			ItemCount: len(d.Items),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// handleGetDataset returns a single dataset's full contents (including items)
// so the run form can render per-item response fields.
func (s *Server) handleGetDataset(w http.ResponseWriter, r *http.Request) {
	ds, err := s.Datasets.Get(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, ds)
}

// createEvalRunRequest is the POST /api/eval-runs payload. Responses maps
// dataset item IDs to the model's generated output for that item.
type createEvalRunRequest struct {
	DatasetID      string            `json:"dataset_id"`
	ModelLabel     string            `json:"model_label"`
	Responses      map[string]string `json:"responses"`
	UseRubricJudge bool              `json:"use_rubric_judge"`
	Notes          string            `json:"notes,omitempty"`
}

// handleCreateEvalRun runs a new evaluation against a dataset with the
// provided per-item responses, then returns the created run with summary.
func (s *Server) handleCreateEvalRun(w http.ResponseWriter, r *http.Request) {
	if s.RunEval == nil {
		writeErr(w, http.StatusServiceUnavailable, errString("eval run endpoint not wired"))
		return
	}
	var req createEvalRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if req.DatasetID == "" || req.ModelLabel == "" {
		writeErr(w, http.StatusBadRequest, errString("dataset_id and model_label are required"))
		return
	}
	if len(req.Responses) == 0 {
		writeErr(w, http.StatusBadRequest, errString("responses must contain at least one item"))
		return
	}
	run, err := s.RunEval.Execute(usecase.RunEvaluationInput{
		DatasetID:      req.DatasetID,
		ModelLabel:     req.ModelLabel,
		Responses:      req.Responses,
		UseRubricJudge: req.UseRubricJudge,
		Notes:          req.Notes,
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	var summary *domain.RunSummary
	if s.EvalSummary != nil {
		if sm, serr := s.EvalSummary.Execute(run.ID); serr == nil {
			summary = sm
		}
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"run":     run,
		"summary": summary,
	})
}

// handleListEvalRuns returns all evaluation runs and the overall score for
// each. Optional ?dataset= filter limits to one dataset.
func (s *Server) handleListEvalRuns(w http.ResponseWriter, r *http.Request) {
	var runs []*domain.EvalRun
	var err error
	if ds := r.URL.Query().Get("dataset"); ds != "" {
		runs, err = s.EvalRuns.ListByDataset(ds)
	} else {
		runs, err = s.EvalRuns.ListAll()
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	type runView struct {
		ID         string   `json:"id"`
		DatasetID  string   `json:"dataset_id"`
		ModelLabel string   `json:"model_label"`
		ScorerMode string   `json:"scorer_mode"`
		CreatedAt  string   `json:"created_at"`
		Notes      string   `json:"notes,omitempty"`
		Overall    *float64 `json:"overall,omitempty"`
		N          int      `json:"n"`
	}
	out := make([]runView, 0, len(runs))
	for _, run := range runs {
		var overall *float64
		n := len(run.Results)
		if s.EvalSummary != nil {
			sum, serr := s.EvalSummary.Execute(run.ID)
			if serr == nil {
				v := sum.Overall
				overall = &v
				n = sum.N
			}
		}
		out = append(out, runView{
			ID: run.ID, DatasetID: run.DatasetID, ModelLabel: run.ModelLabel,
			ScorerMode: string(run.ScorerMode), CreatedAt: run.CreatedAt.Format("2006-01-02 15:04"),
			Notes: run.Notes, Overall: overall, N: n,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// handleGetEvalRun returns a single run with its per-item results, plus the
// run summary (overall/by_type/by_category/by_difficulty).
func (s *Server) handleGetEvalRun(w http.ResponseWriter, r *http.Request) {
	run, err := s.EvalRuns.Get(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	var summary *domain.RunSummary
	if s.EvalSummary != nil {
		if sm, serr := s.EvalSummary.Execute(run.ID); serr == nil {
			summary = sm
		}
	}
	type runDetail struct {
		*domain.EvalRun
		Summary *domain.RunSummary `json:"summary,omitempty"`
	}
	writeJSON(w, http.StatusOK, runDetail{run, summary})
}

// handleCompareEvalRuns compares 2+ runs. Required ?ids=run1,run2[,runN].
func (s *Server) handleCompareEvalRuns(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("ids")
	if raw == "" {
		writeErr(w, http.StatusBadRequest, errString("ids query param is required (comma-separated run ids)"))
		return
	}
	parts := strings.Split(raw, ",")
	ids := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			ids = append(ids, p)
		}
	}
	if len(ids) < 2 {
		writeErr(w, http.StatusBadRequest, errString("compare needs at least 2 run ids"))
		return
	}
	cmp, err := s.EvalCompare.Execute(ids)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, cmp)
}

type errStringT string

func (e errStringT) Error() string { return string(e) }
func errString(s string) error     { return errStringT(s) }

func parseFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}
