package model

import (
	"time"
)

const (
	GoalStatusActive    = "active"
	GoalStatusCompleted = "completed"
)

type Goal struct {
	ID          string    `db:"id"`
	UserID      string    `db:"user_id"`
	Title       string    `db:"title"`
	Description string    `db:"description"`
	Status      string    `db:"status"`
	CurrentStep int       `db:"current_step"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}
