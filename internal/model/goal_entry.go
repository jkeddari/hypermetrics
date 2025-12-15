package model

import (
	"time"
)

type GoalEntry struct {
	ID          string     `db:"id"`
	GoalID      string     `db:"goal_id"`
	Step        int        `db:"step"`
	Completed   bool       `db:"completed"`
	Note        string     `db:"note"`
	CompletedAt *time.Time `db:"completed_at"`
	CreatedAt   time.Time  `db:"created_at"`
}
