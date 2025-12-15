package repository

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/jkeddari/hypermetrics/internal/model"
)

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrDuplicateEmail  = errors.New("email already exists")
	ErrProfileNotFound = errors.New("profile not found")
)

type UserRepository interface {
	Create(user *model.User) error
	ByID(id string) (*model.User, error)
	ByEmail(email string) (*model.User, error)
	Update(user *model.User) error
	Delete(id string) error
}

type userRepository struct {
	db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(user *model.User) error {
	query := `INSERT INTO users (id, email, password_hash, email_verified_at, created_at) VALUES ($1, $2, $3, $4, $5)`

	_, err := r.db.Exec(query, user.ID, user.Email, user.PasswordHash, user.EmailVerifiedAt, user.CreatedAt)
	if err != nil {
		// Check for unique constraint violation (works for both SQLite and PostgreSQL)
		errStr := err.Error()
		if strings.Contains(errStr, "UNIQUE constraint failed") || strings.Contains(errStr, "duplicate key value") {
			return ErrDuplicateEmail
		}
		return err
	}

	return nil
}

func (r *userRepository) ByID(id string) (*model.User, error) {
	user := &model.User{}
	query := `SELECT * FROM users WHERE id = $1`

	err := r.db.Get(user, query, id)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}

	return user, err
}

func (r *userRepository) ByEmail(email string) (*model.User, error) {
	user := &model.User{}
	query := `SELECT * FROM users WHERE email = $1`

	err := r.db.Get(user, query, email)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}

	return user, err
}

func (r *userRepository) Update(user *model.User) error {
	query := `UPDATE users SET email = $1, password_hash = $2, pending_email = $3, email_verified_at = $4 WHERE id = $5`

	_, err := r.db.Exec(query, user.Email, user.PasswordHash, user.PendingEmail, user.EmailVerifiedAt, user.ID)
	return err
}

func (r *userRepository) Delete(id string) error {
	query := `DELETE FROM users WHERE id = $1`

	result, err := r.db.Exec(query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return ErrUserNotFound
	}

	return nil
}
