package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jkeddari/hypermetrics/internal/model"
	"golang.org/x/crypto/bcrypt"
)

const (
	sessionCookieName     = "hm_session"
	csrfCookieName        = "hm_csrf"
	emailVerificationType = "email_verification"
	magicLinkType         = "magic_link"
)

var (
	ErrEmailAlreadyExists     = errors.New("an account already exists for this email")
	ErrInvalidCredentials     = errors.New("invalid email or password")
	ErrInvalidEmail           = errors.New("enter a valid email address")
	ErrWeakPassword           = errors.New("password must contain between 12 and 72 characters")
	ErrEmailNotVerified       = errors.New("email is not verified")
	ErrEmailDelivery          = errors.New("unable to deliver email")
	ErrInvalidAuthToken       = errors.New("invalid or expired authentication link")
	ErrInvalidSession         = errors.New("invalid session")
	ErrInvalidCSRFToken       = errors.New("invalid CSRF token")
	ErrInvalidProfileName     = errors.New("names must contain at most 80 characters and no control characters")
	ErrInvalidCurrentPassword = errors.New("current password is incorrect")
)

type AuthService struct {
	db            *sql.DB
	secret        []byte
	secureCookies bool
	sessionExpiry time.Duration
	appURL        string
	email         *emailSender
}

func NewAuthService(db *sql.DB, secret string, secureCookies bool, sessionExpiry time.Duration, appURL, resendAPIKey, resendFrom string) *AuthService {
	return &AuthService{
		db:            db,
		secret:        []byte(secret),
		secureCookies: secureCookies,
		sessionExpiry: sessionExpiry,
		appURL:        strings.TrimRight(appURL, "/"),
		email:         newEmailSender(resendAPIKey, resendFrom, !secureCookies),
	}
}

func (s *AuthService) Register(ctx context.Context, email, password, planID string) (*model.User, error) {
	email = normalizeEmail(email)
	if !validEmail(email) {
		return nil, ErrInvalidEmail
	}
	if err := validatePassword(password); err != nil {
		return nil, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	id, err := randomToken(18)
	if err != nil {
		return nil, err
	}
	rawToken, err := randomToken(32)
	if err != nil {
		return nil, err
	}
	passwordHash := string(hash)
	user := &model.User{ID: "usr_" + id, Email: email, PasswordHash: &passwordHash, CreatedAt: time.Now().UTC()}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin registration: %w", err)
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO users (id, email, password_hash, created_at) VALUES ($1, $2, $3, $4)`, user.ID, user.Email, user.PasswordHash, user.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrEmailAlreadyExists
		}
		return nil, fmt.Errorf("create user: %w", err)
	}
	if err := replaceAuthToken(ctx, tx, user.ID, emailVerificationType, rawToken, planID, 24*time.Hour); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit registration: %w", err)
	}
	verifyURL := s.appURL + "/verify-email/" + rawToken
	body := fmt.Sprintf("Verify your Hypermetrics email address:\n%s\n\nThis link expires in 24 hours and can only be used once. If you did not create this account, you can ignore this email.", verifyURL)
	if err := s.email.send(ctx, user.Email, "Verify your Hypermetrics email", body); err != nil {
		return user, fmt.Errorf("%w: %v", ErrEmailDelivery, err)
	}
	return user, nil
}

func (s *AuthService) Login(email, password string) (*model.User, error) {
	email = normalizeEmail(email)
	var user model.User
	err := s.db.QueryRow(`SELECT id, email, password_hash, email_verified_at, first_name, last_name, created_at FROM users WHERE email = $1`, email).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.EmailVerifiedAt, &user.FirstName, &user.LastName, &user.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, fmt.Errorf("load user: %w", err)
	}
	if user.PasswordHash == nil || bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(password)) != nil {
		return nil, ErrInvalidCredentials
	}
	if user.EmailVerifiedAt == nil {
		return nil, ErrEmailNotVerified
	}
	return &user, nil
}

func (s *AuthService) SendMagicLink(ctx context.Context, email string) error {
	email = normalizeEmail(email)
	if !validEmail(email) {
		return ErrInvalidEmail
	}
	newUserID, err := randomToken(18)
	if err != nil {
		return err
	}
	rawToken, err := randomToken(32)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin magic link: %w", err)
	}
	defer tx.Rollback()
	var userID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO users (id, email, password_hash, created_at)
		VALUES ($1, $2, NULL, $3)
		ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
		RETURNING id
	`, "usr_"+newUserID, email, time.Now().UTC()).Scan(&userID)
	if err != nil {
		return fmt.Errorf("find or create magic-link user: %w", err)
	}
	if err := replaceAuthToken(ctx, tx, userID, magicLinkType, rawToken, "", 15*time.Minute); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit magic link: %w", err)
	}
	magicURL := s.appURL + "/magic-link/" + rawToken
	body := fmt.Sprintf("Sign in to Hypermetrics:\n%s\n\nThis link expires in 15 minutes and can only be used once. If you did not request it, you can ignore this email.", magicURL)
	htmlBody := fmt.Sprintf(`<!doctype html>
<html lang="en"><body style="margin:0;background:#f6f7f9;font-family:Arial,sans-serif;color:#171717">
  <div style="max-width:560px;margin:32px auto;background:#ffffff;padding:40px;border-radius:12px">
    <h1 style="margin:0 0 16px;font-size:24px">Sign in to Hypermetrics</h1>
    <p style="line-height:1.5">Use the button below to securely sign in. This link expires in 15 minutes and can only be used once.</p>
    <p style="margin:28px 0"><a href="%[1]s" style="display:inline-block;background:#65b800;color:#ffffff;padding:12px 20px;border-radius:8px;text-decoration:none;font-weight:700">Sign in to Hypermetrics</a></p>
    <p style="line-height:1.5">If the button does not work, copy and paste this link into your browser:</p>
    <p style="word-break:break-all"><a href="%[1]s" style="color:#3f7600">%[1]s</a></p>
    <p style="margin:28px 0 0;color:#6b7280;font-size:13px;line-height:1.5">If you did not request this email, you can safely ignore it.</p>
  </div>
</body></html>`, html.EscapeString(magicURL))
	if err := s.email.send(ctx, email, "Your Hypermetrics sign-in link", body, htmlBody); err != nil {
		return fmt.Errorf("%w: %v", ErrEmailDelivery, err)
	}
	return nil
}

