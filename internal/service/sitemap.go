package service

import (
	"encoding/xml"
	"log/slog"
	"strings"
	"time"

	"github.com/jkeddari/hypermetrics/internal/model"
)

// publicRoutes defines all static public routes that should be included in the sitemap
// Add new public pages here (but not auth-protected pages like /dashboard)
var publicRoutes = []struct {
	Path       string
	Priority   string
	ChangeFreq string
}{
	{"/", "1.0", "daily"},
	{"/docs", "0.8", "weekly"},
	{"/login", "0.3", "monthly"},
	{"/register", "0.3", "monthly"},
	// Add new public routes here, e.g.:
	// {"/about", "0.7", "monthly"},
	// {"/contact", "0.5", "monthly"},
}

type SitemapService struct {
	docsService *DocsService
	baseURL     string
}

// NewSitemapService creates a new sitemap service
func NewSitemapService(docsService *DocsService, baseURL string) *SitemapService {
	// Ensure baseURL doesn't have trailing slash
	baseURL = strings.TrimSuffix(baseURL, "/")

	return &SitemapService{
		docsService: docsService,
		baseURL:     baseURL,
	}
}

// GenerateSitemap generates a complete sitemap including all pages
func (s *SitemapService) GenerateSitemap() ([]byte, error) {
	sitemap := model.Sitemap{
		XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  []model.SitemapURL{},
	}

	// Add static routes
	staticRoutes := s.getStaticRoutes()
	for _, route := range staticRoutes {
		sitemap.URLs = append(sitemap.URLs, route)
	}

	// Add documentation pages
	docsURLs := s.getDocsURLs()
	sitemap.URLs = append(sitemap.URLs, docsURLs...)

	// Generate XML
	output, err := xml.MarshalIndent(sitemap, "", "  ")
	if err != nil {
		return nil, err
	}

	// Add XML header
	result := xml.Header + string(output)
	return []byte(result), nil
}

// getStaticRoutes returns the static routes of the application
func (s *SitemapService) getStaticRoutes() []model.SitemapURL {
	today := time.Now().Format("2006-01-02")
	urls := make([]model.SitemapURL, 0, len(publicRoutes))

	for _, route := range publicRoutes {
		urls = append(urls, model.SitemapURL{
			Loc:        s.baseURL + route.Path,
			LastMod:    today,
			ChangeFreq: route.ChangeFreq,
			Priority:   route.Priority,
		})
	}

	return urls
}

// getDocsURLs returns all documentation page URLs
func (s *SitemapService) getDocsURLs() []model.SitemapURL {
	// Build docs tree if not already built
	err := s.docsService.BuildDocsTree()
	if err != nil {
		// Log error but don't fail
		slog.Warn("failed to build docs tree for sitemap", "error", err)
		return []model.SitemapURL{}
	}

	pages := s.docsService.FlatDocsList()
	urls := make([]model.SitemapURL, 0, len(pages))

	today := time.Now().Format("2006-01-02")

	for _, page := range pages {
		// Skip pages without content (directory placeholders)
		if page.HTMLContent == "" && len(page.Children) > 0 {
			continue
		}

		// Determine priority based on path depth
		depth := strings.Count(page.Slug, "/")
		priority := "0.6"
		if depth == 0 {
			priority = "0.8" // Top-level docs
		} else if depth == 1 {
			priority = "0.7" // Second-level docs
		}

		urls = append(urls, model.SitemapURL{
			Loc:        s.baseURL + "/docs/" + page.Slug,
			LastMod:    today,
			ChangeFreq: "weekly",
			Priority:   priority,
		})
	}

	return urls
}
