package web

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/a-h/templ"
	"github.com/jkeddari/hypermetrics/assets"
	"github.com/jkeddari/hypermetrics/docs"
	"github.com/jkeddari/hypermetrics/internal/app"
	"github.com/jkeddari/hypermetrics/internal/middleware"
	"github.com/jkeddari/hypermetrics/internal/model"
	"github.com/jkeddari/hypermetrics/internal/service"
	"github.com/jkeddari/hypermetrics/internal/ui/pages"
)

type userContextKey struct{}

type server struct {
	app *app.WebApp
}

func SetupRoutes(webApp *app.WebApp) http.Handler {
	s := &server{app: webApp}
	mux := http.NewServeMux()
	authRateLimit := middleware.RateLimitAuth()

	staticFiles, _ := fs.Sub(assets.AssetsFS, ".")
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(staticFiles))))

	mux.HandleFunc("GET /{$}", s.home)
	mux.HandleFunc("GET /docs", s.docs)
	mux.HandleFunc("GET /legal/privacy", s.privacy)
	mux.HandleFunc("GET /legal/terms", s.terms)
	mux.HandleFunc("GET /openapi.yaml", s.openAPISpec)
	mux.HandleFunc("GET /auth", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/login", http.StatusPermanentRedirect)
	})
	mux.HandleFunc("GET /login", s.loginPage)
	mux.HandleFunc("POST /login", authRateLimit(s.login))
	mux.HandleFunc("POST /magic-link", authRateLimit(s.sendMagicLink))
	mux.HandleFunc("GET /magic-link/{token}", s.verifyMagicLink)
	mux.HandleFunc("GET /register", s.registerPage)
	mux.HandleFunc("POST /register", authRateLimit(s.register))
	mux.HandleFunc("GET /verify-email/{token}", s.verifyEmail)
	mux.HandleFunc("POST /stripe/webhook", s.stripeWebhook)
	mux.HandleFunc("POST /logout", s.requireAuth(s.logout))
	mux.HandleFunc("GET /app/dashboard", s.requireAuth(s.dashboard))
	mux.HandleFunc("GET /app/settings", s.requireAuth(s.settingsPage))
	mux.HandleFunc("POST /app/settings/profile", s.requireAuth(s.updateProfile))
	mux.HandleFunc("POST /app/settings/password", s.requireAuth(authRateLimit(s.updatePassword)))
	mux.HandleFunc("POST /app/api-keys", s.requireAuth(s.createAPIKey))
	mux.HandleFunc("POST /app/api-keys/{id}/revoke", s.requireAuth(s.revokeAPIKey))
	mux.HandleFunc("POST /app/billing/checkout", s.requireAuth(s.createCheckout))
	mux.HandleFunc("POST /app/billing/portal", s.requireAuth(s.createBillingPortal))
	mux.HandleFunc("GET /{path...}", s.notFound)

	return middleware.Chain(mux, securityHeaders(webApp.Cfg.APIBaseURL), middleware.RequestLogging)
}

func (s *server) home(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	user, err := s.app.AuthService.UserFromRequest(r)
	if err != nil {
		render(w, r, pages.Marketing(false, nil))
		return
	}
	var subscription *model.Subscription
	if s.app.BillingService != nil {
		subscription, err = s.app.BillingService.GetSubscription(r.Context(), user.ID)
		if err != nil && !errors.Is(err, service.ErrSubscriptionNotFound) {
			slog.Error("failed to load homepage subscription", "error", err, "user_id", user.ID)
			http.Error(w, "unable to load homepage", http.StatusInternalServerError)
			return
		}
	}
	render(w, r, pages.Marketing(true, subscription))
}

func (s *server) docs(w http.ResponseWriter, r *http.Request) {
	render(w, r, pages.Docs())
}

func (s *server) privacy(w http.ResponseWriter, r *http.Request) {
	render(w, r, pages.Privacy(s.loggedIn(r)))
}

