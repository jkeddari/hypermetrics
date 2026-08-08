package model

import "time"

type Subscription struct {
	UserID               string
	StripeCustomerID     string
	StripeSubscriptionID string
	PlanID               string
	Status               string
	TrialEnd             *time.Time
	TrialUsedAt          *time.Time
	CancelAtPeriodEnd    bool
	UpdatedAt            time.Time
}

func (s *Subscription) Entitled() bool {
	return s != nil && (s.Status == "active" || s.Status == "trialing" || s.Status == "past_due")
}

func (s *Subscription) RateLimit() int {
	if s != nil && s.PlanID == "pro" {
		return 600
	}
	return 60
}

func (s *Subscription) TrialEligible() bool {
	return s == nil || s.TrialUsedAt == nil
}
