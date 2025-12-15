package handler

import (
	"net/http"
	"strings"

	"github.com/jkeddari/hypermetrics/internal/service"
	"github.com/jkeddari/hypermetrics/internal/ui"
	"github.com/jkeddari/hypermetrics/internal/ui/pages"
)

type DocsHandler struct {
	docsService *service.DocsService
}

func NewDocsHandler(docsService *service.DocsService) *DocsHandler {
	handler := &DocsHandler{
		docsService: docsService,
	}

	// Build the docs tree on initialization
	err := handler.docsService.BuildDocsTree()
	if err != nil {
		// Silently continue - docs might be added later
		_ = err
	}

	return handler
}

func (h *DocsHandler) ShowDocs(w http.ResponseWriter, r *http.Request) {
	// Get the path after /docs/
	path := r.URL.Path
	slug := ""

	if path != "/docs" && path != "/docs/" {
		// Remove /docs/ prefix
		slug = strings.TrimPrefix(path, "/docs/")
		slug = strings.TrimSuffix(slug, "/")
	}

	// Get the requested page
	page, err := h.docsService.DocPage(slug)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		ui.Render(w, r, pages.NotFound())
		return
	}

	// Get the full docs tree for navigation
	docsTree, err := h.docsService.DocsTree()
	if err != nil {
		http.Error(w, "Failed to load documentation", http.StatusInternalServerError)
		return
	}

	// Get previous and next pages
	prevPage, nextPage := h.docsService.PrevNextPages(page)

	// Check if this is an HTMX request
	if r.Header.Get("HX-Request") == "true" {
		// Return only the docs-content fragment for HTMX requests
		// This requires rendering the fragment from the full template
		ui.RenderFragment(w, r, pages.Docs(page, docsTree, prevPage, nextPage), "docs-content", "seo-title", "docs-breadcrumb")
	} else {
		// Return full page for regular requests
		ui.Render(w, r, pages.Docs(page, docsTree, prevPage, nextPage))
	}
}

