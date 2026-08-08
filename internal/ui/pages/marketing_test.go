package pages

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jkeddari/hypermetrics/internal/model"
)

func TestMarketingForActiveSubscriptions(t *testing.T) {
	tests := []struct {
		plan, action string
	}{
		{plan: "builder", action: "Upgrade"},
		{plan: "pro", action: "Downgrade"},
		{plan: "enterprise", action: "Current plan"},
	}
	for _, test := range tests {
		t.Run(test.plan, func(t *testing.T) {
			var body bytes.Buffer
			err := Marketing(true, &model.Subscription{PlanID: test.plan, Status: "active"}).Render(context.Background(), &body)
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