func (s *AuthService) VerifyEmail(ctx context.Context, rawToken string) (*model.User, string, error) {
	return s.consumeAuthToken(ctx, rawToken, emailVerificationType)
}

func (s *AuthService) VerifyMagicLink(ctx context.Context, rawToken string) (*model.User, error) {
	user, _, err := s.consumeAuthToken(ctx, rawToken, magicLinkType)
	return user, err
}

func (s *AuthService) consumeAuthToken(ctx context.Context, rawToken, tokenType string) (*model.User, string, error) {
	if rawToken == "" {
		return nil, "", ErrInvalidAuthToken
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, "", err
	}
	defer tx.Rollback()
	var userID string
	var planID sql.NullString
	err = tx.QueryRowContext(ctx, `
		DELETE FROM auth_tokens
		WHERE token_hash = $1 AND type = $2 AND expires_at > now()
		RETURNING user_id, plan_id
	`, authTokenHash(rawToken), tokenType).Scan(&userID, &planID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", ErrInvalidAuthToken
	}
	if err != nil {
		return nil, "", fmt.Errorf("consume auth token: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET email_verified_at = COALESCE(email_verified_at, now()) WHERE id = $1`, userID); err != nil {
		return nil, "", fmt.Errorf("verify email: %w", err)
	}
	var user model.User
	err = tx.QueryRowContext(ctx, `SELECT id, email, password_hash, email_verified_at, first_name, last_name, created_at FROM users WHERE id = $1`, userID).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.EmailVerifiedAt, &user.FirstName, &user.LastName, &user.CreatedAt)
	if err != nil {
		return nil, "", fmt.Errorf("load verified user: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, "", err
	}
	return &user, planID.String, nil
}

func replaceAuthToken(ctx context.Context, tx *sql.Tx, userID, tokenType, rawToken, planID string, expiry time.Duration) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM auth_tokens WHERE user_id = $1 AND type = $2`, userID, tokenType); err != nil {
		return fmt.Errorf("replace auth token: %w", err)
	}
	var plan any
	if planID != "" {
		plan = planID
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO auth_tokens (token_hash, user_id, type, plan_id, expires_at)
		VALUES ($1, $2, $3, $4, $5)
	`, authTokenHash(rawToken), userID, tokenType, plan, time.Now().UTC().Add(expiry))
	if err != nil {
		return fmt.Errorf("create auth token: %w", err)
	}
	return nil
}

func authTokenHash(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}

