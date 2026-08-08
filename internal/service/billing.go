package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jkeddari/hypermetrics/internal/model"
	"github.com/stripe/stripe-go/v85"
	"github.com/stripe/stripe-go/v85/webhook"
)

const trialDays int64 = 7

var (
	ErrBillingNotConfigured    = errors.New("billing is not configured")
	ErrInvalidPlan             = errors.New("invalid billing plan")
	ErrAlreadySubscribed       = errors.New("an active subscription already exists")
	ErrSubscriptionNotFound    = errors.New("subscription not found")
	ErrInvalidWebhookSignature = errors.New("invalid Stripe webhook signature")
)

type BillingConfig struct {
	AppURL                string
	SecretKey             string
	WebhookSecret         string
	BuilderPriceID        string
	ProPriceID            string
	PortalConfigurationID string
	ResendAPIKey          string
	ResendFromEmail       string
}

type BillingService struct {
	db                    *sql.DB
	stripe                *stripe.Client
	appURL                string
	webhookSecret         string
	builderPriceID        string
	proPriceID            string
	portalConfigurationID string
	resendAPIKey          string
	resendFromEmail       string
	resendURL             string
	httpClient            *http.Client
}

func NewBillingService(db *sql.DB, cfg BillingConfig) *BillingService {
	return &BillingService{
		db: db, stripe: stripe.NewClient(cfg.SecretKey), appURL: strings.TrimRight(cfg.AppURL, "/"),
		webhookSecret: cfg.WebhookSecret, builderPriceID: cfg.BuilderPriceID, proPriceID: cfg.ProPriceID,
		portalConfigurationID: cfg.PortalConfigurationID,
		resendAPIKey:          cfg.ResendAPIKey, resendFromEmail: cfg.ResendFromEmail,
		resendURL: "https://api.resend.com/emails", httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *BillingService) GetSubscription(ctx context.Context, userID string) (*model.Subscription, error) {
	var sub model.Subscription
	var subscriptionID sql.NullString
	var trialEnd sql.NullTime
	var trialUsedAt sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		SELECT user_id, stripe_customer_id, stripe_subscription_id, plan_id, status,
		       trial_end, trial_used_at, cancel_at_period_end, updated_at
		FROM subscriptions WHERE user_id = $1
	`, userID).Scan(&sub.UserID, &sub.StripeCustomerID, &subscriptionID, &sub.PlanID, &sub.Status,
		&trialEnd, &trialUsedAt, &sub.CancelAtPeriodEnd, &sub.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrSubscriptionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load subscription: %w", err)
	}
	sub.StripeSubscriptionID = subscriptionID.String
	if trialEnd.Valid {
		sub.TrialEnd = &trialEnd.Time
	}
	if trialUsedAt.Valid {
		sub.TrialUsedAt = &trialUsedAt.Time
	}
	return &sub, nil
}

func (s *BillingService) CreateCheckout(ctx context.Context, user *model.User, planID string) (string, error) {
	priceID, ok := s.priceForPlan(planID)
	if !ok {
		return "", ErrInvalidPlan
	}
	sub, err := s.GetSubscription(ctx, user.ID)
	if err == nil && sub.Entitled() {
		return "", ErrAlreadySubscribed
	}
	if err != nil && !errors.Is(err, ErrSubscriptionNotFound) {
		return "", err
	}

	customerID := ""
	if sub != nil {
		customerID = sub.StripeCustomerID
	}
	if customerID == "" {
		params := &stripe.CustomerCreateParams{Email: stripe.String(user.Email), Metadata: map[string]string{"user_id": user.ID}}
		params.SetIdempotencyKey("hypermetrics-customer-" + user.ID)
		customer, createErr := s.stripe.V1Customers.Create(ctx, params)
		if createErr != nil {
			return "", fmt.Errorf("create Stripe customer: %w", createErr)
		}
		customerID = customer.ID
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO subscriptions (user_id, stripe_customer_id, plan_id, status)
		VALUES ($1, $2, $3, 'pending')
		ON CONFLICT (user_id) DO UPDATE SET
			stripe_customer_id = EXCLUDED.stripe_customer_id,
			plan_id = EXCLUDED.plan_id,
			status = CASE WHEN subscriptions.status IN ('active', 'trialing', 'past_due') THEN subscriptions.status ELSE 'pending' END,
			updated_at = now()
	`, user.ID, customerID, planID)
	if err != nil {
		return "", fmt.Errorf("store Stripe customer: %w", err)
	}

	subscriptionData := &stripe.CheckoutSessionCreateSubscriptionDataParams{
		Metadata: map[string]string{"user_id": user.ID, "plan": planID},
	}
	if sub == nil || sub.TrialUsedAt == nil {
		subscriptionData.TrialPeriodDays = stripe.Int64(trialDays)
		subscriptionData.TrialSettings = &stripe.CheckoutSessionCreateSubscriptionDataTrialSettingsParams{
			EndBehavior: &stripe.CheckoutSessionCreateSubscriptionDataTrialSettingsEndBehaviorParams{
				MissingPaymentMethod: stripe.String("cancel"),
			},
		}
	}
	params := &stripe.CheckoutSessionCreateParams{
		Mode:                    stripe.String("subscription"),
		Customer:                stripe.String(customerID),
		ClientReferenceID:       stripe.String(user.ID),
		SuccessURL:              stripe.String(s.checkoutSuccessURL()),
		CancelURL:               stripe.String(s.appURL + "/app/dashboard?checkout=cancelled"),
		PaymentMethodCollection: stripe.String("always"),
		AutomaticTax:            &stripe.CheckoutSessionCreateAutomaticTaxParams{Enabled: stripe.Bool(true)},
		TaxIDCollection:         &stripe.CheckoutSessionCreateTaxIDCollectionParams{Enabled: stripe.Bool(true)},
		CustomerUpdate:          &stripe.CheckoutSessionCreateCustomerUpdateParams{Address: stripe.String("auto"), Name: stripe.String("auto")},
		LineItems: []*stripe.CheckoutSessionCreateLineItemParams{{
			Price: stripe.String(priceID), Quantity: stripe.Int64(1),
		}},
		Metadata:         map[string]string{"user_id": user.ID, "plan": planID},
		SubscriptionData: subscriptionData,
	}
	session, err := s.stripe.V1CheckoutSessions.Create(ctx, params)
	if err != nil {
		return "", fmt.Errorf("create Checkout session: %w", err)
	}
	return session.URL, nil
}