func (s *server) terms(w http.ResponseWriter, r *http.Request) {
	render(w, r, pages.Terms(s.loggedIn(r)))
}

func (s *server) openAPISpec(w http.ResponseWriter, _ *http.Request) {
	spec, err := fs.ReadFile(docs.OpenAPIFS, "openapi.yaml")
	if err != nil {
		http.Error(w, "API specification unavailable", http.StatusInternalServerError)
		return
	}
	baseURL := strings.TrimRight(s.app.Cfg.APIBaseURL, "/")
	spec = []byte(strings.ReplaceAll(string(spec), "https://api.hypermetrics.dev", baseURL))
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(spec)
}

func (s *server) loginPage(w http.ResponseWriter, r *http.Request) {
	if s.loggedIn(r) {
		http.Redirect(w, r, "/app/dashboard", http.StatusSeeOther)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	render(w, r, pages.Login(s.app.AuthService.CSRFToken(w, r), "", ""))
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := s.app.AuthService.ValidateCSRF(r); err != nil {
		http.Error(w, "invalid form token", http.StatusForbidden)
		return
	}
	email := strings.TrimSpace(r.FormValue("email"))
	user, err := s.app.AuthService.Login(email, r.FormValue("password"))
	if err != nil {
		message := "Invalid email or password."
		if errors.Is(err, service.ErrEmailNotVerified) {
			message = "Verify your email first, or use a magic link to sign in."
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		render(w, r, pages.Login(s.app.AuthService.CSRFToken(w, r), email, message))
		return
	}
	s.app.AuthService.SetSession(w, user.ID)
	http.Redirect(w, r, "/app/dashboard", http.StatusSeeOther)
}

func (s *server) sendMagicLink(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := s.app.AuthService.ValidateCSRF(r); err != nil {
		http.Error(w, "invalid form token", http.StatusForbidden)
		return
	}
	email := strings.TrimSpace(r.FormValue("email"))
	if err := s.app.AuthService.SendMagicLink(r.Context(), email); err != nil {
		if errors.Is(err, service.ErrInvalidEmail) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			render(w, r, pages.Login(s.app.AuthService.CSRFToken(w, r), email, "Enter a valid email address."))
			return
		}
		if !s.app.Cfg.IsProduction() {
			message := "Resend refused the email. Check RESEND_FROM_EMAIL and the server logs."
			slog.Error("magic link request failed", "error", err)
			w.WriteHeader(http.StatusBadGateway)
			render(w, r, pages.Login(s.app.AuthService.CSRFToken(w, r), email, message))
			return
		}
		slog.Error("magic link request failed", "error", err)
	}
	render(w, r, pages.CheckEmail(email, "Check your inbox", "A secure sign-in link is on its way. If you are new, your account is ready and will be activated when you use it."))
}

func (s *server) verifyMagicLink(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	user, err := s.app.AuthService.VerifyMagicLink(r.Context(), r.PathValue("token"))
	if err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		render(w, r, pages.AuthLinkError("This sign-in link is invalid or has expired."))
		return
	}
	s.app.AuthService.SetSession(w, user.ID)
	http.Redirect(w, r, "/app/dashboard", http.StatusSeeOther)
}

