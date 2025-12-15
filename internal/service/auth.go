package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jkeddari/hypermetrics/internal/model"
	"github.com/jkeddari/hypermetrics/internal/repository"
	"github.com/jkeddari/hypermetrics/internal/validation"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrWeakPassword       = errors.New("password must be at least 12 characters")
	ErrCommonPassword     = errors.New("password is too common, please choose a stronger one")
	ErrEmailNotVerified   = errors.New("email not verified")
	ErrInvalidEmail       = errors.New("invalid email address")
	ErrNameRequired       = errors.New("name is required")
)

type AuthService struct {
	userRepository           repository.UserRepository
	profileRepository        repository.ProfileRepository
	tokenRepository          repository.TokenRepository
	subscriptionService      *SubscriptionService
	emailService             *EmailService
	jwtSecret                string
	isProduction             bool
	jwtExpiry                time.Duration
	tokenEmailVerifyExpiry   time.Duration
	tokenPasswordResetExpiry time.Duration
	tokenEmailChangeExpiry   time.Duration
	tokenMagicLinkExpiry     time.Duration
}

func NewAuthService(
	userRepository repository.UserRepository,
	profileRepository repository.ProfileRepository,
	tokenRepository repository.TokenRepository,
	subscriptionService *SubscriptionService,
	emailService *EmailService,
	jwtSecret string,
	isProduction bool,
	jwtExpiry time.Duration,
	tokenEmailVerifyExpiry time.Duration,
	tokenPasswordResetExpiry time.Duration,
	tokenEmailChangeExpiry time.Duration,
	tokenMagicLinkExpiry time.Duration,
) *AuthService {
	return &AuthService{
		userRepository:           userRepository,
		profileRepository:        profileRepository,
		tokenRepository:          tokenRepository,
		subscriptionService:      subscriptionService,
		emailService:             emailService,
		isProduction:             isProduction,
		jwtSecret:                jwtSecret,
		jwtExpiry:                jwtExpiry,
		tokenEmailVerifyExpiry:   tokenEmailVerifyExpiry,
		tokenPasswordResetExpiry: tokenPasswordResetExpiry,
		tokenEmailChangeExpiry:   tokenEmailChangeExpiry,
		tokenMagicLinkExpiry:     tokenMagicLinkExpiry,
	}
}

func (s *AuthService) Login(email, password string) (*model.User, error) {
	email = strings.TrimSpace(strings.ToLower(email))

	user, err := s.userRepository.ByEmail(email)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, fmt.Errorf("invalid credentials: %w", ErrInvalidCredentials)
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if !user.HasPassword() {
		return nil, fmt.Errorf("this account uses passwordless login. Please use the magic link option")
	}

	err = s.ComparePassword(password, *user.PasswordHash)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials: %w", ErrInvalidCredentials)
	}

	if user.EmailVerifiedAt == nil {
		return nil, fmt.Errorf("email not verified: %w", ErrEmailNotVerified)
	}

	return user, nil
}

func (s *AuthService) ValidatePassword(password string) error {
	return validation.ValidatePassword(password)
}

func (s *AuthService) HashPassword(password string) (string, error) {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedBytes), nil
}

