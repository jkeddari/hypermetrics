package handler

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/jkeddari/hypermetrics/internal/service"
)

type SEOHandler struct {
	sitemapService *service.SitemapService
}

// NewSEOHandler creates a new SEO handler
func NewSEOHandler(docsService *service.DocsService, baseURL string) *SEOHandler {
	return &SEOHandler{
		sitemapService: service.NewSitemapService(docsService, baseURL),
	}
}

// Robots serves the robots.txt file
func (h *SEOHandler) Robots(w http.ResponseWriter, r *http.Request) {
	// Try to serve from static directory
	robotsPath := filepath.Join("static", "robots.txt")
	content, err := os.ReadFile(robotsPath)
	if err != nil {
		// Fallback to a simple default robots.txt
		content = []byte(`User-agent: *
Allow: /
Sitemap: /sitemap.xml`)
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write(content)
}

// Sitemap generates and serves the sitemap.xml dynamically
func (h *SEOHandler) Sitemap(w http.ResponseWriter, r *http.Request) {
	sitemap, err := h.sitemapService.GenerateSitemap()
	if err != nil {
		http.Error(w, "Failed to generate sitemap", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Write(sitemap)
}