func (s *server) registerPage(w http.ResponseWriter, r *http.Request) {
	if s.loggedIn(r) {
		http.Redirect(w, r, "/app/dashboard", http.StatusSeeOther)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	render(w, r, pages.Register(s.app.AuthService.CSRFToken(w, r), "", selectedPlan(r.URL.Query().Get("plan")), ""))
}

func (s *server) register(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := s.app.AuthService.ValidateCSRF(r); err != nil {
		http.Error(w, "invalid form token", http.StatusForbidden)
		return
	}
	email := strings.TrimSpace(r.FormValue("email"))
	planID := selectedPlan(r.FormValue("plan"))
	_, err := s.app.AuthService.Register(r.Context(), email, r.FormValue("password"), planID)
	if err != nil {
		if errors.Is(err, service.ErrEmailDelivery) {
			slog.Error("verification email delivery failed", "error", err)
			w.WriteHeader(http.StatusBadGateway)
			render(w, r, pages.CheckEmail(email, "Your account was created", "We could not deliver the verification email. Use the magic-link option on sign in to request a fresh link."))
			return
		}
		message := "Unable to create the account."
		switch {
		case errors.Is(err, service.ErrEmailAlreadyExists):
			message = "An account already exists for this email."
		case errors.Is(err, service.ErrInvalidEmail):
			message = "Enter a valid email address."
		case errors.Is(err, service.ErrWeakPassword):
			message = "Password must contain between 12 and 72 characters."
		default:
			slog.Error("registration failed", "error", err)
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		render(w, r, pages.Register(s.app.AuthService.CSRFToken(w, r), email, planID, message))
		return
	}
	render(w, r, pages.CheckEmail(email, "Verify your email", "We sent you a verification link. Click it to activate your account and continue."))
}

func (s *server) verifyEmail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	user, planID, err := s.app.AuthService.VerifyEmail(r.Context(), r.PathValue("token"))
	if err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		render(w, r, pages.AuthLinkError("This verification link is invalid or has expired."))
		return
	}
	s.app.AuthService.SetSession(w, user.ID)
	if planID != "" && s.app.BillingService != nil {
		checkoutURL, checkoutErr := s.app.BillingService.CreateCheckout(r.Context(), user, planID)
		if checkoutErr == nil {
			http.Redirect(w, r, checkoutURL, http.StatusSeeOther)
			return
		}
		slog.Error("failed to start Checkout after email verification", "error", checkoutErr, "user_id", user.ID)
	}
	http.Redirect(w, r, "/app/dashboard", http.StatusSeeOther)
}

func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	if err := s.app.AuthService.ValidateCSRF(r); err != nil {
		http.Error(w, "invalid form token", http.StatusForbidden)
		return
	}
	s.app.AuthService.ClearSession(w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *server) dashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	notice := ""
	if r.URL.Query().Get("checkout") == "success" {
		notice = "Checkout completed. Your subscription will be available in a few seconds."
	}
	s.renderDashboard(w, r, "", notice, "")
}

func (s *server) settingsPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	notice := ""
	switch r.URL.Query().Get("updated") {
	case "profile":
		notice = "Profile updated."
	case "password":
		notice = "Password updated."
	}
	s.renderSettings(w, r, currentUser(r), notice, "", "")
}

func (s *server) updateProfile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := s.app.AuthService.ValidateCSRF(r); err != nil {
		http.Error(w, "invalid form token", http.StatusForbidden)
		return
	}
	user := *currentUser(r)
	user.FirstName = strings.TrimSpace(r.FormValue("first_name"))
	user.LastName = strings.TrimSpace(r.FormValue("last_name"))
	if err := s.app.AuthService.UpdateProfile(r.Context(), user.ID, user.FirstName, user.LastName); err != nil {
		if errors.Is(err, service.ErrInvalidProfileName) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			s.renderSettings(w, r, &user, "", "Names must contain at most 80 characters and no control characters.", "")
			return
		}
		slog.Error("failed to update profile", "error", err, "user_id", user.ID)
		http.Error(w, "unable to update profile", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/app/settings?updated=profile", http.StatusSeeOther)
}

