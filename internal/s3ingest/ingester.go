package s3ingest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/jkeddari/hypermetrics/internal/hypercore"
	"github.com/pierrec/lz4/v4"
)

const (
	Bucket = "hl-mainnet-node-data"
	Prefix = "node_fills_by_block/hourly"
)

type Config struct {
	Lookback     time.Duration
	PollInterval time.Duration
}

type Stats struct {
	ObjectsFound     int
	ObjectsProcessed int
	ObjectsSkipped   int
	FillsRead        int
	WalletsSeen      int
	NewWallets       int
}

type s3Client interface {
	ListObjectsV2(context.Context, *s3.ListObjectsV2Input, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

type walletStore interface {
	S3ObjectProcessed(context.Context, string, string) (bool, error)
	ImportS3WalletSignals(context.Context, string, string, []hypercore.WalletSignal) (int, error)
}

type Ingester struct {
	client s3Client
	store  walletStore
	cfg    Config
}

func New(client *s3.Client, store *hypercore.Store, cfg Config) *Ingester {
	if cfg.Lookback <= 0 {
		cfg.Lookback = 30 * 24 * time.Hour
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 15 * time.Minute
	}
	return &Ingester{client: client, store: store, cfg: cfg}
}

func (i *Ingester) Run(ctx context.Context) error {
	if err := i.runAndLog(ctx); err != nil {
		slog.Error("S3 ingestion failed", "error", err)
	}

	ticker := time.NewTicker(i.cfg.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := i.runAndLog(ctx); err != nil {
				slog.Error("S3 ingestion failed", "error", err)
			}
		}
	}
}

func (i *Ingester) runAndLog(ctx context.Context) error {
	stats, err := i.RunOnce(ctx, time.Now().UTC())
	slog.Info("S3 ingestion pass completed",
		"objects_found", stats.ObjectsFound,
		"objects_processed", stats.ObjectsProcessed,
		"objects_skipped", stats.ObjectsSkipped,
		"fills_read", stats.FillsRead,
		"wallets_seen", stats.WalletsSeen,
		"new_wallets", stats.NewWallets,
	)
	return err
}

func (i *Ingester) RunOnce(ctx context.Context, now time.Time) (Stats, error) {
	if i == nil || i.client == nil || i.store == nil {
		return Stats{}, errors.New("S3 ingester is not initialized")
	}

	start := now.UTC().Add(-i.cfg.Lookback).Truncate(24 * time.Hour)
	end := now.UTC().Truncate(24 * time.Hour)
	var stats Stats
	var runErrors []error

	for day := start; !day.After(end); day = day.Add(24 * time.Hour) {
		if err := ctx.Err(); err != nil {
			return stats, err
		}
		prefix := fmt.Sprintf("%s/%s/", Prefix, day.Format("20060102"))
		output, err := i.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:       aws.String(Bucket),
			Prefix:       aws.String(prefix),
			RequestPayer: types.RequestPayerRequester,
		})
		if err != nil {
			runErrors = append(runErrors, fmt.Errorf("list %s: %w", prefix, err))
			continue
		}

		sort.Slice(output.Contents, func(a, b int) bool {
			return aws.ToString(output.Contents[a].Key) < aws.ToString(output.Contents[b].Key)
		})
		for _, object := range output.Contents {
			if err := ctx.Err(); err != nil {
				return stats, err
			}
			key := aws.ToString(object.Key)
			if !strings.HasSuffix(key, ".lz4") {
				continue
			}
			stats.ObjectsFound++
			processed, err := i.processObject(ctx, key, aws.ToString(object.ETag), &stats)
			if err != nil {
				runErrors = append(runErrors, fmt.Errorf("process %s: %w", key, err))
				continue
			}
			if processed {
				stats.ObjectsProcessed++
			} else {
				stats.ObjectsSkipped++
			}
		}
	}

	return stats, errors.Join(runErrors...)
}

func (i *Ingester) processObject(ctx context.Context, key, etag string, stats *Stats) (bool, error) {
	processed, err := i.store.S3ObjectProcessed(ctx, key, etag)
	if err != nil || processed {
		return false, err
	}

	object, err := i.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket:       aws.String(Bucket),
		Key:          aws.String(key),
		RequestPayer: types.RequestPayerRequester,
	})
	if err != nil {
		return false, err
	}
	defer object.Body.Close()

	signals, fills, err := ExtractWalletSignals(lz4.NewReader(object.Body))
	if err != nil {
		return false, err
	}
	newWallets, err := i.store.ImportS3WalletSignals(ctx, key, etag, signals)
	if err != nil {
		return false, err
	}

	stats.FillsRead += fills
	stats.WalletsSeen += len(signals)
	stats.NewWallets += newWallets
	return true, nil
}

type fillBlock struct {
	BlockTime json.RawMessage   `json:"block_time"`
	Events    []json.RawMessage `json:"events"`
}

type fillMeta struct {
	Time int64 `json:"time"`
}

func ExtractWalletSignals(reader io.Reader) ([]hypercore.WalletSignal, int, error) {
	decoder := json.NewDecoder(reader)
	latest := make(map[string]time.Time)
	fills := 0

	for {
		var block fillBlock
		if err := decoder.Decode(&block); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, fills, err
		}

		blockTime := parseJSONTime(block.BlockTime)
		for _, rawEvent := range block.Events {
			var event []json.RawMessage
			if err := json.Unmarshal(rawEvent, &event); err != nil || len(event) < 1 {
				continue
			}
			var address string
			if err := json.Unmarshal(event[0], &address); err != nil {
				continue
			}
			address = hypercore.NormalizeAddress(address)
			if !hypercore.IsAddress(address) {
				continue
			}

			seenAt := blockTime
			if len(event) > 1 {
				var fill fillMeta
				if json.Unmarshal(event[1], &fill) == nil && fill.Time > 0 {
					seenAt = time.UnixMilli(fill.Time).UTC()
				}
			}
			if previous, ok := latest[address]; !ok || seenAt.After(previous) {
				latest[address] = seenAt
			}
			fills++
		}
	}

	addresses := make([]string, 0, len(latest))
	for address := range latest {
		addresses = append(addresses, address)
	}
	sort.Strings(addresses)

	signals := make([]hypercore.WalletSignal, 0, len(addresses))
	for _, address := range addresses {
		signals = append(signals, hypercore.WalletSignal{
			Address: address,
			Source:  hypercore.SourceS3,
			SeenAt:  latest[address],
		})
	}
	return signals, fills, nil
}

func parseJSONTime(raw json.RawMessage) time.Time {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.999999"} {
			if parsed, err := time.Parse(layout, text); err == nil {
				return parsed.UTC()
			}
		}
	}

	var milliseconds int64
	if json.Unmarshal(raw, &milliseconds) == nil && milliseconds > 0 {
		return time.UnixMilli(milliseconds).UTC()
	}
	return time.Time{}
}
