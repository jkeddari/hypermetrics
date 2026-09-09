package pages

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/a-h/templ"
	"github.com/jkeddari/hypermetrics/internal/model"
)

func TestMarketingForActiveSubscriptions(t *testing.T) {
	tests := []struct {
		plan, action string
	}{
		{plan: "builder", action: "Coming soon"},
		{plan: "pro", action: "Downgrade"},
		{plan: "enterprise", action: "Downgrade"},
	}
	for _, test := range tests {
		t.Run(test.plan, func(t *testing.T) {
			var body bytes.Buffer
			err := Marketing("https://hypermetrics.xyz", true, &model.Subscription{PlanID: test.plan, Status: "active"}).Render(context.Background(), &body)
			if err != nil {
				t.Fatal(err)
			}
			html := body.String()
			for _, expected := range []string{"Dashboard", "Current plan", test.action} {
				if !strings.Contains(html, expected) {
					t.Fatalf("active %s page is missing %q", test.plan, expected)
				}
			}
			for _, unexpected := range []string{"Start your 7-day trial", "Start your free trial", "Stop building data plumbing"} {
				if strings.Contains(html, unexpected) {
					t.Fatalf("active %s page contains %q", test.plan, unexpected)
				}
			}
		})
	}
}

func TestPublicPageSEO(t *testing.T) {
	tests := []struct {
		name, canonical, description string
		component                    templ.Component
	}{
		{
			name: "home", canonical: "https://hypermetrics.xyz/",
			description: "Real-time Hyperliquid market intelligence for traders, bot builders, and product teams.",
			component:   Marketing("https://hypermetrics.xyz/", false, nil),
		},
		{
			name: "docs", canonical: "https://hypermetrics.xyz/docs",
			description: "Hypermetrics API documentation for Hyperliquid whale alerts, wallet positions, market structure, authentication, and rate limits.",
			component:   Docs("https://hypermetrics.xyz/"),
		},
		{
			name: "privacy", canonical: "https://hypermetrics.xyz/legal/privacy",
			description: "Learn how Hypermetrics collects, uses, protects, and retains account, billing, and API usage data.",
			component:   Privacy("https://hypermetrics.xyz/", false),
		},
		{
			name: "terms", canonical: "https://hypermetrics.xyz/legal/terms",
			description: "Review the terms governing Hypermetrics accounts, API access, trials, subscriptions, and acceptable use.",
			component:   Terms("https://hypermetrics.xyz/", false),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var body bytes.Buffer
			if err := test.component.Render(context.Background(), &body); err != nil {
				t.Fatal(err)
			}
			html := body.String()
			for _, expected := range []string{
				`rel="canonical" href="` + test.canonical + `"`,
				`property="og:url" content="` + test.canonical + `"`,
				`name="twitter:card" content="summary_large_image"`,
				`name="description" content="` + test.description + `"`,
			} {
				if !strings.Contains(html, expected) {
					t.Fatalf("page is missing %q", expected)
				}
			}
			if strings.Contains(html, test.canonical+"?") {
				t.Fatal("canonical URL must not contain query parameters")
			}
		})
	}
}

func TestHomeIncludesStructuredData(t *testing.T) {
	var body bytes.Buffer
	if err := Marketing("https://hypermetrics.xyz", false, nil).Render(context.Background(), &body); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`application/ld+json`, `"@type":"WebSite"`, `"@type":"Organization"`, `hello@hypermetrics.xyz`, `/assets/images/logo-512.png`, `Free`, `Coming soon`} {
		if !strings.Contains(body.String(), expected) {
			t.Fatalf("structured data is missing %q", expected)
		}
	}
}

func TestPrivateLayoutsAreNoIndex(t *testing.T) {
	var login bytes.Buffer
	if err := Login("csrf", "", "").Render(context.Background(), &login); err != nil {
		t.Fatal(err)
	}
	var dashboard bytes.Buffer
	if err := Dashboard(&model.User{Email: "user@example.com"}, nil, nil, "", "csrf", "", "", "", 3).Render(context.Background(), &dashboard); err != nil {
		t.Fatal(err)
	}
	for name, html := range map[string]string{"login": login.String(), "dashboard": dashboard.String()} {
		if !strings.Contains(html, `name="robots" content="noindex, nofollow"`) {
			t.Fatalf("%s layout must be noindex", name)
		}
	}
}

func TestBillingCardExplainsCancelledTrial(t *testing.T) {
	trialEnd := time.Date(2026, time.August, 15, 12, 0, 0, 0, time.UTC)
	var body bytes.Buffer
	err := billingCard(&model.Subscription{
		PlanID: "builder", Status: "trialing", TrialEnd: &trialEnd, CancelAtPeriodEnd: true,
	}, "csrf").Render(context.Background(), &body)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"Subscription cancelled", "15 Aug 2026", "API and your API keys will no longer be available"} {
		if !strings.Contains(body.String(), expected) {
			t.Fatalf("cancelled trial notice is missing %q", expected)
		}
	}
}

func TestDashboardWarnsAboutPastDuePayment(t *testing.T) {
	var body bytes.Buffer
	err := Dashboard(
		&model.User{Email: "user@example.com"},
		&model.Subscription{PlanID: "builder", Status: "past_due"},
		nil, "", "csrf", "", "", "https://api.example.com", 3,
	).Render(context.Background(), &body)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"Payment failed", "API access remains active", "Update payment method"} {
		if !strings.Contains(body.String(), expected) {
			t.Fatalf("past-due dashboard is missing %q", expected)
		}
	}
}
