package model

import (
	"fmt"
	"time"
)

type Subscription struct {
	ID                     string     `db:"id"`
	UserID                 string     `db:"user_id"`
	PlanID                 string     `db:"plan_id"`
	Status                 string     `db:"status"`
	Provider               string     `db:"provider"`
	ProviderCustomerID     *string    `db:"provider_customer_id"`
	ProviderSubscriptionID *string    `db:"provider_subscription_id"`
	CurrentPeriodEnd       *time.Time `db:"current_period_end"`
	Amount                 *int       `db:"amount"`
	Currency               string     `db:"currency"`
	Interval               *string    `db:"interval"`
	CreatedAt              time.Time  `db:"created_at"`
	UpdatedAt              time.Time  `db:"updated_at"`
}

const (
	SubscriptionStatusActive    = "active"
	SubscriptionStatusCancelled = "cancelled"
)

const (
	ProviderPolar  = "polar"
	ProviderStripe = "stripe"
)

const (
	SubscriptionPlanFree  = "free"
	SubscriptionPlanBasic = "basic"
	SubscriptionPlanPro   = "pro"
)

const (
	SubscriptionIntervalMonthly = "monthly"
	SubscriptionIntervalYearly  = "yearly"
)

const (
	FeatureExport          = "export"
	FeaturePrioritySupport = "priority_support"
)

func (s *Subscription) IsActive() bool {
	return s.Status == SubscriptionStatusActive
}

func (s *Subscription) IsPaid() bool {
	return s.PlanID != SubscriptionPlanFree && s.IsActive()
}

func (s *Subscription) FormatPrice() string {
	if s.Amount == nil || *s.Amount == 0 {
		return ""
	}

	currencySymbols := map[string]string{
		"usd": "$",
		"eur": "€",
		"gbp": "£",
	}

	amount := float64(*s.Amount) / 100.0
	symbol := currencySymbols[s.Currency]
	if symbol == "" {
		symbol = "$"
	}

	interval := "month"
	if s.Interval != nil && *s.Interval == SubscriptionIntervalYearly {
		interval = "year"
	}

	return fmt.Sprintf("%s%.0f/%s", symbol, amount, interval)
}

// GetGoalLimit returns the maximum number of goals allowed for this plan
// Returns -1 for unlimited
func (s *Subscription) GetGoalLimit() int {
	if !s.IsActive() {
		return 3 // Free tier default
	}

	switch s.PlanID {
	case SubscriptionPlanFree:
		return 3
	case SubscriptionPlanBasic:
		return 25
	case SubscriptionPlanPro:
		return -1 // unlimited
	default:
		return 3
	}
}

// HasFeature checks if the subscription has access to a specific feature
func (s *Subscription) HasFeature(feature string) bool {
	if !s.IsActive() {
		return false
	}

	// Feature mapping by plan
	features := map[string][]string{
		SubscriptionPlanFree: {},
		SubscriptionPlanBasic: {
			FeatureExport,
		},
		SubscriptionPlanPro: {
			FeatureExport,
			FeaturePrioritySupport,
		},
	}

	planFeatures, exists := features[s.PlanID]
	if !exists {
		return false
	}

	for _, f := range planFeatures {
		if f == feature {
			return true
		}
	}

	return false
}
