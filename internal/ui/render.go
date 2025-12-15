package ui

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
)

func Render(w http.ResponseWriter, r *http.Request, c templ.Component) {
	err := c.Render(r.Context(), w)
	if err != nil {
		slog.Error("render failed", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

func RenderFragment(w http.ResponseWriter, r *http.Request, c templ.Component, fragmentIDs ...any) {
	err := templ.RenderFragments(r.Context(), w, c, fragmentIDs...)
	if err != nil {
		slog.Error("render fragment failed", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

func RenderOOB(w http.ResponseWriter, r *http.Request, c templ.Component, target string) {
	// Write OOB wrapper start
	_, err := fmt.Fprintf(w, `<div hx-swap-oob="%s">`, target)
	if err != nil {
		slog.Error("render oob write wrapper start failed", "error", err)
		return
	}

	// Render component
	err = c.Render(r.Context(), w)
	if err != nil {
		slog.Error("render oob component render failed", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Write OOB wrapper end
	_, err = w.Write([]byte(`</div>`))
	if err != nil {
		slog.Error("render oob write wrapper end failed", "error", err)
	}
}
