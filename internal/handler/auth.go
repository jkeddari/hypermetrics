package handler

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jkeddari/hypermetrics/internal/config"
	"github.com/jkeddari/hypermetrics/internal/ctxkeys"
	"github.com/jkeddari/hypermetrics/internal/service"
	"github.com/jkeddari/hypermetrics/internal/ui"
	"github.com/jkeddari/hypermetrics/internal/ui/components/toast"
	"github.com/jkeddari/hypermetrics/internal/ui/pages"
	"github.com/jkeddari/hypermetrics/internal/validation"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

type authHandler struct {
	authService         *service.AuthService
	userService         *service.UserService
	subscriptionService *service.SubscriptionService
	googleOAuthConfig   *oauth2.Config
	githubOAuthConfig   *oauth2.Config
}

func NewAuthHandler(authService *service.AuthService, userService *service.UserService, subscriptionService *service.SubscriptionService, cfg *config.Config) *authHandler {
	return &authHandler{
		authService:         authService,
		userService:         userService,
		subscriptionService: subscriptionService,
		googleOAuthConfig: &oauth2.Config{
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleClientSecret,
			RedirectURL:  cfg.AppURL + "/auth/google/callback",
			Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email"},
			Endpoint:     google.Endpoint,
		},
		githubOAuthConfig: &oauth2.Config{
			ClientID:     cfg.GitHubClientID,
			ClientSecret: cfg.GitHubClientSecret,
			RedirectURL:  cfg.AppURL + "/auth/github/callback",
			Scopes:       []string{"user:email"},
			Endpoint:     github.Endpoint,
		},
	}
}