func (s *AuthService) UserFromRequest(r *http.Request) (*model.User, error) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return nil, ErrInvalidSession
	}
	userID, err := s.verifySession(cookie.Value)
	if err != nil {
		return nil, err
	}
	var user model.User
	err = s.db.QueryRow(`SELECT id, email, password_hash, email_verified_at, first_name, last_name, created_at FROM users WHERE id = $1`, userID).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.EmailVerifiedAt, &user.FirstName, &user.LastName, &user.CreatedAt)
	if err != nil {
		return nil, ErrInvalidSession
	}
	return &user, nil
}

func (s *AuthService) SetSession(w http.ResponseWriter, userID string) {
	expires := time.Now().UTC().Add(s.sessionExpiry)
	payload := base64.RawURLEncoding.EncodeToString([]byte(userID + "|" + strconv.FormatInt(expires.Unix(), 10)))
	value := payload + "." + s.sign(payload)
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: value, Path: "/", Expires: expires,
		HttpOnly: true, Secure: s.secureCookies, SameSite: http.SameSiteLaxMode,
	})
}

func (s *AuthService) ClearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: "", Path: "/", MaxAge: -1, Expires: time.Unix(0, 0),
		HttpOnly: true, Secure: s.secureCookies, SameSite: http.SameSiteLaxMode,
	})
}

func (s *AuthService) CSRFToken(w http.ResponseWriter, r *http.Request) string {
	if cookie, err := r.Cookie(csrfCookieName); err == nil && len(cookie.Value) >= 32 {
		return cookie.Value
	}
	token, err := randomToken(32)
	if err != nil {
		panic(err)
	}
	http.SetCookie(w, &http.Cookie{
		Name: csrfCookieName, Value: token, Path: "/", MaxAge: 86400,
		HttpOnly: true, Secure: s.secureCookies, SameSite: http.SameSiteStrictMode,
	})
	return token
}

func (s *AuthService) ValidateCSRF(r *http.Request) error {
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil {
		return ErrInvalidCSRFToken
	}
	formToken := r.FormValue("csrf_token")
	if len(formToken) != len(cookie.Value) || subtle.ConstantTimeCompare([]byte(formToken), []byte(cookie.Value)) != 1 {
		return ErrInvalidCSRFToken
	}
	return nil
}

func (s *AuthService) UpdateProfile(ctx context.Context, userID, firstName, lastName string) error {
	firstName, err := normalizeProfileName(firstName)
	if err != nil {
		return err
	}
	lastName, err = normalizeProfileName(lastName)
	if err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE users SET first_name = $1, last_name = $2 WHERE id = $3`, firstName, lastName, userID); err != nil {
		return fmt.Errorf("update profile: %w", err)
	}
	return nil
}

func (s *AuthService) UpdatePassword(ctx context.Context, userID, currentPassword, newPassword string) error {
	if err := validatePassword(newPassword); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin password update: %w", err)
	}
	defer tx.Rollback()
	var currentHash *string
	if err := tx.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id = $1 FOR UPDATE`, userID).Scan(&currentHash); err != nil {
		return fmt.Errorf("load password: %w", err)
	}
	if currentHash != nil && bcrypt.CompareHashAndPassword([]byte(*currentHash), []byte(currentPassword)) != nil {
		return ErrInvalidCurrentPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = $1 WHERE id = $2`, string(hash), userID); err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit password update: %w", err)
	}
	return nil
}

func (s *AuthService) verifySession(value string) (string, error) {
	payload, signature, ok := strings.Cut(value, ".")
	if !ok || !hmac.Equal([]byte(signature), []byte(s.sign(payload))) {
		return "", ErrInvalidSession
	}
	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return "", ErrInvalidSession
	}
	userID, rawExpiry, ok := strings.Cut(string(decoded), "|")
	if !ok || userID == "" {
		return "", ErrInvalidSession
	}
	expiry, err := strconv.ParseInt(rawExpiry, 10, 64)
	if err != nil || time.Now().Unix() >= expiry {
		return "", ErrInvalidSession
	}
	return userID, nil
}

func (s *AuthService) sign(value string) string {
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(value))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validEmail(email string) bool {
	address, err := mail.ParseAddress(email)
	return err == nil && address.Address == email && len(email) <= 254
}

func validatePassword(password string) error {
	if len(password) < 12 || len(password) > 72 {
		return ErrWeakPassword
	}
	return nil
}

func normalizeProfileName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if !utf8.ValidString(name) || utf8.RuneCountInString(name) > 80 || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return "", ErrInvalidProfileName
	}
	return name, nil
}
