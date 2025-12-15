package payment

import (
	"fmt"
	"log/slog"

	"github.com/jkeddari/hypermetrics/internal/config"
	"github.com/jkeddari/hypermetrics/internal/model"
	"github.com/jkeddari/hypermetrics/internal/service"
)

// NewProvider creates a payment provider based on configuration
func NewProvider(cfg *config.Config, subscriptionService *service.SubscriptionService) (Provider, error) {
	provider := cfg.PaymentProvider

	slog.Info("initializing payment provider", "provider", provider)

	switch provider {
	case model.ProviderPolar:
		if cfg.PolarAPIKey == "" {
			return nil, fmt.Errorf("POLAR_API_KEY is required when using Polar provider")
		}
		return NewPolarProvider(cfg, subscriptionService), nil

	default:
		return nil, fmt.Errorf("unknown payment provider: %s (supported: polar, stripe)", provider)
	}
}
