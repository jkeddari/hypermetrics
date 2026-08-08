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
	body := recorder.Body.String()
	if !strings.Contains(body, "url: https://api.test.internal\n") {
		t.Fatal("configured API base URL is missing from the served specification")
	}
	if strings.Contains(body, "url: https://api.hypermetrics.dev\n") {
		t.Fatal("default API base URL was not replaced")
	}
}

func TestDocsSecurityPolicyAllowsScalarAndAPI(t *testing.T) {
	handler := securityHeaders("https://api.hypermetrics.dev/v1")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/docs", nil))

	policy := recorder.Header().Get("Content-Security-Policy")
	for _, expected := range []string{"https://cdn.jsdelivr.net", "connect-src 'self' https://api.hypermetrics.dev"} {
		if !strings.Contains(policy, expected) {
			t.Fatalf("policy %q does not contain %q", policy, expected)
		}
	}
}

func TestSecurityPolicyAllowsStripeBillingRedirects(t *testing.T) {
	handler := securityHeaders("")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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

func TestSelectedPlanRejectsUnconfiguredPlans(t *testing.T) {
	if selectedPlan("builder") != "builder" || selectedPlan("pro") != "pro" {
		t.Fatal("known plans must be accepted")
	}
	if selectedPlan("enterprise") != "" {
		t.Fatal("Enterprise must not be available through Checkout")
	}
}
