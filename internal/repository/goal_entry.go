package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/jkeddari/hypermetrics/internal/model"
)

var (
	ErrGoalEntryNotFound = errors.New("goal entry not found")
)

type GoalEntryRepository interface {
	CreateEntries(goalID string, count int) error
	Entries(goalID string) ([]*model.GoalEntry, error)
	Entry(goalID string, step int) (*model.GoalEntry, error)
	CompleteEntry(goalID string, step int) error
	UpdateEntry(goalID string, step int, note string, completedAt *time.Time) error
	UncompleteEntry(goalID string, step int) error
}

type goalEntryRepository struct {
	db *sqlx.DB
}

func NewGoalEntryRepository(db *sqlx.DB) GoalEntryRepository {
	return &goalEntryRepository{db: db}
}

// CreateEntries creates bulk entries for a goal (typically 100)
func (r *goalEntryRepository) CreateEntries(goalID string, count int) error {
	if count <= 0 || count > 100 {
		return fmt.Errorf("invalid entry count: %d", count)
	}

	// Begin transaction for bulk insert
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `INSERT INTO goal_entries (id, goal_id, step, completed, note, created_at)
	          VALUES ($1, $2, $3, $4, $5, $6)`

	now := time.Now()
	for i := 1; i <= count; i++ {
		_, err := tx.Exec(query, uuid.New().String(), goalID, i, false, "", now)
		if err != nil {
			return fmt.Errorf("failed to create entry %d: %w", i, err)
		}
	}

	return tx.Commit()
}

func (r *goalEntryRepository) Entries(goalID string) ([]*model.GoalEntry, error) {
	var entries []*model.GoalEntry
	query := `SELECT * FROM goal_entries WHERE goal_id = $1 ORDER BY step ASC`

	err := r.db.Select(&entries, query, goalID)
	if err != nil {
		return nil, err
	}

	return entries, nil
}

func (r *goalEntryRepository) Entry(goalID string, step int) (*model.GoalEntry, error) {
	entry := &model.GoalEntry{}
	query := `SELECT * FROM goal_entries WHERE goal_id = $1 AND step = $2`

	err := r.db.Get(entry, query, goalID, step)
	if err == sql.ErrNoRows {
		return nil, ErrGoalEntryNotFound
	}
	if err != nil {
		return nil, err
	}

	return entry, nil
}

func (r *goalEntryRepository) CompleteEntry(goalID string, step int) error {
	now := time.Now()
	query := `UPDATE goal_entries
	          SET completed = true, completed_at = $1
	          WHERE goal_id = $2 AND step = $3`

	result, err := r.db.Exec(query, now, goalID, step)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return ErrGoalEntryNotFound
	}

	return nil
}

func (r *goalEntryRepository) UpdateEntry(goalID string, step int, note string, completedAt *time.Time) error {
	query := `UPDATE goal_entries
	          SET note = $1, completed_at = $2
	          WHERE goal_id = $3 AND step = $4`

	result, err := r.db.Exec(query, note, completedAt, goalID, step)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return ErrGoalEntryNotFound
	}

	return nil
}

func (r *goalEntryRepository) UncompleteEntry(goalID string, step int) error {
	query := `UPDATE goal_entries
	          SET completed = false, note = '', completed_at = NULL
	          WHERE goal_id = $1 AND step = $2`

	result, err := r.db.Exec(query, goalID, step)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return ErrGoalEntryNotFound
	}

	return nil
}