func (s *AuthService) ComparePassword(password, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func (s *AuthService) GenerateToken() (string, error) {
	bytes := make([]byte, 32)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (s *AuthService) GenerateJWT(user *model.User) (string, error) {
	claims := jwt.MapClaims{
		"user_id": user.ID,
		"email":   user.Email,
		"exp":     time.Now().Add(s.jwtExpiry).Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func (s *AuthService) VerifyJWT(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.jwtSecret), nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

func (s *AuthService) SetJWTCookie(w http.ResponseWriter, token string, expiry time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    token,
		Expires:  expiry,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.isProduction,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *AuthService) ClearJWTCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    "",
		Expires:  time.Unix(0, 0),
		Path:     "/",
		HttpOnly: true,
		Secure:   s.isProduction,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *AuthService) RequestEmailChange(userID, newEmail string) error {
	newEmail = strings.TrimSpace(strings.ToLower(newEmail))

	err := validation.ValidateEmail(newEmail)
	if err != nil {
		return ErrInvalidEmail
	}

	user, err := s.userRepository.ByID(userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return fmt.Errorf("user not found: %w", err)
		}
		return fmt.Errorf("failed to get user: %w", err)
	}

	if newEmail == user.Email {
		return fmt.Errorf("email is already set to this value: %w", ErrInvalidEmail)
	}

	existingUser, err := s.userRepository.ByEmail(newEmail)
	if err != nil && !errors.Is(err, repository.ErrUserNotFound) {
		return fmt.Errorf("failed to check email: %w", err)
	}
	if existingUser != nil {
		return fmt.Errorf("email already in use: %w", ErrEmailAlreadyExists)
	}

	err = s.tokenRepository.DeleteByUserAndType(user.ID, model.TokenTypeEmailChange)
	if err != nil {
		slog.Warn("failed to delete old email change tokens", "error", err, "user_id", user.ID)
	}

	user.PendingEmail = &newEmail
	err = s.userRepository.Update(user)
	if err != nil {
		return fmt.Errorf("failed to save pending email: %w", err)
	}

	verificationToken, err := s.GenerateToken()
	if err != nil {
		return err
	}

	token := &model.Token{
		UserID:    user.ID,
		Type:      model.TokenTypeEmailChange,
		Token:     verificationToken,
		ExpiresAt: time.Now().Add(s.tokenEmailChangeExpiry),
	}
	err = s.tokenRepository.Create(token)
	if err != nil {
		return err
	}

	profile, err := s.profileRepository.ByUserID(user.ID)
	name := "User"
	if err == nil {
		name = profile.Name
	}

	err = s.emailService.SendEmailChangeVerification(newEmail, verificationToken, name)
	if err != nil {
		return fmt.Errorf("failed to send verification email: %w", err)
	}

	err = s.emailService.SendEmailChangeNotification(user.Email, newEmail, name)
	if err != nil {
		// Log error but don't fail the request
		slog.Warn("failed to send email change notification", "error", err, "user_id", user.ID)
	}

	return nil
}

// VerifyEmailChange completes the email change after verification
func (s *AuthService) VerifyEmailChange(token string) (*model.User, error) {
	// ConsumeToken atomically marks token as used (prevents race conditions)
	tokenModel, err := s.tokenRepository.ConsumeToken(token)
	if err != nil {
		return nil, errors.New("invalid or expired verification link")
	}

	if tokenModel.Type != model.TokenTypeEmailChange {
		return nil, errors.New("invalid token type")
	}

	user, err := s.userRepository.ByID(tokenModel.UserID)
	if err != nil {
		return nil, err
	}

	// Check if pending email exists
	if user.PendingEmail == nil || *user.PendingEmail == "" {
		return nil, errors.New("no pending email change found")
	}

	// Move pending email to email
	user.Email = *user.PendingEmail
	user.PendingEmail = nil

	err = s.userRepository.Update(user)
	if err != nil {
		return nil, fmt.Errorf("failed to update email: %w", err)
	}

	return user, nil
}

func (s *AuthService) SetPassword(userID, newPassword string) error {
	user, err := s.userRepository.ByID(userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	if user.HasPassword() {
		return errors.New("password already set, use change password instead")
	}

	err = s.ValidatePassword(newPassword)
	if err != nil {
		return err
	}

	hashedPassword, err := s.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	user.PasswordHash = &hashedPassword
	err = s.userRepository.Update(user)
	if err != nil {
		return fmt.Errorf("failed to set password: %w", err)
	}

	slog.Info("password set for passwordless account", "user_id", userID)
	return nil
}

func (s *AuthService) RemovePassword(userID string) error {
	user, err := s.userRepository.ByID(userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	if !user.HasPassword() {
		return errors.New("account is already passwordless")
	}

	user.PasswordHash = nil
	err = s.userRepository.Update(user)
	if err != nil {
		return fmt.Errorf("failed to remove password: %w", err)
	}

	slog.Info("password removed, account is now passwordless", "user_id", userID)
	return nil
}

// SendMagicLink generates a magic link and sends it to the user's email
// SendMagicLink handles the combined login/signup flow
// If user exists → sends magic link for login
// If user is new → creates account and sends magic link for verification + first login
func (s *AuthService) SendMagicLink(email string) error {
	email = strings.TrimSpace(strings.ToLower(email))

	// Validate email
	err := validation.ValidateEmail(email)
	if err != nil {
		return ErrInvalidEmail
	}

	// Check if user exists
	user, err := s.userRepository.ByEmail(email)
	if err != nil {
		// User doesn't exist - create new passwordless account
		now := time.Now()
		userID := uuid.New().String()

		user = &model.User{
			ID:        userID,
			Email:     email,
			CreatedAt: now,
			// password_hash is NULL for passwordless accounts
		}

		err = s.userRepository.Create(user)
		if err != nil {
			return fmt.Errorf("failed to create user: %w", err)
		}

		// Create empty profile (name will be set during onboarding)
		profile := &model.Profile{
			ID:        uuid.New().String(),
			UserID:    userID,
			Name:      "", // Will be filled in onboarding
			CreatedAt: now,
		}

		err = s.profileRepository.Create(profile)
		if err != nil {
			return fmt.Errorf("failed to create profile: %w", err)
		}

		// Create free subscription for new user
		err = s.subscriptionService.CreateFreeSubscription(userID)
		if err != nil {
			slog.Warn("failed to create free subscription", "error", err, "user_id", userID)
			// Don't fail user creation
		}

		slog.Info("new passwordless user created", "email", email, "user_id", userID)
	}

	// Delete any existing magic link tokens for this user
	err = s.tokenRepository.DeleteByUserAndType(user.ID, model.TokenTypeMagicLink)
	if err != nil {
		slog.Warn("failed to delete old magic link tokens", "error", err, "user_id", user.ID)
	}

	// Generate magic link token
	magicToken, err := s.GenerateToken()
	if err != nil {
		return fmt.Errorf("failed to generate token: %w", err)
	}

	// Save token to database
	token := &model.Token{
		UserID:    user.ID,
		Type:      model.TokenTypeMagicLink,
		Token:     magicToken,
		ExpiresAt: time.Now().Add(s.tokenMagicLinkExpiry),
	}
	err = s.tokenRepository.Create(token)
	if err != nil {
		return fmt.Errorf("failed to create token: %w", err)
	}

	// Get user's profile for name
	profile, err := s.profileRepository.ByUserID(user.ID)
	name := ""
	if err == nil && profile != nil {
		name = profile.Name
	}

	// Send magic link email
	err = s.emailService.SendMagicLinkEmail(user.Email, magicToken, name)
	if err != nil {
		slog.Error("failed to send magic link email", "error", err, "email", user.Email)
		return fmt.Errorf("failed to send email: %w", err)
	}

	slog.Info("magic link sent", "email", user.Email)
	return nil
}

// SendForgotPasswordLink sends a forgot password link that will remove the password and log the user in
// Reuses magic link infrastructure but sends different email
func (s *AuthService) SendForgotPasswordLink(email string) error {
	email = strings.TrimSpace(strings.ToLower(email))

	// Validate email
	err := validation.ValidateEmail(email)
	if err != nil {
		return ErrInvalidEmail
	}

	// Check if user exists
	user, err := s.userRepository.ByEmail(email)
	if err != nil {
		// User doesn't exist - silently fail to prevent email enumeration
		slog.Info("forgot password requested for non-existent email", "email", email)
		return nil // Always return success to prevent enumeration
	}

	// Only works if user has a password
	if !user.HasPassword() {
		slog.Info("forgot password requested for passwordless account", "email", email)
		return nil // Return success to prevent enumeration
	}

	// Delete any existing magic link tokens for this user
	err = s.tokenRepository.DeleteByUserAndType(user.ID, model.TokenTypeMagicLink)
	if err != nil {
		slog.Warn("failed to delete old tokens", "error", err, "user_id", user.ID)
	}

	// Generate magic link token (reuse same token type)
	magicToken, err := s.GenerateToken()
	if err != nil {
		return fmt.Errorf("failed to generate token: %w", err)
	}

	// Save token to database
	token := &model.Token{
		UserID:    user.ID,
		Type:      model.TokenTypeMagicLink,
		Token:     magicToken,
		ExpiresAt: time.Now().Add(s.tokenMagicLinkExpiry),
	}
	err = s.tokenRepository.Create(token)
	if err != nil {
		return fmt.Errorf("failed to create token: %w", err)
	}

	// Get user's profile for name
	profile, err := s.profileRepository.ByUserID(user.ID)
	name := ""
	if err == nil && profile != nil {
		name = profile.Name
	}

	// Send forgot password email (different template)
	err = s.emailService.SendForgotPasswordEmail(user.Email, magicToken, name)
	if err != nil {
		slog.Error("failed to send forgot password email", "error", err, "email", user.Email)
		return fmt.Errorf("failed to send email: %w", err)
	}

	slog.Info("forgot password link sent", "email", user.Email)
	return nil
}

// VerifyMagicLink verifies the magic link token and returns the authenticated user
func (s *AuthService) VerifyMagicLink(token string) (*model.User, error) {
	// ConsumeToken atomically marks token as used (prevents race conditions)
	tokenModel, err := s.tokenRepository.ConsumeToken(token)
	if err != nil {
		return nil, fmt.Errorf("invalid or expired magic link")
	}

	// Verify token type
	if tokenModel.Type != model.TokenTypeMagicLink {
		return nil, fmt.Errorf("invalid token type")
	}

	// Get user
	user, err := s.userRepository.ByID(tokenModel.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	// Auto-verify email if not already verified (important for passwordless registration)
	if user.EmailVerifiedAt == nil {
		now := time.Now()
		user.EmailVerifiedAt = &now
		err = s.userRepository.Update(user)
		if err != nil {
			slog.Warn("failed to verify email", "error", err, "user_id", user.ID)
		}
		// Note: Welcome email is sent after onboarding (when we have the user's name)
	}

	slog.Info("user authenticated via magic link", "user_id", user.ID, "email", user.Email)
	return user, nil
}

// NeedsOnboarding checks if user needs to complete onboarding (name not set)
func (s *AuthService) NeedsOnboarding(userID string) (bool, error) {
	profile, err := s.profileRepository.ByUserID(userID)
	if err != nil {
		return false, fmt.Errorf("failed to get profile: %w", err)
	}

	// If name is empty, user needs onboarding
	return profile.Name == "", nil
}

// CompleteOnboarding sets the user's name during onboarding
func (s *AuthService) CompleteOnboarding(userID, name string) error {
	name = strings.TrimSpace(name)

	// Validate name
	err := validation.ValidateName(name)
	if err != nil {
		return err
	}

	err = s.profileRepository.UpdateName(userID, name)
	if err != nil {
		return fmt.Errorf("failed to update profile: %w", err)
	}

	// Send welcome email now that we have the name
	user, err := s.userRepository.ByID(userID)
	if err == nil {
		err = s.emailService.SendWelcomeEmail(user.Email, name)
		if err != nil {
			slog.Warn("failed to send welcome email", "error", err, "email", user.Email)
		}
	}

	slog.Info("onboarding completed", "user_id", userID, "name", name)
	return nil
}

// AuthenticateOAuth handles OAuth authentication (Google, GitHub, etc.)
// It creates a new user if one doesn't exist, or returns existing user
func (s *AuthService) AuthenticateOAuth(email, provider string) (*model.User, error) {
	email = strings.TrimSpace(strings.ToLower(email))

	// Validate email
	err := validation.ValidateEmail(email)
	if err != nil {
		return nil, ErrInvalidEmail
	}

	// Check if user exists
	user, err := s.userRepository.ByEmail(email)
	if err != nil {
		if !errors.Is(err, repository.ErrUserNotFound) {
			return nil, fmt.Errorf("failed to lookup user: %w", err)
		}

		// User doesn't exist - create new account via OAuth
		now := time.Now()
		userID := uuid.New().String()

		user = &model.User{
			ID:              userID,
			Email:           email,
			EmailVerifiedAt: &now, // OAuth provider has verified email
			CreatedAt:       now,
			// password_hash is NULL for OAuth accounts
		}

		err = s.userRepository.Create(user)
		if err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}

		// Create empty profile (name will be set during onboarding)
		profile := &model.Profile{
			ID:        uuid.New().String(),
			UserID:    userID,
			Name:      "", // Will be filled in onboarding
			CreatedAt: now,
		}

		err = s.profileRepository.Create(profile)
		if err != nil {
			return nil, fmt.Errorf("failed to create profile: %w", err)
		}

		// Create free subscription for new user
		err = s.subscriptionService.CreateFreeSubscription(userID)
		if err != nil {
			slog.Warn("failed to create free subscription", "error", err, "user_id", userID)
			// Don't fail user creation
		}

		slog.Info("new OAuth user created", "email", email, "user_id", userID, "provider", provider)
		return user, nil
	}

	// User exists - ensure email is verified (OAuth provider has verified it)
	if user.EmailVerifiedAt == nil {
		now := time.Now()
		user.EmailVerifiedAt = &now
		err = s.userRepository.Update(user)
		if err != nil {
			slog.Warn("failed to mark email as verified", "error", err, "user_id", user.ID)
			// Don't fail login
		}
	}

	slog.Info("user authenticated via OAuth", "user_id", user.ID, "email", user.Email, "provider", provider)
	return user, nil
}
