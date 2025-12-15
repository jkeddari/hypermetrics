package handler

import (
	"net/http"

	"github.com/jkeddari/hypermetrics/internal/service"
	"github.com/jkeddari/hypermetrics/internal/ui"
	"github.com/jkeddari/hypermetrics/internal/ui/pages"
)

type LegalHandler struct {
	legalService *service.LegalService
}

func NewLegalHandler(legalService *service.LegalService) *LegalHandler {
	handler := &LegalHandler{
		legalService: legalService,
	}

	// Load legal pages on initialization
	err := handler.legalService.LoadPages()
	if err != nil {
		// Silently continue - pages might be added later
		_ = err
	}

	return handler
}

func (h *LegalHandler) ShowPage(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("page")
	
	page, err := h.legalService.Page(slug)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		ui.Render(w, r, pages.NotFound())
		return
	}

	ui.Render(w, r, pages.Legal(page))
}