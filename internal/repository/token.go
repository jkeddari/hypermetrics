package repository

import (
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/jkeddari/hypermetrics/internal/model"
)

var (
	ErrTokenNotFound = errors.New("token not found")
	ErrTokenExpired  = errors.New("token has expired")
	ErrTokenUsed     = errors.New("token has already been used")
)

type TokenRepository interface {
	Create(token *model.Token) error
	ConsumeToken(token string) (*model.Token, error)
	DeleteByUserAndType(userID, tokenType string) error
}

type tokenRepository struct {
	db *sqlx.DB
}

func NewTokenRepository(db *sqlx.DB) TokenRepository {
	return &tokenRepository{db: db}
}

func (r *tokenRepository) Create(token *model.Token) error {
	if token.ID == "" {
		token.ID = uuid.New().String()
	}
	if token.CreatedAt.IsZero() {
		token.CreatedAt = time.Now()
	}

	query := `
		INSERT INTO tokens (id, user_id, type, token, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.Exec(query,
		token.ID,
		token.UserID,
		token.Type,
		token.Token,
		token.ExpiresAt,
		token.CreatedAt,
	)
	return err
}

// ConsumeToken atomically marks token as used and returns it
// This prevents race conditions where two requests could use the same token
// Only the first request will succeed, the second will get ErrTokenNotFound
func (r *tokenRepository) ConsumeToken(token string) (*model.Token, error) {
	var t model.Token
	now := time.Now()

	// Atomic UPDATE with RETURNING - only one request can succeed
	// This is a single database operation, preventing race conditions
	query := `
		UPDATE tokens
		SET used_at = $1
		WHERE token = $2
		AND used_at IS NULL
		AND expires_at > $3
		RETURNING *
	`

	err := r.db.Get(&t, query, now, token, now)
	if err == sql.ErrNoRows {
		return nil, ErrTokenNotFound
	}
	if err != nil {
		return nil, err
	}

	return &t, nil
}

func (r *tokenRepository) DeleteByUserAndType(userID, tokenType string) error {
	query := `DELETE FROM tokens WHERE user_id = $1 AND type = $2 AND used_at IS NULL`
	_, err := r.db.Exec(query, userID, tokenType)
	return err
}

// CleanupExpired removes used and expired tokens older than the given duration.
// This is an optional maintenance operation for production environments.
//
// By design, tokens are NOT automatically deleted to maintain an audit trail.
// Call this method periodically if you need to clean up old tokens (e.g., via cron job).
//
// Example:
//
//	// Remove tokens older than 90 days
//	err := tokenRepo.CleanupExpired(90 * 24 * time.Hour)
//
// For most applications, this cleanup is not necessary. Only use it if:
//   - You have very high token generation volume
//   - You need to comply with data retention policies
//   - Your database storage is constrained
func (r *tokenRepository) CleanupExpired(olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	query := `
		DELETE FROM tokens
		WHERE (used_at IS NOT NULL AND used_at < $1)
		   OR (expires_at < $1)
	`
	result, err := r.db.Exec(query, cutoff)
	if err != nil {
		return 0, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return rowsAffected, nil
}

