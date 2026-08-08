package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jkeddari/hypermetrics/internal/db"
)

func TestSessionAndCSRFProtection(t *testing.T) {
	auth := NewAuthService(nil, "0123456789abcdef0123456789abcdef", false, time.Hour, "http://localhost:3000", "", "")

	sessionResponse := httptest.NewRecorder()
	auth.SetSession(sessionResponse, "usr_test")
	sessionCookie := sessionResponse.Result().Cookies()[0]
	userID, err := auth.verifySession(sessionCookie.Value)
	if err != nil || userID != "usr_test" {
		t.Fatalf("unexpected session result: user=%q err=%v", userID, err)
	}
	if _, err := auth.verifySession(sessionCookie.Value + "tampered"); err == nil {
		t.Fatal("tampered session must be rejected")
	}

	csrfResponse := httptest.NewRecorder()
	csrfRequest := httptest.NewRequest("GET", "/register", nil)
	token := auth.CSRFToken(csrfResponse, csrfRequest)
	csrfCookie := csrfResponse.Result().Cookies()[0]

	form := url.Values{"csrf_token": {token}}
	postRequest := httptest.NewRequest("POST", "/register", strings.NewReader(form.Encode()))
	postRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	postRequest.AddCookie(csrfCookie)
	if err := auth.ValidateCSRF(postRequest); err != nil {
		t.Fatalf("valid CSRF token rejected: %v", err)
	}

	badForm := url.Values{"csrf_token": {"wrong"}}
	badRequest := httptest.NewRequest("POST", "/register", strings.NewReader(badForm.Encode()))
	badRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	badRequest.AddCookie(csrfCookie)
	if err := auth.ValidateCSRF(badRequest); err == nil {
		t.Fatal("invalid CSRF token must be rejected")
	}
}

func TestAuthTokensAreStoredAsHashes(t *testing.T) {
	raw := "a-secret-that-will-be-sent-by-email"
	if got := authTokenHash(raw); got == raw || len(got) != 64 {
		t.Fatalf("unexpected token hash %q", got)
	}
	if authTokenHash(raw) != authTokenHash(raw) || authTokenHash(raw) == authTokenHash(raw+"x") {
		t.Fatal("token hashes must be stable and unique to their input")
	}
}

func TestAccountSettingsValidation(t *testing.T) {
	if got, err := normalizeProfileName("  Jordan  "); err != nil || got != "Jordan" {
		t.Fatalf("normalized name = %q, err = %v", got, err)
	}
	if _, err := normalizeProfileName(strings.Repeat("a", 81)); !errors.Is(err, ErrInvalidProfileName) {
		t.Fatalf("long name error = %v", err)
	}
	if _, err := normalizeProfileName("bad\nname"); !errors.Is(err, ErrInvalidProfileName) {
		t.Fatalf("control character error = %v", err)
	}
	if err := validatePassword("too-short"); !errors.Is(err, ErrWeakPassword) {
		t.Fatalf("short password error = %v", err)
	}
	if err := validatePassword("a-secure-password"); err != nil {
		t.Fatalf("valid password error = %v", err)
	}
}