func (h *authHandler) Logout(w http.ResponseWriter, r *http.Request) {
	h.authService.ClearJWTCookie(w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *authHandler) ForgotPasswordPage(w http.ResponseWriter, r *http.Request) {
	ui.Render(w, r, pages.ForgotPassword(""))
}

func (h *authHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(r.FormValue("email"))

	if email == "" {
		ui.Render(w, r, pages.ForgotPassword("Email is required"))
		return
	}

	err := validation.ValidateEmail(email)
	if err != nil {
		ui.Render(w, r, pages.ForgotPassword("Please provide a valid email address"))
		return
	}

	err = h.authService.SendForgotPasswordLink(email)
	if err != nil {
		// Don't reveal specific errors to user
		slog.Warn("forgot password link send failed", "error", err, "email", email)
	}

	// Always show success message to prevent email enumeration
	ui.Render(w, r, pages.ForgotPasswordSent(email))
}

func (h *authHandler) VerifyForgotPassword(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")

	user, err := h.authService.VerifyMagicLink(token)
	if err != nil {
		slog.Warn("forgot password verification failed", "error", err, "token", token)
		ui.Render(w, r, pages.Auth("Invalid or expired link. Please try again."))
		return
	}

	if user.HasPassword() {
		err = h.authService.RemovePassword(user.ID)
		if err != nil {
			slog.Error("failed to remove password during forgot password flow", "error", err, "user_id", user.ID)
			ui.Render(w, r, pages.Auth("An error occurred. Please try again."))
			return
		}
		slog.Info("password removed via forgot password flow", "user_id", user.ID)
	}

	jwtToken, err := h.authService.GenerateJWT(user)
	if err != nil {
		slog.Error("failed to generate JWT", "error", err, "user_id", user.ID)
		ui.Render(w, r, pages.Auth("An error occurred. Please try again."))
		return
	}

	h.authService.SetJWTCookie(w, jwtToken, time.Now().Add(7*24*time.Hour))

	slog.Info("user logged in via forgot password flow", "user_id", user.ID, "email", user.Email)

	// Redirect to settings with query param for toast notification
	http.Redirect(w, r, "/app/settings?password_removed=1", http.StatusSeeOther)
}

func (h *authHandler) VerifyEmailChange(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")

	user, err := h.authService.VerifyEmailChange(token)
	if err != nil {
		slog.Warn("email change verification failed", "error", err, "token", token)
		ui.Render(w, r, pages.VerifyEmailError("Invalid or expired verification link"))
		return
	}

	jwtToken, err := h.authService.GenerateJWT(user)
	if err != nil {
		slog.Error("failed to generate JWT after email change", "error", err, "user_id", user.ID)
		ui.Render(w, r, pages.VerifyEmailError("An error occurred. Please try again."))
		return
	}

	h.authService.SetJWTCookie(w, jwtToken, time.Now().Add(7*24*time.Hour))

	slog.Info("email changed", "user_id", user.ID, "new_email", user.Email)
	ui.Render(w, r, pages.VerifyEmailSuccess())
}

func (h *authHandler) SendMagicLink(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(r.FormValue("email"))

	if email == "" {
		ui.Render(w, r, pages.Auth("Email is required"))
		return
	}

	err := validation.ValidateEmail(email)
	if err != nil {
		ui.Render(w, r, pages.Auth("Please provide a valid email address"))
		return
	}

	err = h.authService.SendMagicLink(email)
	if err != nil {
		// Don't reveal specific errors to prevent email enumeration
		slog.Warn("magic link send failed", "error", err, "email", email)
	}

	if r.URL.Query().Get("resend") == "true" {
		ui.RenderOOB(w, r, toast.Toast(toast.Props{
			Title:       "Magic link sent",
			Description: "Check your email for a new magic link",
			Variant:     toast.VariantSuccess,
			Icon:        true,
			Dismissible: true,
			Duration:    5000,
		}), "beforeend:#toast-container")
		return
	}

	ui.Render(w, r, pages.MagicLinkSent(email))
}

func (h *authHandler) VerifyMagicLink(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")

	user, err := h.authService.VerifyMagicLink(token)
	if err != nil {
		slog.Warn("magic link verification failed", "error", err, "token", token)
		ui.Render(w, r, pages.Auth("Invalid or expired magic link. Please try again."))
		return
	}

	jwtToken, err := h.authService.GenerateJWT(user)
	if err != nil {
		slog.Error("failed to generate JWT", "error", err, "user_id", user.ID)
		ui.Render(w, r, pages.Auth("An error occurred. Please try again."))
		return
	}

	h.authService.SetJWTCookie(w, jwtToken, time.Now().Add(7*24*time.Hour))

	needsOnboarding, err := h.authService.NeedsOnboarding(user.ID)
	if err != nil {
		slog.Warn("failed to check onboarding status", "error", err, "user_id", user.ID)
	}

	if needsOnboarding {
		slog.Info("new user needs onboarding", "user_id", user.ID, "email", user.Email)
		http.Redirect(w, r, "/auth/onboarding", http.StatusSeeOther)
		return
	}

	slog.Info("user logged in via magic link", "user_id", user.ID, "email", user.Email)
	http.Redirect(w, r, "/app/dashboard", http.StatusSeeOther)
}

func (h *authHandler) AuthPage(w http.ResponseWriter, r *http.Request) {
	ui.Render(w, r, pages.Auth(""))
}

func (h *authHandler) PasswordPage(w http.ResponseWriter, r *http.Request) {
	ui.Render(w, r, pages.AuthPassword(""))
}

func (h *authHandler) OnboardingPage(w http.ResponseWriter, r *http.Request) {
	ui.Render(w, r, pages.Onboarding(""))
}

func (h *authHandler) CompleteOnboarding(w http.ResponseWriter, r *http.Request) {
	user := ctxkeys.User(r.Context())
	if user == nil {
		http.Redirect(w, r, "/auth", http.StatusSeeOther)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))

	err := h.authService.CompleteOnboarding(user.ID, name)
	if err != nil {
		slog.Error("onboarding failed", "error", err, "user_id", user.ID)
		ui.Render(w, r, pages.Onboarding("Please enter your name"))
		return
	}

	slog.Info("onboarding completed", "user_id", user.ID, "name", name)
	http.Redirect(w, r, "/app/dashboard", http.StatusSeeOther)
}

func (h *authHandler) PasswordAuth(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")

	if email == "" || password == "" {
		ui.Render(w, r, pages.AuthPassword("Email and password are required"))
		return
	}

	err := validation.ValidateEmail(email)
	if err != nil {
		ui.Render(w, r, pages.AuthPassword("Please provide a valid email address"))
		return
	}

	user, err := h.authService.Login(email, password)
	if err != nil {
		slog.Warn("password login failed", "error", err, "email", email)
		ui.Render(w, r, pages.AuthPassword("Invalid email or password"))
		return
	}

	jwtToken, err := h.authService.GenerateJWT(user)
	if err != nil {
		slog.Error("failed to generate JWT", "error", err, "user_id", user.ID)
		ui.Render(w, r, pages.AuthPassword("An error occurred. Please try again."))
		return
	}

	h.authService.SetJWTCookie(w, jwtToken, time.Now().Add(7*24*time.Hour))

	slog.Info("user logged in with password", "user_id", user.ID, "email", user.Email)
	http.Redirect(w, r, "/app/dashboard", http.StatusSeeOther)
}

