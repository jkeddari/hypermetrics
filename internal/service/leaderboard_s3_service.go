package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jkeddari/hypermetrics/internal/config"
	"github.com/jkeddari/hypermetrics/internal/hypermetrics/leaderboard"
	"github.com/jkeddari/hypermetrics/internal/storage"
)

type LeaderboardS3Service struct {
	storage   storage.Storage
	s3BaseURL string
}

type s3LeaderboardPayload struct {
	LastUpdate  time.Time                               `json:"last_update"`
	Leaderboard map[string][]leaderboard.LeaderBoardRow `json:"leaderboard"`
}

func NewLeaderboardS3Service(cfg *config.Config) (*LeaderboardS3Service, error) {
	store, err := storage.NewS3Storage(storage.S3Config{
		Region:               cfg.S3Region,
		Bucket:               cfg.S3Bucket,
		AccessKey:            cfg.S3AccessKey,
		SecretKey:            cfg.S3SecretKey,
		Endpoint:             cfg.S3Endpoint,
		PresignExpiryPublic:  cfg.S3PresignExpiryPrivate,
		PresignExpiryPrivate: cfg.S3PresignExpiryPrivate,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	s3BaseURL := cfg.S3Endpoint
	if s3BaseURL == "" {
		s3BaseURL = fmt.Sprintf("https://s3.%s.amazonaws.com/%s", cfg.S3Region, cfg.S3Bucket)
	} else {
		s3BaseURL = convertToPublicURL(s3BaseURL, cfg.S3Bucket)
	}

	return &LeaderboardS3Service{
		storage:   store,
		s3BaseURL: s3BaseURL,
	}, nil
}

func convertToPublicURL(s3Endpoint, bucket string) string {
	if len(s3Endpoint) > 0 && s3Endpoint[len(s3Endpoint)-3:] == "/s3" {
		publicURL := s3Endpoint[:len(s3Endpoint)-3]
		return fmt.Sprintf("%s/object/public/%s", publicURL, bucket)
	}
	return fmt.Sprintf("%s/%s", s3Endpoint, bucket)
}

func (s *LeaderboardS3Service) GetSortedLeaderboard(sortCode string, desc bool) ([]leaderboard.LeaderBoardRow, error) {
	payload, err := s.fetchFromS3()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from S3: %w", err)
	}

	data, exists := payload.Leaderboard[sortCode]
	if !exists {
		return nil, fmt.Errorf("unknown sort code: %s", sortCode)
	}

	return data, nil
}

func (s *LeaderboardS3Service) GetLastRefresh() time.Time {
	payload, err := s.fetchFromS3()
	if err != nil {
		return time.Time{}
	}

	return payload.LastUpdate
}

func (s *LeaderboardS3Service) fetchFromS3() (*s3LeaderboardPayload, error) {
	url := fmt.Sprintf("%s/leaderboard.json", s.s3BaseURL)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP GET failed (url=%s): %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("S3 returned status %d (url=%s)", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body failed: %w", err)
	}

	var payload s3LeaderboardPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("JSON unmarshal failed: %w", err)
	}

	return &payload, nil
}
