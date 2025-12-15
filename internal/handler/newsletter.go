package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/jkeddari/hypermetrics/internal/service"
	"github.com/jkeddari/hypermetrics/internal/ui"
	"github.com/jkeddari/hypermetrics/internal/ui/blocks"
	"github.com/jkeddari/hypermetrics/internal/validation"
)

type newsletterHandler struct {
	emailService *service.EmailService
}

func NewNewsletterHandler(emailService *service.EmailService) *newsletterHandler {
	return &newsletterHandler{
		emailService: emailService,
	}
}

func (h *newsletterHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(strings.ToLower(r.FormValue("email")))

	err := validation.ValidateEmail(email)
	if err != nil {
		ui.Render(w, r, blocks.FooterNewsletterForm("Please provide a valid email address"))
		return
	}

	err = h.emailService.SubscribeNewsletter(email)
	if err != nil {
		// Service layer already logs errors - just handle error case
		// Return success to prevent email enumeration
		slog.Warn("newsletter subscription error", "error", err, "email", email)
	}

	// Always show success (prevents email enumeration)
	ui.Render(w, r, blocks.FooterNewsletterSuccess())
}