func (s *BillingService) SyncCheckoutSession(ctx context.Context, userID, sessionID string) error {
	if sessionID == "" {
		return errors.New("missing Checkout session ID")
	}
	session, err := s.stripe.V1CheckoutSessions.Retrieve(ctx, sessionID, nil)
	if err != nil {
		return fmt.Errorf("retrieve Checkout session: %w", err)
	}
	if session.ClientReferenceID != userID || session.Subscription == nil || string(session.Status) != "complete" {
		return errors.New("Checkout session does not belong to the current user or is not complete")
	}
	sub, err := s.stripe.V1Subscriptions.Retrieve(ctx, session.Subscription.ID, nil)
	if err != nil {
		return fmt.Errorf("retrieve Stripe subscription: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, _, err := s.syncSubscription(ctx, tx, sub, time.Now().Unix()); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *BillingService) checkoutSuccessURL() string {
	return s.appURL + "/app/dashboard?checkout=success&session_id={CHECKOUT_SESSION_ID}"
}

func (s *BillingService) CreatePortal(ctx context.Context, userID string) (string, error) {
	sub, err := s.GetSubscription(ctx, userID)
	if err != nil {
		return "", err
	}
	params := &stripe.BillingPortalSessionCreateParams{
		Customer: stripe.String(sub.StripeCustomerID), ReturnURL: stripe.String(s.appURL + "/app/dashboard"),
	}
	if s.portalConfigurationID != "" {
		params.Configuration = stripe.String(s.portalConfigurationID)
	}
	session, err := s.stripe.V1BillingPortalSessions.Create(ctx, params)
	if err != nil {
		return "", fmt.Errorf("create billing portal session: %w", err)
	}
	return session.URL, nil
}

func (s *BillingService) HandleWebhook(ctx context.Context, payload []byte, signature string) error {
	if s.webhookSecret == "" {
		return ErrBillingNotConfigured
	}
	event, err := webhook.ConstructEvent(payload, signature, s.webhookSecret)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidWebhookSignature, err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var inserted string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO stripe_events (id, type) VALUES ($1, $2)
		ON CONFLICT DO NOTHING RETURNING id
	`, event.ID, string(event.Type)).Scan(&inserted)
	if errors.Is(err, sql.ErrNoRows) {
		return tx.Commit()
	}
	if err != nil {
		return fmt.Errorf("record Stripe event: %w", err)
	}

	switch event.Type {
	case stripe.EventTypeCheckoutSessionCompleted:
		var session stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
			return err
		}
		if err := s.syncCheckout(ctx, tx, &session, event.Created); err != nil {
			return err
		}
	case stripe.EventTypeInvoicePaymentFailed:
		var invoice stripe.Invoice
		if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
			return err
		}
		if err := s.sendPaymentFailed(ctx, tx, event.ID, &invoice); err != nil {
			return err
		}
	case stripe.EventTypeCustomerSubscriptionCreated,
		stripe.EventTypeCustomerSubscriptionUpdated,
		stripe.EventTypeCustomerSubscriptionDeleted,
		stripe.EventTypeCustomerSubscriptionPaused,
		stripe.EventTypeCustomerSubscriptionResumed,
		stripe.EventTypeCustomerSubscriptionTrialWillEnd:
		var sub stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			return err
		}
		userID, applied, err := s.syncSubscription(ctx, tx, &sub, event.Created)
		if err != nil {
			return err
		}
		if applied && event.Type == stripe.EventTypeCustomerSubscriptionTrialWillEnd && sub.Status == stripe.SubscriptionStatusTrialing {
			if err := s.sendTrialReminder(ctx, tx, event.ID, userID, &sub); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func (s *BillingService) syncCheckout(ctx context.Context, tx *sql.Tx, session *stripe.CheckoutSession, eventCreated int64) error {
	userID := session.ClientReferenceID
	if userID == "" {
		userID = session.Metadata["user_id"]
	}
	if userID == "" || session.Customer == nil {
		return errors.New("Checkout session is missing customer linkage")
	}
	planID := session.Metadata["plan"]
	if _, ok := s.priceForPlan(planID); !ok {
		return ErrInvalidPlan
	}
	subscriptionID := ""
	if session.Subscription != nil {
		subscriptionID = session.Subscription.ID
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO subscriptions (user_id, stripe_customer_id, stripe_subscription_id, plan_id, status, stripe_event_created)
		VALUES ($1, $2, NULLIF($3, ''), $4, 'pending', $5)
		ON CONFLICT (user_id) DO UPDATE SET
			stripe_customer_id = EXCLUDED.stripe_customer_id,
			stripe_subscription_id = COALESCE(EXCLUDED.stripe_subscription_id, subscriptions.stripe_subscription_id),
			plan_id = EXCLUDED.plan_id,
			stripe_event_created = GREATEST(subscriptions.stripe_event_created, EXCLUDED.stripe_event_created),
			updated_at = now()
		WHERE subscriptions.stripe_event_created <= EXCLUDED.stripe_event_created
	`, userID, session.Customer.ID, subscriptionID, planID, eventCreated)
	return err
}

func (s *BillingService) syncSubscription(ctx context.Context, tx *sql.Tx, sub *stripe.Subscription, eventCreated int64) (string, bool, error) {
	if sub.Customer == nil {
		return "", false, errors.New("Stripe subscription is missing its customer")
	}
	userID := sub.Metadata["user_id"]
	if userID == "" {
		err := tx.QueryRowContext(ctx, `
			SELECT user_id FROM subscriptions
			WHERE stripe_subscription_id = $1 OR stripe_customer_id = $2
		`, sub.ID, sub.Customer.ID).Scan(&userID)
		if err != nil {
			return "", false, fmt.Errorf("find subscription owner: %w", err)
		}
	}
	planID, ok := s.planFromSubscription(sub)
	if !ok {
		planID = sub.Metadata["plan"]
	}
	if _, ok := s.priceForPlan(planID); !ok {
		return "", false, ErrInvalidPlan
	}
	var trialEnd any
	if sub.TrialEnd > 0 {
		trialEnd = time.Unix(sub.TrialEnd, 0).UTC()
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO subscriptions (
			user_id, stripe_customer_id, stripe_subscription_id, plan_id, status,
			trial_end, trial_used_at, cancel_at_period_end, stripe_event_created
		) VALUES ($1, $2, $3, $4, $5, $6, $6, $7, $8)
		ON CONFLICT (user_id) DO UPDATE SET
			stripe_customer_id = EXCLUDED.stripe_customer_id,
			stripe_subscription_id = EXCLUDED.stripe_subscription_id,
			plan_id = EXCLUDED.plan_id,
			status = EXCLUDED.status,
			trial_end = EXCLUDED.trial_end,
			trial_used_at = COALESCE(subscriptions.trial_used_at, EXCLUDED.trial_used_at),
			cancel_at_period_end = EXCLUDED.cancel_at_period_end,
			stripe_event_created = EXCLUDED.stripe_event_created,
			updated_at = now()
		WHERE subscriptions.stripe_event_created <= EXCLUDED.stripe_event_created
	`, userID, sub.Customer.ID, sub.ID, planID, string(sub.Status), trialEnd, cancellationScheduled(sub), eventCreated)
	if err != nil {
		return "", false, fmt.Errorf("sync subscription: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return "", false, err
	}
	if rows == 0 {
		return userID, false, nil
	}
	if _, err := tx.ExecContext(ctx, `UPDATE api_keys SET plan_id = $1 WHERE user_id = $2`, planID, userID); err != nil {
		return "", false, fmt.Errorf("sync API key plan: %w", err)
	}
	return userID, true, nil
}

func (s *BillingService) sendTrialReminder(ctx context.Context, tx *sql.Tx, eventID, userID string, sub *stripe.Subscription) error {
	var email string
	if err := tx.QueryRowContext(ctx, `SELECT email FROM users WHERE id = $1`, userID).Scan(&email); err != nil {
		return fmt.Errorf("load trial reminder recipient: %w", err)
	}
	planID, _ := s.planFromSubscription(sub)
	if planID == "" {
		planID = sub.Metadata["plan"]
	}
	if sub.TrialEnd == 0 {
		return errors.New("Stripe trial reminder is missing its end date")
	}
	trialEnd := time.Unix(sub.TrialEnd, 0).UTC().Format("2 January 2006 at 15:04 UTC")
	textBody := fmt.Sprintf("Your Hypermetrics %s trial ends on %s. Your subscription will then renew automatically. You can manage or cancel it from your Hypermetrics dashboard.", displayPlanName(planID), trialEnd)
	return s.sendBillingEmail(ctx, eventID, email, "Your Hypermetrics trial ends soon", textBody,
		"<p>"+textBody+"</p><p><a href=\""+s.appURL+"/app/dashboard\">Manage your subscription</a></p>")
}

func (s *BillingService) sendPaymentFailed(ctx context.Context, tx *sql.Tx, eventID string, invoice *stripe.Invoice) error {
	if invoice == nil || invoice.Customer == nil {
		return errors.New("Stripe payment failure is missing its customer")
	}
	if invoice.AttemptCount > 1 {
		return nil
	}
	var email string
	err := tx.QueryRowContext(ctx, `
		SELECT u.email
		FROM subscriptions s
		JOIN users u ON u.id = s.user_id
		WHERE s.stripe_customer_id = $1
	`, invoice.Customer.ID).Scan(&email)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load payment failure recipient: %w", err)
	}
	dashboardURL := s.appURL + "/app/dashboard#billing"
	textBody := "We couldn't process your Hypermetrics subscription payment. Stripe may retry the payment automatically. Update your payment method to avoid or resolve an interruption: " + dashboardURL
	return s.sendBillingEmail(ctx, eventID, email, "Action required: Hypermetrics payment failed", textBody,
		"<p>We couldn't process your Hypermetrics subscription payment.</p><p>Stripe may retry the payment automatically. Update your payment method to avoid or resolve an interruption.</p><p><a href=\""+dashboardURL+"\">Update payment method</a></p>")
}

func (s *BillingService) sendBillingEmail(ctx context.Context, eventID, email, subject, textBody, htmlBody string) error {
	if s.resendAPIKey == "" || s.resendFromEmail == "" {
		return ErrBillingNotConfigured
	}
	body, err := json.Marshal(map[string]any{
		"from": s.resendFromEmail, "to": []string{email}, "subject": subject, "text": textBody, "html": htmlBody,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.resendURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.resendAPIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", eventID)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send billing email: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("send billing email: Resend returned %s: %s", resp.Status, strings.TrimSpace(string(message)))
	}
	return nil
}

func (s *BillingService) priceForPlan(planID string) (string, bool) {
	switch planID {
	case "builder":
		return s.builderPriceID, s.builderPriceID != ""
	case "pro":
		return s.proPriceID, s.proPriceID != ""
	default:
		return "", false
	}
}

func (s *BillingService) planFromSubscription(sub *stripe.Subscription) (string, bool) {
	if sub.Items == nil || len(sub.Items.Data) == 0 || sub.Items.Data[0].Price == nil {
		return "", false
	}
	priceID := sub.Items.Data[0].Price.ID
	switch priceID {
	case s.builderPriceID:
		return "builder", true
	case s.proPriceID:
		return "pro", true
	default:
		return "", false
	}
}

func cancellationScheduled(sub *stripe.Subscription) bool {
	return sub != nil && (sub.CancelAtPeriodEnd || sub.CancelAt > 0)
}

func displayPlanName(planID string) string {
	if planID == "pro" {
		return "Pro"
	}
	return "Builder"
}
