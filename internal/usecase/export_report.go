package usecase

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"

	"evalarena/internal/domain"
)

//go:embed report_template.html
var reportTemplateSrc string

type ExportReportUseCase struct {
	Repo      ArenaRepository
	WinRateUC *ComputeWinRateUseCase
	EloUC     *ComputeEloRankingUseCase
}

func NewExportReportUseCase(repo ArenaRepository) *ExportReportUseCase {
	return &ExportReportUseCase{
		Repo:      repo,
		WinRateUC: NewComputeWinRateUseCase(repo),
		EloUC:     NewComputeEloRankingUseCase(repo),
	}
}

type reportData struct {
	Arena       *domain.Arena
	WinRate     *WinRateReport
	EloRankings []domain.EloRating
}

// Execute renders a self-contained static HTML report for the given arena,
// suitable for sharing publicly ("we compared X vs Y" posts).
func (uc *ExportReportUseCase) Execute(arenaID string) ([]byte, error) {
	arena, err := uc.Repo.Get(arenaID)
	if err != nil {
		return nil, err
	}
	wr, err := uc.WinRateUC.Execute(arenaID)
	if err != nil {
		return nil, err
	}
	var elo []domain.EloRating
	if arena.Mode == domain.ModeTournament {
		elo, err = uc.EloUC.Execute(arenaID)
		if err != nil {
			return nil, err
		}
	}

	tmpl, err := template.New("report").Funcs(template.FuncMap{
		"pct": func(f float64) string { return fmt.Sprintf("%.1f%%", f*100) },
		"inc": func(i int) int { return i + 1 },
	}).Parse(reportTemplateSrc)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, reportData{Arena: arena, WinRate: wr, EloRankings: elo}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
