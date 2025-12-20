package routes

import (
	"io/fs"
	"net/http"

	"github.com/jkeddari/hypermetrics/assets"
	"github.com/jkeddari/hypermetrics/internal/app"
	"github.com/jkeddari/hypermetrics/internal/handler"
	"github.com/jkeddari/hypermetrics/internal/middleware"
)

func SetupRoutes(app *app.App) http.Handler {
	// Handlers
	home := handler.NewHomeHandler()
	seo := handler.NewSEOHandler(app.DocsService, app.Cfg.AppURL)
	docs := handler.NewDocsHandler(app.DocsService)
	legal := handler.NewLegalHandler(app.LegalService)
	newsletter := handler.NewNewsletterHandler(app.EmailService)
	auth := handler.NewAuthHandler(app.AuthService, app.UserService, app.SubscriptionService, app.Cfg)
	account := handler.NewAccountHandler(app.AuthService, app.UserService, app.FileService)
	profile := handler.NewProfileHandler(app.ProfileService)
	dashboard := handler.NewDashboardHandler()
	settings := handler.NewSettingsHandler()
	// goal := handler.NewGoalHandler(app.GoalService)
	billing := handler.NewBillingHandler(app.SubscriptionService, app.PaymentService)
	leaderboard := handler.NewLeaderboardHandler(app.LeaderboardService)
	topPositions := handler.NewTopPositionsHandler()

	mux := http.NewServeMux()

	// ============================================================================
	// PUBLIC ROUTES
	// ============================================================================

	// Static files
	sub, _ := fs.Sub(assets.AssetsFS, ".")
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(sub))))

	// SEO
	mux.HandleFunc("GET /robots.txt", seo.Robots)
	mux.HandleFunc("GET /sitemap.xml", seo.Sitemap)

	// Home
	mux.HandleFunc("GET /{$}", home.HomePage)

	// Content
	mux.HandleFunc("GET /docs", docs.ShowDocs)
	mux.HandleFunc("GET /docs/", docs.ShowDocs)
	mux.HandleFunc("GET /legal/{page}", legal.ShowPage)

	// Newsletter
	mux.HandleFunc("POST /newsletter/subscribe", newsletter.Subscribe)

	// Auth - Authentication flow (rate limited)
	rateLimiter := middleware.RateLimitAuth()

	// Auth Pages
	mux.HandleFunc("GET /auth", middleware.RequireGuest(auth.AuthPage))
	mux.HandleFunc("GET /auth/password", middleware.RequireGuest(auth.PasswordPage))
	mux.HandleFunc("GET /auth/forgot-password", middleware.RequireGuest(auth.ForgotPasswordPage))
	mux.HandleFunc("GET /auth/onboarding", middleware.RequireAuth(auth.OnboardingPage))

	// OAuth
	mux.HandleFunc("GET /auth/google", rateLimiter(middleware.RequireGuest(auth.GoogleAuth)))
	mux.HandleFunc("GET /auth/google/callback", rateLimiter(auth.GoogleCallback))
	mux.HandleFunc("GET /auth/github", rateLimiter(middleware.RequireGuest(auth.GitHubAuth)))
	mux.HandleFunc("GET /auth/github/callback", rateLimiter(auth.GitHubCallback))

	// Token Verifications
	mux.HandleFunc("GET /auth/magic-link/{token}", auth.VerifyMagicLink)
	mux.HandleFunc("GET /auth/forgot-password/{token}", auth.VerifyForgotPassword)
	mux.HandleFunc("GET /auth/verify-email-change/{token}", auth.VerifyEmailChange)

	// Auth Actions
	mux.HandleFunc("POST /auth/magic-link", rateLimiter(middleware.RequireGuest(auth.SendMagicLink)))
	mux.HandleFunc("POST /auth/password", rateLimiter(middleware.RequireGuest(auth.PasswordAuth)))
	mux.HandleFunc("POST /auth/forgot-password", rateLimiter(middleware.RequireGuest(auth.ForgotPassword)))
	mux.HandleFunc("POST /auth/onboarding", middleware.RequireAuth(auth.CompleteOnboarding))
	mux.HandleFunc("POST /auth/logout", auth.Logout)

	// ============================================================================
	// PROTECTED ROUTES (/app/*)
	// ============================================================================

	// App Pages
	mux.HandleFunc("GET /app/dashboard", middleware.RequireAuth(dashboard.DashboardPage))
	mux.HandleFunc("GET /app/settings", middleware.RequireAuth(settings.SettingsPage))

	// Profile
	mux.HandleFunc("PATCH /app/profile/name", middleware.RequireAuth(profile.UpdateName))

	// Account (Security & Identity)
	mux.HandleFunc("PATCH /app/account/email", middleware.RequireAuth(account.ChangeEmail))
	mux.HandleFunc("POST /app/account/password", middleware.RequireAuth(account.ChangePassword))
	mux.HandleFunc("POST /app/account/avatar", middleware.RequireAuth(account.UploadAvatar))
	mux.HandleFunc("DELETE /app/account/avatar", middleware.RequireAuth(account.DeleteAvatar))
	mux.HandleFunc("POST /app/account/password/set", middleware.RequireAuth(account.SetPassword))
	mux.HandleFunc("DELETE /app/account/password", middleware.RequireAuth(account.RemovePassword))
	mux.HandleFunc("DELETE /app/account", middleware.RequireAuth(account.DeleteAccount))

	// Billing
	mux.HandleFunc("GET /app/billing", middleware.RequireAuth(billing.BillingPage))
	mux.HandleFunc("POST /app/billing/checkout", middleware.RequireAuth(billing.CreateCheckout))
	mux.HandleFunc("GET /app/billing/portal", middleware.RequireAuth(billing.CustomerPortal))

	// Leaderboard
	mux.HandleFunc("GET /app/leaderboard", middleware.RequireAuth(leaderboard.LeaderboardPage))

	// Top-positions
	mux.HandleFunc("GET /app/top-positions", middleware.RequireAuth(topPositions.TopPositionsPage))

	// Goals
	// mux.HandleFunc("GET /app/goals", middleware.RequireAuth(goal.GoalsPage))
	// mux.HandleFunc("GET /app/goals/{id}", middleware.RequireAuth(goal.GoalDetailPage))
	// mux.HandleFunc("GET /app/goals/{id}/edit-dialog", middleware.RequireAuth(goal.EditDialog))
	// mux.HandleFunc("GET /app/goals/{id}/delete-dialog", middleware.RequireAuth(goal.DeleteDialog))
	// mux.HandleFunc("GET /app/goals/{id}/entries/{step}/dialog", middleware.RequireAuth(goal.EntryDialog))
	// mux.HandleFunc("GET /app/goals/export", middleware.RequireAuth(goal.Export))
	// mux.HandleFunc("POST /app/goals", middleware.RequireAuth(goal.Create))
	// mux.HandleFunc("POST /app/goals/{id}/entries/{step}/complete", middleware.RequireAuth(goal.CompleteEntry))
	// mux.HandleFunc("PUT /app/goals/{id}", middleware.RequireAuth(goal.Update))
	// mux.HandleFunc("PATCH /app/goals/{id}/entries/{step}", middleware.RequireAuth(goal.UpdateEntry))
	// mux.HandleFunc("DELETE /app/goals/{id}", middleware.RequireAuth(goal.Delete))
	// mux.HandleFunc("DELETE /app/goals/{id}/entries/{step}", middleware.RequireAuth(goal.UncompleteEntry))

	// ============================================================================
	// WEBHOOKS
	// ============================================================================

	// Payment provider webhook (works with both Polar and Stripe)
	mux.HandleFunc("POST /webhooks/payment", billing.Webhook)

	// ============================================================================
	// FALLBACK
	// ============================================================================

	// 404
	mux.HandleFunc("/{path...}", home.NotFoundPage)

	// Global middleware - executed in order (top to bottom)
	handler := middleware.Chain(
		mux,
		middleware.Config(app.Cfg), // Config must be first (needed by SecurityHeaders for S3 endpoint)
		middleware.NonceMiddleware, // Generate CSP nonce for each request (must be before SecurityHeaders)
		middleware.SecurityHeaders, // Security headers for all responses (XSS, clickjacking, etc.)
		middleware.RequestLogging,
		middleware.CSRFProtection, // CSRF protection for all state-changing requests
		middleware.AuthMiddleware(app.AuthService, app.UserService, app.ProfileService, app.SubscriptionService),
		middleware.WithURLPath,
	)

	return handler
}
