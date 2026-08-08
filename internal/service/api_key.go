package service

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jkeddari/hypermetrics/internal/model"
)

const MaxAPIKeysPerUser = 3

var (
	ErrInvalidAPIKey        = errors.New("invalid api key")
	ErrAPIKeyLimit          = errors.New("maximum number of API keys reached")
	ErrInvalidAPIKeyName    = errors.New("API key name must contain between 1 and 50 characters")
	ErrSubscriptionRequired = errors.New("an active subscription is required")
)

type APIKeyService struct {
	db         *sql.DB
	staticKeys map[string]struct{}
}

func NewAPIKeyService(db *sql.DB, apiKeys ...string) *APIKeyService {
	keyMap := make(map[string]struct{}, len(apiKeys))
	for _, apiKey := range apiKeys {
		apiKey = strings.TrimSpace(apiKey)
		if apiKey != "" {
			keyMap[apiKey] = struct{}{}
		}
	}
	return &APIKeyService{db: db, staticKeys: keyMap}
}

func (s *APIKeyService) ValidateAPIKey(rawKey string) (*model.APIKey, error) {
	rawKey = strings.TrimSpace(rawKey)
	if rawKey == "" {
		return nil, ErrInvalidAPIKey
	}
	if _, ok := s.staticKeys[rawKey]; ok {
		return &model.APIKey{ID: rawKey, Name: "static API key", KeyPrefix: keyPrefix(rawKey), PlanID: "early-access", Active: true}, nil
	}
	if s.db == nil {
		return nil, ErrInvalidAPIKey
	}

	var key model.APIKey
	err := s.db.QueryRow(`
		SELECT k.id, k.user_id, k.name, k.key_prefix, k.plan_id, k.active, k.last_used_at, k.created_at
		FROM api_keys k
		LEFT JOIN subscriptions s ON s.user_id = k.user_id
		WHERE k.key_hash = $1 AND k.active = TRUE
		  AND (k.plan_id = 'early-access' OR (s.status IN ('active', 'trialing', 'past_due') AND s.plan_id = k.plan_id))
	`, hashAPIKey(rawKey)).Scan(
		&key.ID, &key.UserID, &key.Name, &key.KeyPrefix, &key.PlanID,
		&key.Active, &key.LastUsedAt, &key.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrInvalidAPIKey
	}
	if err != nil {
		return nil, fmt.Errorf("validate API key: %w", err)
	}

	now := time.Now().UTC()
	_, _ = s.db.Exec(`UPDATE api_keys SET last_used_at = $1 WHERE id = $2`, now, key.ID)
	key.LastUsedAt = &now
	return &key, nil
}

func (s *APIKeyService) CreateAPIKey(userID, name string) (*model.APIKey, string, error) {
	name = strings.TrimSpace(name)
	if len(name) == 0 || len(name) > 50 {
		return nil, "", ErrInvalidAPIKeyName
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, "", fmt.Errorf("begin API key creation: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`SELECT pg_advisory_xact_lock(hashtext($1))`, userID); err != nil {
		return nil, "", fmt.Errorf("lock API key creation: %w", err)
	}
	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM api_keys WHERE user_id = $1 AND active = TRUE`, userID).Scan(&count); err != nil {
		return nil, "", fmt.Errorf("count API keys: %w", err)
	}
	if count >= MaxAPIKeysPerUser {
		return nil, "", ErrAPIKeyLimit
	}
	var planID string
	if err := tx.QueryRow(`SELECT plan_id FROM subscriptions WHERE user_id = $1 AND status IN ('active', 'trialing', 'past_due')`, userID).Scan(&planID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", ErrSubscriptionRequired
		}
		return nil, "", fmt.Errorf("load subscription plan: %w", err)
	}

	secret, err := randomToken(32)
	if err != nil {
		return nil, "", err
	}
	rawKey := "hm_live_" + secret
	idToken, err := randomToken(12)
	if err != nil {
		return nil, "", err
	}
	key := &model.APIKey{
		ID:        "key_" + idToken,
		UserID:    userID,
		Name:      name,
		KeyPrefix: keyPrefix(rawKey),
		PlanID:    planID,
		Active:    true,
		CreatedAt: time.Now().UTC(),
	}
	_, err = tx.Exec(`
		INSERT INTO api_keys (id, user_id, name, key_prefix, key_hash, plan_id, active, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, TRUE, $7)
	`, key.ID, key.UserID, key.Name, key.KeyPrefix, hashAPIKey(rawKey), key.PlanID, key.CreatedAt)
	if err != nil {
		return nil, "", fmt.Errorf("create API key: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, "", fmt.Errorf("commit API key creation: %w", err)
	}
	return key, rawKey, nil
}

func (s *APIKeyService) ListAPIKeys(userID string) ([]model.APIKey, error) {
	rows, err := s.db.Query(`
		SELECT id, user_id, name, key_prefix, plan_id, active, last_used_at, created_at
		FROM api_keys
		WHERE user_id = $1 AND active = TRUE
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list API keys: %w", err)
	}
	defer rows.Close()

	keys := make([]model.APIKey, 0, MaxAPIKeysPerUser)
	for rows.Next() {
		var key model.APIKey
		if err := rows.Scan(&key.ID, &key.UserID, &key.Name, &key.KeyPrefix, &key.PlanID, &key.Active, &key.LastUsedAt, &key.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan API key: %w", err)
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

func (s *APIKeyService) RevokeAPIKey(userID, keyID string) error {
	result, err := s.db.Exec(`UPDATE api_keys SET active = FALSE WHERE id = $1 AND user_id = $2 AND active = TRUE`, keyID, userID)
	if err != nil {
		return fmt.Errorf("revoke API key: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrInvalidAPIKey
	}
	return nil
}

func hashAPIKey(apiKey string) string {
	hash := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(hash[:])
}

func keyPrefix(apiKey string) string {
	if len(apiKey) <= 16 {
		return apiKey
	}
	return apiKey[:16]
}

func randomToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate secure token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
