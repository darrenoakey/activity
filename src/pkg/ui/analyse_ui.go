package ui

import (
	"context"
	"strings"

	"activity/pkg/proc"

	"gioui.org/layout"
)

func (a *App) handleToolbarClicks(gtx layout.Context, procs []proc.Info, samples uint64) {
	if a.toggleBtn.Clicked(gtx) {
		a.mu.Lock()
		a.showHidden = !a.showHidden
		a.mu.Unlock()
	}
	if a.trendsBtn.Clicked(gtx) {
		a.toggleTrends()
	}
	if a.dismissBtn.Clicked(gtx) {
		a.clearAnalysis()
	}
	if a.analyseBtn.Clicked(gtx) {
		a.startAnalyse(procs, samples)
	}
}

func (a *App) toggleTrends() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.showTrends = !a.showTrends
	if a.showTrends {
		a.sortCol = SortAvgRSS
		return
	}
	a.sortCol = SortCPU
}

func (a *App) clearAnalysis() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.analyseErr = ""
	a.analyseLines = nil
}

func (a *App) startAnalyse(procs []proc.Info, samples uint64) {
	a.mu.Lock()
	if a.analysing {
		a.mu.Unlock()
		return
	}
	a.analysing = true
	a.analyseErr = ""
	a.analyseLines = nil
	a.mu.Unlock()

	snapshot := append([]proc.Info(nil), procs...)
	go a.runAnalyse(snapshot, samples)
}

func (a *App) runAnalyse(procs []proc.Info, samples uint64) {
	ctx, cancel := context.WithTimeout(context.Background(), analyseTimeout)
	defer cancel()
	text, err := askAgentd(ctx, procs, samples)
	a.mu.Lock()
	a.analysing = false
	if err != nil {
		a.analyseErr = err.Error()
		a.analyseLines = nil
	} else {
		a.analyseErr = ""
		a.analyseLines = splitLines(text)
	}
	a.mu.Unlock()
	a.win.Invalidate()
}

func splitLines(text string) []string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, strings.TrimRight(line, "\r"))
	}
	return out
}
