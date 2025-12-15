package handler

import (
	"net/http"

	"github.com/jkeddari/hypermetrics/internal/ui"
	"github.com/jkeddari/hypermetrics/internal/ui/pages"
)

type HomeHandler struct{}

func NewHomeHandler() *HomeHandler {
	return &HomeHandler{}
}

func (h *HomeHandler) HomePage(w http.ResponseWriter, r *http.Request) {
	ui.Render(w, r, pages.Home())
}

func (h *HomeHandler) NotFoundPage(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	ui.Render(w, r, pages.NotFound())
}
