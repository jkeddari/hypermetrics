package repository

import (
	"database/sql"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/jkeddari/hypermetrics/internal/model"
)

const (
	GoalSortRecent   = "recent"
	GoalSortProgress = "progress"
	GoalSortTitle    = "title"
)

var (
	ErrGoalNotFound = errors.New("goal not found")
)

type GoalRepository interface {
	Create(goal *model.Goal) error
	ByID(userID, goalID string) (*model.Goal, error)
	Goals(userID, sortBy string) ([]*model.Goal, error)
	CountUserGoals(userID string) (int, error)
	Update(goal *model.Goal) error
	Delete(userID, goalID string) error
}

type goalRepository struct {
	db *sqlx.DB
}

func NewGoalRepository(db *sqlx.DB) GoalRepository {
	return &goalRepository{db: db}
}

func (r *goalRepository) Create(goal *model.Goal) error {
	query := `INSERT INTO goals (id, user_id, title, description, status, current_step, created_at, updated_at)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := r.db.Exec(query,
		goal.ID,
		goal.UserID,
		goal.Title,
		goal.Description,
		goal.Status,
		goal.CurrentStep,
		goal.CreatedAt,
		goal.UpdatedAt,
	)

	return err
}

func (r *goalRepository) ByID(userID, goalID string) (*model.Goal, error) {
	goal := &model.Goal{}
	query := `SELECT * FROM goals WHERE id = $1 AND user_id = $2`

	err := r.db.Get(goal, query, goalID, userID)
	if err == sql.ErrNoRows {
		return nil, ErrGoalNotFound
	}

	return goal, err
}

func (r *goalRepository) Goals(userID, sortBy string) ([]*model.Goal, error) {
	var goals []*model.Goal

	// Validate and build ORDER BY clause
	var orderBy string
	switch sortBy {
	case GoalSortProgress:
		orderBy = "ORDER BY current_step DESC, updated_at DESC"
	case GoalSortTitle:
		orderBy = "ORDER BY LOWER(title) ASC"
	default: // GoalSortRecent or empty
		orderBy = "ORDER BY updated_at DESC"
	}

	query := `SELECT * FROM goals WHERE user_id = $1 ` + orderBy

	err := r.db.Select(&goals, query, userID)
	if err != nil {
		return nil, err
	}

	return goals, nil
}

func (r *goalRepository) CountUserGoals(userID string) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM goals WHERE user_id = $1 AND status = $2`
	err := r.db.QueryRow(query, userID, model.GoalStatusActive).Scan(&count)
	return count, err
}

func (r *goalRepository) Update(goal *model.Goal) error {
	query := `UPDATE goals
	          SET title = $1, description = $2, status = $3, current_step = $4, updated_at = $5
	          WHERE id = $6 AND user_id = $7`

	result, err := r.db.Exec(query,
		goal.Title,
		goal.Description,
		goal.Status,
		goal.CurrentStep,
		time.Now(),
		goal.ID,
		goal.UserID,
	)

	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return ErrGoalNotFound
	}

	return nil
}

func (r *goalRepository) Delete(userID, goalID string) error {
	query := `DELETE FROM goals WHERE id = $1 AND user_id = $2`
	result, err := r.db.Exec(query, goalID, userID)

	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return ErrGoalNotFound
	}

	return nil
}
