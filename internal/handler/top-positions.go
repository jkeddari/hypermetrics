package handler

import (
	"net/http"

	"github.com/jkeddari/hypermetrics/internal/ui"
	"github.com/jkeddari/hypermetrics/internal/ui/pages"
)

type TopPoisitionsHandler struct{}

func NewTopPositionsHandler() *TopPoisitionsHandler {
	return &TopPoisitionsHandler{}
}

func (h *TopPoisitionsHandler) TopPositionsPage(w http.ResponseWriter, r *http.Request) {
	ui.Render(w, r, pages.ComigSoon("Top positions"))
}
