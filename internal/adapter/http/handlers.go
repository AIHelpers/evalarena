// Package http wires the usecase layer to a REST API using the standard
// library's net/http (Go 1.22+ pattern-based ServeMux), avoiding a router
// dependency so the whole app builds with zero external modules.
package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"evalarena/internal/domain"
	"evalarena/internal/usecase"
)

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
}

func (s *Server) Routes() *http.ServeMux {
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

	// Static frontend.
	mux.Handle("/", http.FileServer(http.Dir("frontend")))

	return mux
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

func (s *Server) handleListArenas(w http.ResponseWriter, r *http.Request) {
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

type errStringT string

func (e errStringT) Error() string { return string(e) }
func errString(s string) error     { return errStringT(s) }

func parseFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}
