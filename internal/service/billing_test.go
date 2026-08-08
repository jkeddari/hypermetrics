package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jkeddari/hypermetrics/internal/db"
	"github.com/jkeddari/hypermetrics/internal/model"
	"github.com/stripe/stripe-go/v85"
	stripewebhook "github.com/stripe/stripe-go/v85/webhook"
)

func TestBillingPlanMappingAndSingleTrial(t *testing.T) {
	service := NewBillingService(nil, BillingConfig{
		SecretKey: "sk_test_example", BuilderPriceID: "price_builder", ProPriceID: "price_pro",
	})
	sub := &stripe.Subscription{Items: &stripe.SubscriptionItemList{Data: []*stripe.SubscriptionItem{{Price: &stripe.Price{ID: "price_pro"}}}}}
	planID, ok := service.planFromSubscription(sub)
	if !ok || planID != "pro" {
		t.Fatalf("plan = %q, ok = %v", planID, ok)
	}

	if !((*model.Subscription)(nil)).TrialEligible() {
		t.Fatal("new customer should be eligible for a trial")
	}
	trialEnd := time.Now()
	usedTrial := &model.Subscription{TrialUsedAt: &trialEnd}
	if usedTrial.TrialEligible() {
		t.Fatal("customer must not receive a second trial")
	}
	if !(&model.Subscription{Status: "past_due"}).Entitled() {
		t.Fatal("past-due subscriptions should keep access during Stripe retries")
	}
	if !cancellationScheduled(&stripe.Subscription{CancelAt: time.Now().Add(7 * 24 * time.Hour).Unix()}) {
		t.Fatal("a Stripe cancel_at date must mark the subscription as scheduled for cancellation")
	}
}

func TestBillingEmailUsesResendIdempotency(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer re_test" || r.Header.Get("Idempotency-Key") != "evt_payment_failed" {
			t.Error("Resend authentication or idempotency header is missing")
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "Update payment method") {
			t.Error("payment recovery link is missing from the email")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	billing := NewBillingService(nil, BillingConfig{SecretKey: "sk_test", ResendAPIKey: "re_test", ResendFromEmail: "Hypermetrics <billing@example.com>"})
	billing.resendURL = server.URL
	billing.httpClient = server.Client()
	if err := billing.sendBillingEmail(context.Background(), "evt_payment_failed", "user@example.com", "Payment failed", "Update payment method", "<p>Update payment method</p>"); err != nil {
		t.Fatal(err)
	}
}

func TestBillingRejectsInvalidWebhookSignature(t *testing.T) {
	service := NewBillingService(nil, BillingConfig{SecretKey: "sk_test_example", WebhookSecret: "whsec_example"})
	err := service.HandleWebhook(context.Background(), []byte(`{"id":"evt_test"}`), "invalid")
	if !errors.Is(err, ErrInvalidWebhookSignature) {
		t.Fatalf("error = %v, want invalid signature", err)
	}
}

func TestWebhookControlsSubscriptionAndAPIKeyAccess(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}
	database, err := db.InitPostgres(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.RunMigrations(database); err != nil {
		t.Fatal(err)
	}

	userID := "usr_stripe_billing_test"
	cleanup := func() {
		_, _ = database.Exec(`DELETE FROM stripe_events WHERE id IN ('evt_subscription_created_test', 'evt_subscription_past_due_test', 'evt_subscription_deleted_test')`)
		_, _ = database.Exec(`DELETE FROM api_keys WHERE user_id = $1`, userID)
		_, _ = database.Exec(`DELETE FROM users WHERE id = $1`, userID)
	}
	cleanup()
	defer cleanup()
	if _, err := database.Exec(`INSERT INTO users (id, email, password_hash) VALUES ($1, $2, 'unused')`, userID, "stripe-billing-test@example.com"); err != nil {
		t.Fatal(err)
	}

	billing := NewBillingService(database, BillingConfig{
		SecretKey: "sk_test_example", WebhookSecret: "whsec_test",
		BuilderPriceID: "price_builder", ProPriceID: "price_pro",
	})
	trialEnd := time.Now().Add(7 * 24 * time.Hour).Unix()
	created := fmt.Sprintf(`{"id":"evt_subscription_created_test","object":"event","api_version":"%s","type":"customer.subscription.created","created":%d,"livemode":false,"data":{"object":{"id":"sub_billing_test","object":"subscription","customer":"cus_billing_test","status":"trialing","trial_end":%d,"cancel_at_period_end":false,"metadata":{"user_id":"%s","plan":"builder"},"items":{"object":"list","data":[{"id":"si_billing_test","price":{"id":"price_builder","object":"price"}}]}}}}`, stripe.APIVersion, time.Now().Unix(), trialEnd, userID)
	if err := billing.HandleWebhook(context.Background(), []byte(created), signedHeader([]byte(created), "whsec_test")); err != nil {
		t.Fatal(err)
	}

	sub, err := billing.GetSubscription(context.Background(), userID)
	if err != nil || !sub.Entitled() || sub.PlanID != "builder" || sub.TrialUsedAt == nil {
		t.Fatalf("unexpected subscription: sub=%+v err=%v", sub, err)
	}
	keys := NewAPIKeyService(database)
	_, secret, err := keys.CreateAPIKey(userID, "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := keys.ValidateAPIKey(secret); err != nil {
		t.Fatalf("trialing API key rejected: %v", err)
	}
	pastDue := fmt.Sprintf(`{"id":"evt_subscription_past_due_test","object":"event","api_version":"%s","type":"customer.subscription.updated","created":%d,"livemode":false,"data":{"object":{"id":"sub_billing_test","object":"subscription","customer":"cus_billing_test","status":"past_due","trial_end":%d,"cancel_at_period_end":false,"metadata":{"user_id":"%s","plan":"builder"},"items":{"object":"list","data":[{"id":"si_billing_test","price":{"id":"price_builder","object":"price"}}]}}}}`, stripe.APIVersion, time.Now().Add(time.Second).Unix(), trialEnd, userID)
	if err := billing.HandleWebhook(context.Background(), []byte(pastDue), signedHeader([]byte(pastDue), "whsec_test")); err != nil {
		t.Fatal(err)
	}
	if _, err := keys.ValidateAPIKey(secret); err != nil {
		t.Fatalf("past-due API key rejected during Stripe retries: %v", err)
	}

	deleted := fmt.Sprintf(`{"id":"evt_subscription_deleted_test","object":"event","api_version":"%s","type":"customer.subscription.deleted","created":%d,"livemode":false,"data":{"object":{"id":"sub_billing_test","object":"subscription","customer":"cus_billing_test","status":"canceled","trial_end":%d,"cancel_at_period_end":false,"metadata":{"user_id":"%s","plan":"builder"},"items":{"object":"list","data":[{"id":"si_billing_test","price":{"id":"price_builder","object":"price"}}]}}}}`, stripe.APIVersion, time.Now().Add(2*time.Second).Unix(), trialEnd, userID)
	if err := billing.HandleWebhook(context.Background(), []byte(deleted), signedHeader([]byte(deleted), "whsec_test")); err != nil {
		t.Fatal(err)
	}
	if _, err := keys.ValidateAPIKey(secret); !errors.Is(err, ErrInvalidAPIKey) {
		t.Fatalf("canceled subscription API key error = %v, want invalid", err)
	}
}

func signedHeader(payload []byte, secret string) string {
	signed := stripewebhook.GenerateTestSignedPayload(&stripewebhook.UnsignedPayload{Payload: payload, Secret: secret})
	return signed.Header
}