func (s *server) updatePassword(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := s.app.AuthService.ValidateCSRF(r); err != nil {
		http.Error(w, "invalid form token", http.StatusForbidden)
		return
	}
	user := currentUser(r)
	newPassword := r.FormValue("new_password")
	if newPassword != r.FormValue("confirm_password") {
		w.WriteHeader(http.StatusUnprocessableEntity)
		s.renderSettings(w, r, user, "", "", "New password and confirmation do not match.")
		return
	}
	if err := s.app.AuthService.UpdatePassword(r.Context(), user.ID, r.FormValue("current_password"), newPassword); err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCurrentPassword):
			w.WriteHeader(http.StatusUnprocessableEntity)
			s.renderSettings(w, r, user, "", "", "Current password is incorrect.")
		case errors.Is(err, service.ErrWeakPassword):
			w.WriteHeader(http.StatusUnprocessableEntity)
			s.renderSettings(w, r, user, "", "", "Password must contain between 12 and 72 characters.")
		default:
			slog.Error("failed to update password", "error", err, "user_id", user.ID)
			http.Error(w, "unable to update password", http.StatusInternalServerError)
		}
		return
	}
	s.app.AuthService.SetSession(w, user.ID)
	http.Redirect(w, r, "/app/settings?updated=password", http.StatusSeeOther)
}

func (s *server) renderSettings(w http.ResponseWriter, r *http.Request, user *model.User, notice, profileError, passwordError string) {
	render(w, r, pages.Settings(user, s.app.AuthService.CSRFToken(w, r), notice, profileError, passwordError))
}

