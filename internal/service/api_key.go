package service

import (
	"errors"
	"strings"

	"github.com/jkeddari/hypermetrics/internal/model"
)

var (
	ErrInvalidAPIKey                  = errors.New("invalid api key")
	ErrAPIKeyValidationNotImplemented = errors.New("api key validation not implemented")
)

type APIKeyService struct {
	apiKeys map[string]struct{}
}

func NewAPIKeyService(apiKeys ...string) *APIKeyService {
	keyMap := make(map[string]struct{}, len(apiKeys))
	for _, apiKey := range apiKeys {
		apiKey = strings.TrimSpace(apiKey)
		if apiKey != "" {
			keyMap[apiKey] = struct{}{}
		}
	}
	return &APIKeyService{
		apiKeys: keyMap,
	}
}

func (s *APIKeyService) ValidateAPIKey(apiKey string) (*model.APIKey, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, ErrInvalidAPIKey
	}
	if _, ok := s.apiKeys[apiKey]; !ok {
		return nil, ErrInvalidAPIKey
	}
	return &model.APIKey{
		ID:        apiKey,
		Name:      "static api key",
		KeyPrefix: keyPrefix(apiKey),
		Active:    true,
	}, nil
}

func keyPrefix(apiKey string) string {
	if len(apiKey) <= 8 {
		return apiKey
	}
	return apiKey[:8]
}
