package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/jkeddari/hypermetrics/internal/hypermetrics/leaderboard"
	"github.com/jkeddari/hypermetrics/internal/storage"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

const filename = "leaderboard.json"

var leaderboardCmd = &cobra.Command{
	Use:   "leaderboard",
	Short: "Fetch and store Hyperliquid leaderboard data",
	Long:  "Fetches leaderboard data from Hyperliquid API, sorts by all combinations, and stores in S3",
	RunE:  runLeaderboard,
}

func init() {
	rootCmd.AddCommand(leaderboardCmd)
}

type LeaderboardData struct {
	LastUpdate  time.Time                               `json:"last_update"`
	Leaderboard map[string][]leaderboard.LeaderBoardRow `json:"leaderboard"`
}

func runLeaderboard(cmd *cobra.Command, args []string) error {
	s3Config := loadS3Config()
	limit := loadLeaderboardLimit()

	store, err := storage.NewS3Storage(s3Config)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	lb, err := leaderboard.NewLiveLeaderboard(nil)
	if err != nil {
		return fmt.Errorf("failed to create leaderboard: %w", err)
	}

	slog.Info("fetching and processing leaderboard data from Hyperliquid API", "limit", limit)

	sortCodes := []struct {
		code string
		name string
	}{
		{"value", "Account Value"},
		{"dailypnl", "Daily PNL"},
		{"dailyroi", "Daily ROI"},
		{"dailyvlm", "Daily Volume"},
		{"weeklypnl", "Weekly PNL"},
		{"weeklyroi", "Weekly ROI"},
		{"weeklyvlm", "Weekly Volume"},
		{"monthlypnl", "Monthly PNL"},
		{"monthlyroi", "Monthly ROI"},
		{"monthlyvlm", "Monthly Volume"},
		{"alltimepnl", "All-Time PNL"},
		{"alltimeroi", "All-Time ROI"},
		{"alltimevlm", "All-Time Volume"},
	}

	data := LeaderboardData{
		LastUpdate:  lb.LastRefresh(),
		Leaderboard: make(map[string][]leaderboard.LeaderBoardRow),
	}

	for _, sc := range sortCodes {
		slog.Info("processing sort combination", "code", sc.code, "metric", sc.name)

		sorted, err := lb.SortBoards(sc.code, true)
		if err != nil {
			return fmt.Errorf("failed to sort by %s: %w", sc.code, err)
		}

		if len(sorted) > limit {
			sorted = sorted[:limit]
		}

		data.Leaderboard[sc.code] = sorted
		slog.Info("sorted leaderboard", "code", sc.code, "rows", len(sorted))
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}

	reader := bytes.NewReader(jsonData)
	if err := store.Save(filename, reader); err != nil {
		return fmt.Errorf("upload to S3: %w", err)
	}

	slog.Info("uploaded leaderboard to S3", "filename", filename, "size_kb", len(jsonData)/1024)
	slog.Info("leaderboard fetch completed successfully", "sort_combinations", len(sortCodes))
	return nil
}

// loadS3Config loads S3 configuration from environment variables
func loadS3Config() storage.S3Config {
	godotenv.Load()

	endpoint := os.Getenv("S3_ENDPOINT")
	bucket := os.Getenv("S3_BUCKET")
	accessKey := os.Getenv("S3_ACCESS_KEY")
	secretKey := os.Getenv("S3_SECRET_KEY")
	region := os.Getenv("S3_REGION")

	if endpoint == "" || bucket == "" || accessKey == "" || secretKey == "" || region == "" {
		slog.Error("missing required S3 environment variables",
			"S3_ENDPOINT", endpoint != "",
			"S3_BUCKET", bucket != "",
			"S3_ACCESS_KEY", accessKey != "",
			"S3_SECRET_KEY", secretKey != "",
			"S3_REGION", region != "",
		)
		os.Exit(1)
	}

	slog.Info("loaded S3 configuration",
		"bucket", bucket,
		"region", region,
		"endpoint", endpoint,
	)

	return storage.S3Config{
		Region:    region,
		Bucket:    bucket,
		AccessKey: accessKey,
		SecretKey: secretKey,
		Endpoint:  endpoint,
	}
}

func loadLeaderboardLimit() int {
	limitStr := os.Getenv("LEADERBOARD_LIMIT")
	if limitStr == "" {
		return 100
	}

	if limit, err := strconv.Atoi(limitStr); err != nil {
		return limit
	}

	slog.Warn("invalid LEADERBOARD_LIMIT, using default", "value", limitStr, "default", 100)
	return 100
}
