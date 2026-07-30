package model

import "time"

type APIKey struct {
	ID         string     `db:"id"`
	UserID     string     `db:"user_id"`
	Name       string     `db:"name"`
	KeyPrefix  string     `db:"key_prefix"`
	PlanID     string     `db:"plan_id"`
	Active     bool       `db:"active"`
	LastUsedAt *time.Time `db:"last_used_at"`
	CreatedAt  time.Time  `db:"created_at"`
}
