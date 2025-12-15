package middleware

import (
	"net/http"

	"github.com/jkeddari/hypermetrics/internal/ctxkeys"
	"github.com/jkeddari/hypermetrics/internal/service"
)

// AuthMiddleware checks for JWT token and adds user + profile + subscription to context if valid
func AuthMiddleware(authService *service.AuthService, userService *service.UserService, profileService *service.ProfileService, subscriptionService *service.SubscriptionService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get JWT from cookie
			cookie, err := r.Cookie("auth_token")
			if err != nil {
				// No cookie, continue without auth
				next.ServeHTTP(w, r)
				return
			}

			// Verify token
			claims, err := authService.VerifyJWT(cookie.Value)
			if err != nil {
				// Invalid token, clear cookie and continue
				authService.ClearJWTCookie(w)
				next.ServeHTTP(w, r)
				return
			}

			// Get user ID from claims
			userID, ok := claims["user_id"].(string)
			if !ok {
				authService.ClearJWTCookie(w)
				next.ServeHTTP(w, r)
				return
			}

			// Fetch user from database
			user, err := userService.ByID(userID)
			if err != nil {
				authService.ClearJWTCookie(w)
				next.ServeHTTP(w, r)
				return
			}

			// Security: Remove password hash from context
			user.PasswordHash = nil

			profile, err := profileService.ByUserID(userID)
			if err != nil {
				// Profile not found - this shouldn't happen but handle gracefully
				authService.ClearJWTCookie(w)
				next.ServeHTTP(w, r)
				return
			}

			subscription, err := subscriptionService.Subscription(userID)
			if err != nil {
				// Subscription not found - something wrong, clear cookie
				authService.ClearJWTCookie(w)
				next.ServeHTTP(w, r)
				return
			}

			// Add user + profile + subscription to context
			ctx := ctxkeys.WithUser(r.Context(), user)
			ctx = ctxkeys.WithProfile(ctx, profile)
			ctx = ctxkeys.WithSubscription(ctx, subscription)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAuth ensures the user is authenticated and has completed onboarding
func RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ctxkeys.User(r.Context())
		if user == nil {
			// For HTMX requests, use HX-Redirect header to force full page redirect
			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("HX-Redirect", "/auth")
				w.WriteHeader(http.StatusSeeOther)
				return
			}
			// For regular requests, use standard redirect
			http.Redirect(w, r, "/auth", http.StatusSeeOther)
			return
		}

		// Check if user has completed onboarding
		// Uses profile.Name as indicator (empty = incomplete onboarding)
		profile := ctxkeys.Profile(r.Context())
		if profile.Name == "" && r.URL.Path != "/auth/onboarding" {
			// User hasn't completed onboarding, redirect to onboarding
			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("HX-Redirect", "/auth/onboarding")
				w.WriteHeader(http.StatusSeeOther)
				return
			}
			http.Redirect(w, r, "/auth/onboarding", http.StatusSeeOther)
			return
		}

		next.ServeHTTP(w, r)
	}
}

// RequireGuest ensures the user is not authenticated
func RequireGuest(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ctxkeys.User(r.Context())
		if user != nil {
			// For HTMX requests, use HX-Redirect header to force full page redirect
			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("HX-Redirect", "/app/dashboard")
				w.WriteHeader(http.StatusSeeOther)
				return
			}
			// For regular requests, use standard redirect
			http.Redirect(w, r, "/app/dashboard", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	}
}
