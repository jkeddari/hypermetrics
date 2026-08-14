package app

import (
	"database/sql"
	"fmt"

	"github.com/jkeddari/hypermetrics/internal/config"
	"github.com/jkeddari/hypermetrics/internal/corebus"
	"github.com/jkeddari/hypermetrics/internal/db"
	"github.com/jkeddari/hypermetrics/internal/hypercore"
	"github.com/jkeddari/hypermetrics/internal/service"
	"github.com/nats-io/nats.go"
)

type APIApp struct {
	Cfg            *config.Config
	DB             *sql.DB
	APIKeyService  *service.APIKeyService
	HypercoreStore *hypercore.Store
	CoreClient     *corebus.Client
	NATS           *nats.Conn
}

type WebApp struct {
	Cfg            *config.Config
	DB             *sql.DB
	AuthService    *service.AuthService
	APIKeyService  *service.APIKeyService
	BillingService *service.BillingService
}

func NewAPI(cfg *config.Config) (*APIApp, error) {
	database, err := db.InitPostgres(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	if err := db.RunMigrations(database); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	store := hypercore.OpenStore(database, hypercore.PriorityConfig{
		WhaleThresholdUSD:    cfg.HypercoreWhaleThresholdUSD,
		RejectedCandidateTTL: cfg.HypercoreRejectedCandidateTTL,
	})
	natsConn, err := corebus.Connect(cfg.NATSURL, "hypermetrics-api")
	if err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	return &APIApp{
		Cfg:            cfg,
		DB:             database,
		APIKeyService:  service.NewAPIKeyService(database, cfg.APIKeys...),
		HypercoreStore: store,
		CoreClient:     corebus.NewClient(natsConn),
		NATS:           natsConn,
	}, nil
}

func NewWeb(cfg *config.Config) (*WebApp, error) {
	if cfg.IsProduction() && len(cfg.SessionSecret) < 32 {
		return nil, fmt.Errorf("SESSION_SECRET must contain at least 32 characters in production")
	}
	if cfg.IsProduction() && (cfg.StripeSecretKey == "" || cfg.StripeWebhookSecret == "" || cfg.StripeBuilderPriceID == "" || cfg.StripeProPriceID == "") {
		return nil, fmt.Errorf("Stripe billing configuration is required in production")
	}
	if cfg.IsProduction() && (cfg.ResendAPIKey == "" || cfg.ResendFromEmail == "") {
		return nil, fmt.Errorf("Resend email configuration is required in production")
	}
	database, err := db.InitPostgres(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}
	if err := db.RunMigrations(database); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}
	webApp := &WebApp{
		Cfg: cfg,
		DB:  database,
		AuthService: service.NewAuthService(
			database, cfg.SessionSecret, cfg.IsProduction(), cfg.SessionExpiry,
			cfg.AppURL, cfg.ResendAPIKey, cfg.ResendFromEmail,
		),
		APIKeyService: service.NewAPIKeyService(database, cfg.APIKeys...),
	}
	if cfg.StripeSecretKey != "" {
		webApp.BillingService = service.NewBillingService(database, service.BillingConfig{
			AppURL: cfg.AppURL, SecretKey: cfg.StripeSecretKey, WebhookSecret: cfg.StripeWebhookSecret,
			BuilderPriceID: cfg.StripeBuilderPriceID, ProPriceID: cfg.StripeProPriceID,
			PortalConfigurationID: cfg.StripePortalConfigurationID,
			ResendAPIKey:          cfg.ResendAPIKey, ResendFromEmail: cfg.ResendFromEmail,
		})
	}
	return webApp, nil
}

func (a *WebApp) Close() error {
	if a != nil && a.DB != nil {
		return a.DB.Close()
	}
	return nil
}

func (a *APIApp) Close() error {
	if a.NATS != nil {
		a.NATS.Close()
	}
	if a.DB != nil {
		return a.DB.Close()
	}
	return nil
}
