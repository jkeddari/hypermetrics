package app

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jkeddari/hypermetrics/internal/config"
	"github.com/jkeddari/hypermetrics/internal/db"
	"github.com/jkeddari/hypermetrics/internal/model"
	"github.com/jkeddari/hypermetrics/internal/repository"
	"github.com/jkeddari/hypermetrics/internal/service"
	"github.com/jkeddari/hypermetrics/internal/service/payment"
	"github.com/jkeddari/hypermetrics/internal/storage"
	"github.com/jmoiron/sqlx"
)

// maskPassword masks the password in a connection string for logging
func maskPassword(connStr string) string {
	if strings.Contains(connStr, "password=") {
		parts := strings.Split(connStr, " ")
		for i, part := range parts {
			if strings.HasPrefix(part, "password=") {
				parts[i] = "password=***"
			}
		}
		return strings.Join(parts, " ")
	}
	if strings.Contains(connStr, "://") && strings.Contains(connStr, "@") {
		// Format: postgresql://user:password@host/db
		idx := strings.Index(connStr, "://")
		atIdx := strings.Index(connStr, "@")
		if idx > 0 && atIdx > idx {
			userIdx := strings.Index(connStr[idx+3:], ":")
			if userIdx > 0 {
				return connStr[:idx+3+userIdx+1] + "***" + connStr[atIdx:]
			}
		}
	}
	return connStr
}

type App struct {
	Cfg                 *config.Config
	DB                  *sqlx.DB
	AuthService         *service.AuthService
	UserService         *service.UserService
	ProfileService      *service.ProfileService
	EmailService        *service.EmailService
	FileService         *service.FileService
	SubscriptionService *service.SubscriptionService
	PaymentService      payment.Provider
	DocsService         *service.DocsService
	LegalService        *service.LegalService
	LeaderboardService  *service.LeaderboardS3Service
}

func New(cfg *config.Config) (*App, error) {
	// Initialize database
	var database *sqlx.DB
	var err error

	if cfg.IsProduction() {
		database, err = db.InitProduction(cfg.DBSupabasePassword)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Cloud SQL: %v", err)
		}
	} else {
		database, err = db.InitDev()
		if err != nil {
			return nil, fmt.Errorf("failed to initialize database: %v", err)
		}
	}

	// Run database migrations
	driver := "pgx"
	if cfg.IsDevelopment() {
		driver = "sqlite"
	}
	err = db.RunMigrations(database.DB, driver)
	if err != nil {
		return nil, fmt.Errorf("failed to run migrations: %v", err)
	}

	// Repositories
	userRepository := repository.NewUserRepository(database)
	profileRepository := repository.NewProfileRepository(database)
	tokenRepository := repository.NewTokenRepository(database)
	fileRepository := repository.NewFileRepository(database)
	subscriptionRepository := repository.NewSubscriptionRepository(database)

	// Storage
	fileStorage, err := storage.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %v", err)
	}

	// Services
	emailService := service.NewEmailService(
		cfg.ResendAPIKey,
		cfg.EmailFrom,
		cfg.ResendAudienceID,
		cfg.AppURL,
		cfg.AppName,
		cfg.IsDevelopment(),
	)
	fileService := service.NewFileService(fileRepository, fileStorage)
	subscriptionService := service.NewSubscriptionService(subscriptionRepository)

	// Initialize payment provider based on config
	paymentProvider, err := payment.NewProvider(cfg, subscriptionService)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize payment provider: %v", err)
	}

	authService := service.NewAuthService(
		userRepository,
		profileRepository,
		tokenRepository,
		subscriptionService,
		emailService,
		cfg.JWTSecret,
		cfg.IsProduction(),
		cfg.JWTExpiry,
		cfg.TokenEmailVerifyExpiry,
		cfg.TokenPasswordResetExpiry,
		cfg.TokenEmailChangeExpiry,
		cfg.TokenMagicLinkExpiry,
	)
	userService := service.NewUserService(userRepository, profileRepository, fileService, emailService, subscriptionService)
	profileService := service.NewProfileService(profileRepository)
	docsService := service.NewDocsService(cfg.ContentPath)
	legalService := service.NewLegalService(cfg.ContentPath)

	// Initialize leaderboard service (S3-based)
	leaderboardService, err := service.NewLeaderboardS3Service(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize leaderboard service: %v", err)
	}

	// Create dev test user
	if cfg.IsDevelopment() {
		// Check if test user already exists
		existingUser, _ := userRepository.ByEmail("test@test.com")
		if existingUser == nil {
			passwd, err := authService.HashPassword("test")
			if err != nil {
				return nil, err
			}
			now := time.Now()
			testUserID := "dev-test-user-id"

			// Create user
			err = userRepository.Create(&model.User{
				ID:              testUserID,
				Email:           "test@test.com",
				PasswordHash:    &passwd,
				PendingEmail:    nil,
				EmailVerifiedAt: &now,
				CreatedAt:       now,
				AvatarURL:       "",
			})
			if err != nil {
				slog.Warn("failed to create dev test user", "error", err)
			} else {
				// Create profile
				err = profileRepository.Create(&model.Profile{
					ID:        "dev-test-profile-id",
					UserID:    testUserID,
					Name:      "Dev Test User",
					CreatedAt: now,
				})
				if err != nil {
					slog.Warn("failed to create dev test profile", "error", err)
				}

				// Create free subscription
				err = subscriptionService.CreateFreeSubscription(testUserID)
				if err != nil {
					slog.Warn("failed to create dev test subscription", "error", err)
				}

				slog.Info("dev test user created", "email", "test@test.com", "password", "test")
			}
		}
	}

	return &App{
		Cfg:                 cfg,
		DB:                  database,
		AuthService:         authService,
		UserService:         userService,
		ProfileService:      profileService,
		EmailService:        emailService,
		FileService:         fileService,
		SubscriptionService: subscriptionService,
		PaymentService:      paymentProvider,
		DocsService:         docsService,
		LegalService:        legalService,
		LeaderboardService:  leaderboardService,
	}, nil
}

func (a *App) Close() error {
	if a.DB != nil {
		return a.DB.Close()
	}
	return nil
}
