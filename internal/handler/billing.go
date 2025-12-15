package handler

import (
	"io"
	"log/slog"
	"net/http"

	"github.com/jkeddari/hypermetrics/internal/ctxkeys"
	"github.com/jkeddari/hypermetrics/internal/service"
	"github.com/jkeddari/hypermetrics/internal/service/payment"
	"github.com/jkeddari/hypermetrics/internal/ui"
	"github.com/jkeddari/hypermetrics/internal/ui/pages"
)

type BillingHandler struct {
	subscriptionService *service.SubscriptionService
	paymentService      payment.Provider
}

func NewBillingHandler(subscriptionService *service.SubscriptionService, paymentService payment.Provider) *BillingHandler {
	return &BillingHandler{
		subscriptionService: subscriptionService,
		paymentService:      paymentService,
	}
}

func (h *BillingHandler) BillingPage(w http.ResponseWriter, r *http.Request) {
	ui.Render(w, r, pages.Billing())
}

func (h *BillingHandler) CreateCheckout(w http.ResponseWriter, r *http.Request) {
	user := ctxkeys.User(r.Context())

	profile := ctxkeys.Profile(r.Context())
	if profile == nil {
		http.Error(w, "Profile not found", http.StatusInternalServerError)
		return
	}

	planID := r.FormValue("plan_id")
	if planID == "" {
		http.Error(w, "Invalid plan selected", http.StatusBadRequest)
		return
	}

	interval := r.FormValue("interval")
	if interval == "" {
		interval = "monthly"
	}

	checkoutURL, err := h.paymentService.CreateCheckoutURL(user.ID, planID, interval, user.Email, profile.Name)
	if err != nil {
		slog.Error("failed to create checkout", "error", err, "user_id", user.ID, "plan_id", planID, "provider", h.paymentService.Name())
		http.Error(w, "Failed to create checkout session", http.StatusInternalServerError)
		return
	}

	slog.Info("redirecting to checkout", "user_id", user.ID, "provider", h.paymentService.Name(), "checkout_url", checkoutURL)
	http.Redirect(w, r, checkoutURL, http.StatusSeeOther)
}

func (h *BillingHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error("failed to read webhook payload", "error", err)
		http.Error(w, "Failed to read payload", http.StatusBadRequest)
		return
	}
	defer func() {
		closeErr := r.Body.Close()
		if closeErr != nil {
			slog.Error("failed to close request body", "error", closeErr)
		}
	}()

	err = h.paymentService.HandleWebhook(payload, r.Header)
	if err != nil {
		slog.Error("failed to handle webhook", "error", err, "provider", h.paymentService.Name())
		http.Error(w, "Failed to process webhook", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"received": true}`))
}

func (h *BillingHandler) CustomerPortal(w http.ResponseWriter, r *http.Request) {
	user := ctxkeys.User(r.Context())

	portalURL, err := h.paymentService.CustomerPortalURL(user.ID)
	if err != nil {
		slog.Error("failed to get customer portal", "error", err, "user_id", user.ID, "provider", h.paymentService.Name())
		http.Error(w, "Failed to access customer portal", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, portalURL, http.StatusSeeOther)
}