// GoogleAuth redirects user to Google OAuth consent screen
func (h *authHandler) GoogleAuth(w http.ResponseWriter, r *http.Request) {
	// Generate secure state token for CSRF protection
	state := generateOAuthState()

	cfg := ctxkeys.Config(r.Context())
	isProduction := cfg != nil && cfg.IsProduction()

	// Store state in secure cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		Secure:   isProduction, // Secure flag based on APP_ENV (safer than r.TLS behind load balancers)
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600, // 10 minutes
	})

	url := h.googleOAuthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// GoogleCallback handles the OAuth callback from Google
func (h *authHandler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	// Validate state parameter for CSRF protection
	state := r.URL.Query().Get("state")
	cookie, err := r.Cookie("oauth_state")
	if err != nil || cookie.Value != state || state == "" {
		slog.Warn("google oauth state validation failed", "error", err)
		ui.Render(w, r, pages.Auth("OAuth authentication failed. Please try again."))
		return
	}

	// Clear state cookie
	http.SetCookie(w, &http.Cookie{
		Name:   "oauth_state",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	code := r.URL.Query().Get("code")
	if code == "" {
		slog.Warn("google oauth callback missing code")
		ui.Render(w, r, pages.Auth("OAuth authentication failed. Please try again."))
		return
	}

	// Exchange code for token
	token, err := h.googleOAuthConfig.Exchange(context.Background(), code)
	if err != nil {
		slog.Error("google oauth token exchange failed", "error", err)
		ui.Render(w, r, pages.Auth("OAuth authentication failed. Please try again."))
		return
	}

	// Get user info from Google
	client := h.googleOAuthConfig.Client(context.Background(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		slog.Error("failed to get google user info", "error", err)
		ui.Render(w, r, pages.Auth("OAuth authentication failed. Please try again."))
		return
	}
	defer func() {
		closeErr := resp.Body.Close()
		if closeErr != nil {
			slog.Error("failed to close response body", "error", closeErr)
		}
	}()

	var userInfo struct {
		Email string `json:"email"`
	}
	err = json.NewDecoder(resp.Body).Decode(&userInfo)
	if err != nil {
		slog.Error("failed to decode google user info", "error", err)
		ui.Render(w, r, pages.Auth("OAuth authentication failed. Please try again."))
		return
	}

	// Authenticate or create user
	user, err := h.authService.AuthenticateOAuth(userInfo.Email, "google")
	if err != nil {
		slog.Error("oauth authentication failed", "error", err, "email", userInfo.Email)
		ui.Render(w, r, pages.Auth("Authentication failed. Please try again."))
		return
	}

	// Generate JWT
	jwtToken, err := h.authService.GenerateJWT(user)
	if err != nil {
		slog.Error("failed to generate JWT", "error", err, "user_id", user.ID)
		ui.Render(w, r, pages.Auth("An error occurred. Please try again."))
		return
	}

	h.authService.SetJWTCookie(w, jwtToken, time.Now().Add(7*24*time.Hour))

	slog.Info("user logged in with google oauth", "user_id", user.ID, "email", user.Email)

	// Check if user needs onboarding
	needsOnboarding, err := h.authService.NeedsOnboarding(user.ID)
	if err != nil {
		slog.Warn("failed to check onboarding status", "error", err, "user_id", user.ID)
	}

	if needsOnboarding {
		http.Redirect(w, r, "/auth/onboarding", http.StatusSeeOther)
	} else {
		http.Redirect(w, r, "/app/dashboard", http.StatusSeeOther)
	}
}

// GitHubAuth redirects user to GitHub OAuth consent screen
func (h *authHandler) GitHubAuth(w http.ResponseWriter, r *http.Request) {
	// Generate secure state token for CSRF protection
	state := generateOAuthState()

	// Get config from context to determine if we're in production
	cfg := ctxkeys.Config(r.Context())
	isProduction := cfg != nil && cfg.IsProduction()

	// Store state in secure cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		Secure:   isProduction, // Secure flag based on APP_ENV (safer than r.TLS behind load balancers)
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600, // 10 minutes
	})

	url := h.githubOAuthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// GitHubCallback handles the OAuth callback from GitHub
func (h *authHandler) GitHubCallback(w http.ResponseWriter, r *http.Request) {
	// Validate state parameter for CSRF protection
	state := r.URL.Query().Get("state")
	cookie, err := r.Cookie("oauth_state")
	if err != nil || cookie.Value != state || state == "" {
		slog.Warn("github oauth state validation failed", "error", err)
		ui.Render(w, r, pages.Auth("OAuth authentication failed. Please try again."))
		return
	}

	// Clear state cookie
	http.SetCookie(w, &http.Cookie{
		Name:   "oauth_state",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	code := r.URL.Query().Get("code")
	if code == "" {
		slog.Warn("github oauth callback missing code")
		ui.Render(w, r, pages.Auth("OAuth authentication failed. Please try again."))
		return
	}

	// Exchange code for token
	token, err := h.githubOAuthConfig.Exchange(context.Background(), code)
	if err != nil {
		slog.Error("github oauth token exchange failed", "error", err)
		ui.Render(w, r, pages.Auth("OAuth authentication failed. Please try again."))
		return
	}

	// Get user info from GitHub
	client := h.githubOAuthConfig.Client(context.Background(), token)
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		slog.Error("failed to get github user info", "error", err)
		ui.Render(w, r, pages.Auth("OAuth authentication failed. Please try again."))
		return
	}
	defer func() {
		closeErr := resp.Body.Close()
		if closeErr != nil {
			slog.Error("failed to close response body", "error", closeErr)
		}
	}()

	var userInfo struct {
		Email string `json:"email"`
	}
	err = json.NewDecoder(resp.Body).Decode(&userInfo)
	if err != nil {
		slog.Error("failed to decode github user info", "error", err)
		ui.Render(w, r, pages.Auth("OAuth authentication failed. Please try again."))
		return
	}

	// GitHub API may not return email in main response if it's private
	// Need to fetch from /user/emails endpoint
	if userInfo.Email == "" {
		emailResp, err := client.Get("https://api.github.com/user/emails")
		if err != nil {
			slog.Error("failed to get github user emails", "error", err)
			ui.Render(w, r, pages.Auth("OAuth authentication failed. Please try again."))
			return
		}
		defer func() {
			closeErr := emailResp.Body.Close()
			if closeErr != nil {
				slog.Error("failed to close email response body", "error", closeErr)
			}
		}()

		var emails []struct {
			Email   string `json:"email"`
			Primary bool   `json:"primary"`
		}
		err = json.NewDecoder(emailResp.Body).Decode(&emails)
		if err != nil {
			slog.Error("failed to decode github emails", "error", err)
			ui.Render(w, r, pages.Auth("OAuth authentication failed. Please try again."))
			return
		}

		// Find primary email
		for _, e := range emails {
			if e.Primary {
				userInfo.Email = e.Email
				break
			}
		}
	}

	if userInfo.Email == "" {
		slog.Warn("github oauth: no email found")
		ui.Render(w, r, pages.Auth("Could not retrieve email from GitHub. Please make sure your email is public."))
		return
	}

	// Authenticate or create user
	user, err := h.authService.AuthenticateOAuth(userInfo.Email, "github")
	if err != nil {
		slog.Error("oauth authentication failed", "error", err, "email", userInfo.Email)
		ui.Render(w, r, pages.Auth("Authentication failed. Please try again."))
		return
	}

	// Generate JWT
	jwtToken, err := h.authService.GenerateJWT(user)
	if err != nil {
		slog.Error("failed to generate JWT", "error", err, "user_id", user.ID)
		ui.Render(w, r, pages.Auth("An error occurred. Please try again."))
		return
	}

	h.authService.SetJWTCookie(w, jwtToken, time.Now().Add(7*24*time.Hour))

	slog.Info("user logged in with github oauth", "user_id", user.ID, "email", user.Email)

	// Check if user needs onboarding
	needsOnboarding, err := h.authService.NeedsOnboarding(user.ID)
	if err != nil {
		slog.Warn("failed to check onboarding status", "error", err, "user_id", user.ID)
	}

	if needsOnboarding {
		http.Redirect(w, r, "/auth/onboarding", http.StatusSeeOther)
	} else {
		http.Redirect(w, r, "/app/dashboard", http.StatusSeeOther)
	}
}

// generateOAuthState creates cryptographically secure random state token for OAuth CSRF protection
func generateOAuthState() string {
	bytes := make([]byte, 32)
	_, err := rand.Read(bytes)
	if err != nil {
		panic("failed to generate oauth state: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(bytes)
}
