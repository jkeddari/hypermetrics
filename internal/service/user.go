package service

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jkeddari/hypermetrics/internal/model"
	"github.com/jkeddari/hypermetrics/internal/repository"
	"github.com/jkeddari/hypermetrics/internal/validation"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCurrentPassword = errors.New("current password is incorrect")
	ErrActiveSubscription     = errors.New("cannot delete account with active subscription")
)

type UserService struct {
	userRepository      repository.UserRepository
	profileRepository   repository.ProfileRepository
	fileService         *FileService
	emailService        *EmailService
	subscriptionService *SubscriptionService
}

func NewUserService(
	userRepository repository.UserRepository,
	profileRepository repository.ProfileRepository,
	fileService *FileService,
	emailService *EmailService,
	subscriptionService *SubscriptionService,
) *UserService {
	return &UserService{
		userRepository:      userRepository,
		profileRepository:   profileRepository,
		fileService:         fileService,
		emailService:        emailService,
		subscriptionService: subscriptionService,
	}
}

func (s *UserService) ByID(id string) (*model.User, error) {
	user, err := s.userRepository.ByID(id)
	if err != nil {
		return nil, err
	}

	// Populate avatar URL
	avatar, err := s.fileService.Avatar("user", id)
	if err == nil {
		user.AvatarURL = s.fileService.URL(avatar)
	}

	return user, nil
}

func (s *UserService) UpdatePassword(userID, currentPassword, newPassword string) error {
	user, err := s.userRepository.ByID(userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	if !user.HasPassword() {
		return fmt.Errorf("passwordless accounts cannot update password. Please set a password first")
	}

	err = bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(currentPassword))
	if err != nil {
		return ErrInvalidCurrentPassword
	}

	// Validate new password
	err = validation.ValidatePassword(newPassword)
	if err != nil {
		return err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	hashStr := string(hashedPassword)
	user.PasswordHash = &hashStr

	err = s.userRepository.Update(user)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}

func (s *UserService) DeleteAccount(userID string) error {
	// Check if user has an active paid subscription or current period is still running
	subscription, err := s.subscriptionService.Subscription(userID)
	if err != nil {
		return fmt.Errorf("failed to check subscription: %w", err)
	}

	// Block deletion if user has paid plan (not free) AND (subscription is active OR period hasn't ended yet)
	if subscription.PlanID != model.SubscriptionPlanFree &&
		(subscription.Status == model.SubscriptionStatusActive ||
			(subscription.CurrentPeriodEnd != nil && subscription.CurrentPeriodEnd.After(time.Now()))) {
		return ErrActiveSubscription
	}

	user, err := s.userRepository.ByID(userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	profile, err := s.profileRepository.ByUserID(userID)
	if err != nil {
		// Continue without profile name if not found
		slog.Warn("failed to get profile for deletion email", "user_id", userID, "error", err)
	}

	name := "User"
	if profile != nil {
		name = profile.Name
	}

	err = s.fileService.DeleteAllUserFilesFromStorage(userID)
	if err != nil {
		// Log warning but don't fail - orphaned files are better than failed deletion
		slog.Warn("failed to delete user files from storage", "user_id", userID, "error", err)
	}

	err = s.emailService.SendAccountDeletedEmail(user.Email, name)
	if err != nil {
		slog.Warn("failed to send account deleted email", "user_id", userID, "email", user.Email, "error", err)
	}

	// Delete user from database
	// Foreign key CASCADE will automatically delete:
	// - profiles (ON DELETE CASCADE)
	// - tokens (ON DELETE CASCADE)
	// - files (ON DELETE CASCADE) - DB records only, physical files already deleted above
	// - subscriptions (ON DELETE CASCADE)
	err = s.userRepository.Delete(userID)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}