func (s *server) createAPIKey(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if err := s.app.AuthService.ValidateCSRF(r); err != nil {
		http.Error(w, "invalid form token", http.StatusForbidden)
		return
	}
	user := currentUser(r)
	_, secret, createErr := s.app.APIKeyService.CreateAPIKey(user.ID, r.FormValue("name"))
	if createErr != nil {
		message := "Unable to create the API key."
		if errors.Is(createErr, service.ErrAPIKeyLimit) {
			message = "You already have the maximum of three active API keys."
		} else if errors.Is(createErr, service.ErrInvalidAPIKeyName) {
			message = "Key name must contain between 1 and 50 characters."
		} else if errors.Is(createErr, service.ErrSubscriptionRequired) {
			message = "Start an active subscription or trial before creating an API key."
		} else {
			slog.Error("failed to create API key", "error", createErr, "user_id", user.ID)
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		s.renderDashboard(w, r, "", "", message)
		return
	}
	s.renderDashboard(w, r, secret, "", "")
}

func (s *server) createCheckout(w http.ResponseWriter, r *http.Request) {
	if err := s.app.AuthService.ValidateCSRF(r); err != nil {
		http.Error(w, "invalid form token", http.StatusForbidden)
		return
	}
	if s.app.BillingService == nil {
		http.Error(w, "billing is unavailable", http.StatusServiceUnavailable)
		return
	}
	checkoutURL, err := s.app.BillingService.CreateCheckout(r.Context(), currentUser(r), selectedPlan(r.FormValue("plan")))
	if errors.Is(err, service.ErrAlreadySubscribed) {
		s.createBillingPortal(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to create Checkout session", "error", err, "user_id", currentUser(r).ID)
		http.Error(w, "unable to start Checkout", http.StatusBadGateway)
		return
	}
	http.Redirect(w, r, checkoutURL, http.StatusSeeOther)
}

func (s *server) createBillingPortal(w http.ResponseWriter, r *http.Request) {
	if err := s.app.AuthService.ValidateCSRF(r); err != nil {
		http.Error(w, "invalid form token", http.StatusForbidden)
		return
	}
	if s.app.BillingService == nil {
		http.Error(w, "billing is unavailable", http.StatusServiceUnavailable)
		return
	}
	portalURL, err := s.app.BillingService.CreatePortal(r.Context(), currentUser(r).ID)
	if err != nil {
		slog.Error("failed to create billing portal session", "error", err, "user_id", currentUser(r).ID)
		http.Error(w, "unable to open billing portal", http.StatusBadGateway)
		return
	}
	http.Redirect(w, r, portalURL, http.StatusSeeOther)
}

func (s *server) stripeWebhook(w http.ResponseWriter, r *http.Request) {
	if s.app.BillingService == nil {
		http.Error(w, "billing is unavailable", http.StatusServiceUnavailable)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "invalid webhook body", http.StatusBadRequest)
		return
	}
	err = s.app.BillingService.HandleWebhook(r.Context(), payload, r.Header.Get("Stripe-Signature"))
	if errors.Is(err, service.ErrInvalidWebhookSignature) {
		http.Error(w, "invalid webhook signature", http.StatusBadRequest)
		return
	}
	if err != nil {
		slog.Error("Stripe webhook failed", "error", err)
		http.Error(w, "webhook processing failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *server) renderDashboard(w http.ResponseWriter, r *http.Request, secret, notice, errorMessage string) {
	user := currentUser(r)
	keys, err := s.app.APIKeyService.ListAPIKeys(user.ID)
	if err != nil {
		slog.Error("failed to list API keys", "error", err, "user_id", user.ID)
		http.Error(w, "unable to load dashboard", http.StatusInternalServerError)
		return
	}
	var subscription *model.Subscription
	if s.app.BillingService != nil {
		subscription, err = s.app.BillingService.GetSubscription(r.Context(), user.ID)
		if err != nil && !errors.Is(err, service.ErrSubscriptionNotFound) {
			slog.Error("failed to load subscription", "error", err, "user_id", user.ID)
			http.Error(w, "unable to load dashboard", http.StatusInternalServerError)
			return
		}
	}
	render(w, r, pages.Dashboard(user, subscription, keys, secret, s.app.AuthService.CSRFToken(w, r), notice, errorMessage, s.app.Cfg.APIBaseURL, service.MaxAPIKeysPerUser))
}

func selectedPlan(planID string) string {
	if planID == "builder" || planID == "pro" {
		return planID
	}
	return ""
}

func (s *server) revokeAPIKey(w http.ResponseWriter, r *http.Request) {
	if err := s.app.AuthService.ValidateCSRF(r); err != nil {
		http.Error(w, "invalid form token", http.StatusForbidden)
		return
	}
	if err := s.app.APIKeyService.RevokeAPIKey(currentUser(r).ID, r.PathValue("id")); err != nil {
		http.Error(w, "API key not found", http.StatusNotFound)
		return
	}
	http.Redirect(w, r, "/app/dashboard", http.StatusSeeOther)
}

func (s *server) notFound(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	render(w, r, pages.NotFound(s.loggedIn(r)))
}

func (s *server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := s.app.AuthService.UserFromRequest(r)
		if err != nil {
			s.app.AuthService.ClearSession(w)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), userContextKey{}, user)))
	}
}

func (s *server) loggedIn(r *http.Request) bool {
	_, err := s.app.AuthService.UserFromRequest(r)
	return err == nil
}

func currentUser(r *http.Request) *model.User {
	return r.Context().Value(userContextKey{}).(*model.User)
}

func render(w http.ResponseWriter, r *http.Request, component templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := component.Render(r.Context(), w); err != nil {
		slog.Error("failed to render page", "error", err, "path", r.URL.Path)
	}
}

func securityHeaders(apiBaseURL string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			policy := "default-src 'self'; style-src 'self'; img-src 'self' data:; base-uri 'self'; frame-ancestors 'none'; form-action 'self' https://checkout.stripe.com https://billing.stripe.com"
			if r.URL.Path == "/docs" {
				policy = "default-src 'self'; script-src 'self' https://cdn.jsdelivr.net; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self' data:; connect-src 'self' " + origin(apiBaseURL) + "; worker-src blob:; base-uri 'self'; frame-ancestors 'none'; form-action 'self'"
			}
			w.Header().Set("Content-Security-Policy", policy)
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
			next.ServeHTTP(w, r)
		})
	}
}

func origin(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "'self'"
	}
	return parsed.Scheme + "://" + parsed.Host
}
