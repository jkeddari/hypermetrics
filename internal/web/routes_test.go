package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jkeddari/hypermetrics/internal/app"
	"github.com/jkeddari/hypermetrics/internal/config"
)

func TestOpenAPISpecUsesConfiguredAPIBaseURL(t *testing.T) {
	s := &server{app: &app.WebApp{Cfg: &config.Config{APIBaseURL: "https://api.test.internal/"}}}
	recorder := httptest.NewRecorder()

	s.openAPISpec(recorder, httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/yaml; charset=utf-8" {
		t.Fatalf("content type = %q", got)
	}
	if got := recorder.Header().Get("X-Robots-Tag"); got != "noindex" {
		t.Fatalf("X-Robots-Tag = %q, want noindex", got)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "url: https://api.test.internal\n") {
		t.Fatal("configured API base URL is missing from the served specification")
	}
	if strings.Contains(body, "url: https://api.hypermetrics.xyz\n") {
		t.Fatal("default API base URL was not replaced")
	}
}

func TestDocsSecurityPolicyAllowsScalarAndAPI(t *testing.T) {
	handler := securityHeaders("https://api.example.com/v1", true)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/docs", nil))

	policy := recorder.Header().Get("Content-Security-Policy")
	for _, expected := range []string{"https://cdn.jsdelivr.net", "connect-src 'self' https://api.example.com"} {
		if !strings.Contains(policy, expected) {
			t.Fatalf("policy %q does not contain %q", policy, expected)
		}
	}
}

func TestSecurityPolicyAllowsStripeBillingRedirects(t *testing.T) {
	handler := securityHeaders("", true)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/app/dashboard", nil))

	policy := recorder.Header().Get("Content-Security-Policy")
	for _, expected := range []string{"https://checkout.stripe.com", "https://billing.stripe.com"} {
		if !strings.Contains(policy, expected) {
			t.Fatalf("policy %q does not contain %q", policy, expected)
		}
	}
}

func TestRobotsByEnvironment(t *testing.T) {
	tests := []struct {
		name, environment, expected string
	}{
		{name: "production", environment: "production", expected: "User-agent: *\nAllow: /\nDisallow: /app/\n\nSitemap: https://hypermetrics.xyz/sitemap.xml\n"},
		{name: "non-production", environment: "development", expected: "User-agent: *\nDisallow: /\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := &server{app: &app.WebApp{Cfg: &config.Config{AppEnv: test.environment, AppURL: "https://hypermetrics.xyz/"}}}
			recorder := httptest.NewRecorder()
			s.robots(recorder, httptest.NewRequest(http.MethodGet, "/robots.txt", nil))
			if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
				t.Fatalf("unexpected robots response: status=%d content-type=%q", recorder.Code, recorder.Header().Get("Content-Type"))
			}
			if recorder.Body.String() != test.expected {
				t.Fatalf("robots body = %q, want %q", recorder.Body.String(), test.expected)
			}
		})
	}
}

func TestSitemapContainsOnlyPublicCanonicalPages(t *testing.T) {
	s := &server{app: &app.WebApp{Cfg: &config.Config{AppURL: "https://hypermetrics.xyz/"}}}
	recorder := httptest.NewRecorder()
	s.sitemap(recorder, httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil))

	if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Type") != "application/xml; charset=utf-8" {
		t.Fatalf("unexpected sitemap response: status=%d content-type=%q", recorder.Code, recorder.Header().Get("Content-Type"))
	}
	body := recorder.Body.String()
	for _, expected := range []string{
		"<loc>https://hypermetrics.xyz/</loc>",
		"<loc>https://hypermetrics.xyz/docs</loc>",
		"<loc>https://hypermetrics.xyz/legal/privacy</loc>",
		"<loc>https://hypermetrics.xyz/legal/terms</loc>",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("sitemap is missing %q", expected)
		}
	}
	if strings.Count(body, "<url>") != 4 {
		t.Fatalf("sitemap contains %d URLs, want 4", strings.Count(body, "<url>"))
	}
	for _, excluded := range []string{"/login", "/register", "/app/", "/magic-link/", "/verify-email/", "/openapi.yaml"} {
		if strings.Contains(body, excluded) {
			t.Fatalf("sitemap must not contain %q", excluded)
		}
	}
}

func TestNonProductionResponsesAreNoIndex(t *testing.T) {
	handler := securityHeaders("", false)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if got := recorder.Header().Get("X-Robots-Tag"); got != "noindex, nofollow" {
		t.Fatalf("X-Robots-Tag = %q, want noindex, nofollow", got)
	}
}

func TestSelectedPlanRejectsUnconfiguredPlans(t *testing.T) {
	if selectedPlan("builder") != "builder" || selectedPlan("pro") != "pro" {
		t.Fatal("known plans must be accepted")
	}
	if selectedPlan("enterprise") != "" {
		t.Fatal("Enterprise must not be available through Checkout")
	}
}
