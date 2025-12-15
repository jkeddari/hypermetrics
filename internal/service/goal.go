package service

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jkeddari/hypermetrics/internal/model"
	"github.com/jkeddari/hypermetrics/internal/repository"
)

var (
	ErrGoalLimitReached     = errors.New("free plan goal limit reached")
	ErrInvalidStep          = errors.New("invalid step: must complete previous steps first")
	ErrGoalAlreadyCompleted = errors.New("goal already completed")
)

type GoalService struct {
	repo                repository.GoalRepository
	entryRepo           repository.GoalEntryRepository
	fileRepo            repository.FileRepository
	subscriptionService *SubscriptionService
}

func NewGoalService(
	repo repository.GoalRepository,
	entryRepo repository.GoalEntryRepository,
	fileRepo repository.FileRepository,
	subscriptionService *SubscriptionService,
) *GoalService {
	return &GoalService{
		repo:                repo,
		entryRepo:           entryRepo,
		fileRepo:            fileRepo,
		subscriptionService: subscriptionService,
	}
}

func (s *GoalService) Create(userID, title, description string) (*model.Goal, error) {
	subscription, err := s.subscriptionService.Subscription(userID)
	if err != nil {
		return nil, err
	}

	// Check goal limit based on plan
	limit := subscription.GetGoalLimit()
	if limit != -1 { // -1 means unlimited
		count, err := s.repo.CountUserGoals(userID)
		if err != nil {
			return nil, err
		}

		if count >= limit {
			return nil, ErrGoalLimitReached
		}
	}

	now := time.Now()
	goal := &model.Goal{
		ID:          uuid.New().String(),
		UserID:      userID,
		Title:       title,
		Description: description,
		Status:      model.GoalStatusActive,
		CurrentStep: 0,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	err = s.repo.Create(goal)
	if err != nil {
		return nil, fmt.Errorf("failed to create goal: %w", err)
	}

	// Create 100 entries for the goal
	err = s.entryRepo.CreateEntries(goal.ID, 100)
	if err != nil {
		// Rollback: delete the goal if entries creation fails
		delErr := s.repo.Delete(userID, goal.ID)
		if delErr != nil {
			slog.Error("failed to delete goal during rollback", "error", delErr, "goalID", goal.ID)
		}
		return nil, fmt.Errorf("failed to create goal entries: %w", err)
	}

	return goal, nil
}

func (s *GoalService) ByID(userID, goalID string) (*model.Goal, error) {
	return s.repo.ByID(userID, goalID)
}

func (s *GoalService) Goals(userID, sortBy string) ([]*model.Goal, error) {
	return s.repo.Goals(userID, sortBy)
}

func (s *GoalService) GoalWithEntries(userID, goalID string) (*model.Goal, []*model.GoalEntry, error) {
	// Verify ownership
	goal, err := s.repo.ByID(userID, goalID)
	if err != nil {
		return nil, nil, err
	}

	entries, err := s.entryRepo.Entries(goalID)
	if err != nil {
		return nil, nil, err
	}

	return goal, entries, nil
}

func (s *GoalService) CountUserGoals(userID string) (int, error) {
	return s.repo.CountUserGoals(userID)
}

func (s *GoalService) Update(userID, goalID, title, description, status string) error {
	// Verify ownership
	goal, err := s.repo.ByID(userID, goalID)
	if err != nil {
		return err
	}

	goal.Title = title
	goal.Description = description
	goal.Status = status
	goal.UpdatedAt = time.Now()

	return s.repo.Update(goal)
}

func (s *GoalService) CompleteEntry(userID, goalID string, step int) error {
	// Verify ownership
	goal, err := s.repo.ByID(userID, goalID)
	if err != nil {
		return err
	}

	if goal.Status == model.GoalStatusCompleted {
		return ErrGoalAlreadyCompleted
	}

	if step != goal.CurrentStep+1 {
		return ErrInvalidStep
	}

	err = s.entryRepo.CompleteEntry(goalID, step)
	if err != nil {
		return err
	}

	goal.CurrentStep = step

	if step == 100 {
		goal.Status = model.GoalStatusCompleted
	}

	goal.UpdatedAt = time.Now()
	return s.repo.Update(goal)
}

func (s *GoalService) Delete(userID, goalID string) error {
	// Verify ownership
	_, err := s.repo.ByID(userID, goalID)
	if err != nil {
		return err
	}

	return s.repo.Delete(userID, goalID)
}

func (s *GoalService) EntryByGoalAndStep(goalID string, step int) (*model.GoalEntry, error) {
	return s.entryRepo.Entry(goalID, step)
}

func (s *GoalService) UpdateEntry(userID, goalID string, step int, note string, completedAt *time.Time) error {
	// Verify ownership
	_, err := s.repo.ByID(userID, goalID)
	if err != nil {
		return err
	}

	entry, err := s.entryRepo.Entry(goalID, step)
	if err != nil {
		return err
	}

	if !entry.Completed {
		return errors.New("cannot update incomplete entry")
	}

	return s.entryRepo.UpdateEntry(goalID, step, note, completedAt)
}

func (s *GoalService) UncompleteEntry(userID, goalID string, step int) error {
	// Verify ownership
	goal, err := s.repo.ByID(userID, goalID)
	if err != nil {
		return err
	}

	if step != goal.CurrentStep {
		return errors.New("can only uncomplete the last completed step")
	}

	err = s.entryRepo.UncompleteEntry(goalID, step)
	if err != nil {
		return err
	}

	goal.CurrentStep = step - 1

	if goal.Status == model.GoalStatusCompleted {
		goal.Status = model.GoalStatusActive
	}

	goal.UpdatedAt = time.Now()
	return s.repo.Update(goal)
}