func TestEmailVerificationAndMagicLink(t *testing.T) {
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

	email := "auth-flow-test@example.com"
	magicEmail := "magic-auth-flow-test@example.com"
	cleanup := func() { _, _ = database.Exec(`DELETE FROM users WHERE email IN ($1, $2)`, email, magicEmail) }
	cleanup()
	defer cleanup()

	messages := make(chan string, 2)
	resend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Text string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		messages <- payload.Text
		w.WriteHeader(http.StatusOK)
	}))
	defer resend.Close()

	auth := NewAuthService(database, "0123456789abcdef0123456789abcdef", false, time.Hour, "https://test.example", "re_test", "Hypermetrics <hello@example.com>")
	auth.email.resendURL = resend.URL
	auth.email.httpClient = resend.Client()
	if err := auth.SendMagicLink(context.Background(), magicEmail); err != nil {
		t.Fatalf("magic-link signup failed: %v", err)
	}
	magicSignupToken := tokenFromEmail(t, <-messages, "/magic-link/")
	magicUser, err := auth.VerifyMagicLink(context.Background(), magicSignupToken)
	if err != nil {
		t.Fatalf("verify magic-link signup: %v", err)
	}
	if magicUser.Email != magicEmail || magicUser.EmailVerifiedAt == nil || magicUser.PasswordHash != nil {
		t.Fatalf("unexpected passwordless user: %+v", magicUser)
	}
	if err := auth.UpdatePassword(context.Background(), magicUser.ID, "", "passwordless-secure"); err != nil {
		t.Fatalf("set password on passwordless account: %v", err)
	}
	if _, err := auth.Login(magicEmail, "passwordless-secure"); err != nil {
		t.Fatalf("passwordless account password login failed: %v", err)
	}
	if _, err := auth.VerifyMagicLink(context.Background(), magicSignupToken); !errors.Is(err, ErrInvalidAuthToken) {
		t.Fatalf("reused signup magic link error = %v", err)
	}

	if _, err := auth.Register(context.Background(), email, "a-secure-password", "pro"); err != nil {
		t.Fatal(err)
	}
	verificationToken := tokenFromEmail(t, <-messages, "/verify-email/")
	if _, err := auth.Login(email, "a-secure-password"); !errors.Is(err, ErrEmailNotVerified) {
		t.Fatalf("unverified login error = %v", err)
	}
	user, planID, err := auth.VerifyEmail(context.Background(), verificationToken)
	if err != nil || planID != "pro" || user.EmailVerifiedAt == nil {
		t.Fatalf("verification result: user=%+v plan=%q err=%v", user, planID, err)
	}
	if _, err := auth.Login(email, "a-secure-password"); err != nil {
		t.Fatalf("verified password login failed: %v", err)
	}
	if err := auth.UpdateProfile(context.Background(), user.ID, "  Jane ", " Doe "); err != nil {
		t.Fatalf("update profile: %v", err)
	}
	profiledUser, err := auth.Login(email, "a-secure-password")
	if err != nil || profiledUser.FirstName != "Jane" || profiledUser.LastName != "Doe" {
		t.Fatalf("updated profile: user=%+v err=%v", profiledUser, err)
	}
	if err := auth.UpdatePassword(context.Background(), user.ID, "wrong-password", "a-new-secure-password"); !errors.Is(err, ErrInvalidCurrentPassword) {
		t.Fatalf("wrong current password error = %v", err)
	}
	if err := auth.UpdatePassword(context.Background(), user.ID, "a-secure-password", "a-new-secure-password"); err != nil {
		t.Fatalf("update password: %v", err)
	}
	if _, err := auth.Login(email, "a-secure-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("old password login error = %v", err)
	}
	if _, err := auth.Login(email, "a-new-secure-password"); err != nil {
		t.Fatalf("new password login failed: %v", err)
	}

	if err := auth.SendMagicLink(context.Background(), email); err != nil {
		t.Fatal(err)
	}
	magicToken := tokenFromEmail(t, <-messages, "/magic-link/")
	if _, err := auth.VerifyMagicLink(context.Background(), magicToken); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.VerifyMagicLink(context.Background(), magicToken); !errors.Is(err, ErrInvalidAuthToken) {
		t.Fatalf("reused magic link error = %v", err)
	}
}

func tokenFromEmail(t *testing.T, body, marker string) string {
	t.Helper()
	_, suffix, ok := strings.Cut(body, marker)
	if !ok {
		t.Fatalf("email body does not contain %q: %q", marker, body)
	}
	return strings.TrimSpace(strings.SplitN(suffix, "\n", 2)[0])
}

func TestStaticAPIKeyStillWorks(t *testing.T) {
	service := NewAPIKeyService(nil, "legacy-key")
	key, err := service.ValidateAPIKey("legacy-key")
	if err != nil || !key.Active {
		t.Fatalf("static API key rejected: key=%+v err=%v", key, err)
	}
	if _, err := service.ValidateAPIKey("wrong-key"); err == nil {
		t.Fatal("unknown API key must be rejected")
	}
}
