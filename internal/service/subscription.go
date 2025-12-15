package service

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jkeddari/hypermetrics/internal/model"
	"github.com/jkeddari/hypermetrics/internal/repository"
)

type SubscriptionService struct {
	repo repository.SubscriptionRepository
}

func NewSubscriptionService(repo repository.SubscriptionRepository) *SubscriptionService {
	return &SubscriptionService{repo: repo}
}

func (s *SubscriptionService) CreateFreeSubscription(userID string) error {
	now := time.Now()
	subscription := &model.Subscription{
		ID:        uuid.New().String(),
		UserID:    userID,
		PlanID:    model.SubscriptionPlanFree,
		Status:    model.SubscriptionStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}

	err := s.repo.Create(subscription)
	if err != nil {
		return fmt.Errorf("failed to create free subscription: %w", err)
	}

	return nil
}

func (s *SubscriptionService) Subscription(userID string) (*model.Subscription, error) {
	sub, err := s.repo.ByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}

	return sub, nil
}

func (s *SubscriptionService) ByProviderSubscriptionID(providerSubID string) (*model.Subscription, error) {
	sub, err := s.repo.ByProviderSubscriptionID(providerSubID)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscription by provider ID: %w", err)
	}

	return sub, nil
}

func (s *SubscriptionService) ByProviderCustomerID(providerCustomerID string) (*model.Subscription, error) {
	sub, err := s.repo.ByProviderCustomerID(providerCustomerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscription by provider customer ID: %w", err)
	}

	return sub, nil
}

func (s *SubscriptionService) UpdateSubscription(sub *model.Subscription) error {
	sub.UpdatedAt = time.Now()

	err := s.repo.Update(sub)
	if err != nil {
		return fmt.Errorf("failed to update subscription: %w", err)
	}

	return nil
}

func (s *SubscriptionService) DowngradeToFree(sub *model.Subscription) error {
	sub.PlanID = model.SubscriptionPlanFree
	sub.Status = model.SubscriptionStatusActive
	sub.ProviderSubscriptionID = nil
	sub.CurrentPeriodEnd = nil
	sub.Amount = nil
	sub.Currency = ""
	sub.Interval = nil

	return s.UpdateSubscription(sub)
}
