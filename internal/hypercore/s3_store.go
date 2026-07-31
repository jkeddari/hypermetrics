package hypercore

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type s3WalletSignal struct {
	Address string    `json:"address"`
	SeenAt  time.Time `json:"seen_at"`
}

func (s *Store) S3ObjectProcessed(ctx context.Context, objectKey, etag string) (bool, error) {
	if s == nil || s.db == nil {
		return false, ErrStoreUnavailable
	}

	var processed bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM s3_ingestion_objects
			WHERE object_key = $1 AND etag = $2
		)`, objectKey, etag,
	).Scan(&processed)
	return processed, err
}

// ImportS3WalletSignals atomically discovers wallets and records the archive as processed.
func (s *Store) ImportS3WalletSignals(
	ctx context.Context,
	objectKey string,
	etag string,
	signals []WalletSignal,
) (int, error) {
	if s == nil || s.db == nil {
		return 0, ErrStoreUnavailable
	}
	if strings.TrimSpace(objectKey) == "" {
		return 0, fmt.Errorf("S3 object key is required")
	}

	byAddress := make(map[string]time.Time, len(signals))
	for _, signal := range signals {
		address := NormalizeAddress(signal.Address)
		if !IsAddress(address) {
			return 0, fmt.Errorf("invalid wallet address: %q", signal.Address)
		}
		seenAt := signal.SeenAt
		if seenAt.IsZero() {
			seenAt = s.cfg.now()
		}
		if previous, ok := byAddress[address]; !ok || seenAt.After(previous) {
			byAddress[address] = seenAt
		}
	}

	addresses := make([]string, 0, len(byAddress))
	for address := range byAddress {
		addresses = append(addresses, address)
	}
	sort.Strings(addresses)

	payload := make([]s3WalletSignal, 0, len(addresses))
	for _, address := range addresses {
		payload = append(payload, s3WalletSignal{Address: address, SeenAt: byAddress[address]})
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`,
		SourceS3+":"+objectKey,
	); err != nil {
		return 0, err
	}

	var alreadyProcessed bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM s3_ingestion_objects
			WHERE object_key = $1 AND etag = $2
		)`, objectKey, etag,
	).Scan(&alreadyProcessed); err != nil {
		return 0, err
	}
	if alreadyProcessed {
		return 0, tx.Commit()
	}

	var newWallets int
	if err := tx.QueryRowContext(ctx, `
		WITH input AS (
			SELECT address, seen_at
			FROM jsonb_to_recordset($1::jsonb)
				AS x(address TEXT, seen_at TIMESTAMPTZ)
		),
		new_wallets AS (
			INSERT INTO wallets (
				address, first_seen_at, next_refresh_at, priority_score, tier
			)
			SELECT address, seen_at, now(), 100, $2
			FROM input
			ON CONFLICT (address) DO NOTHING
			RETURNING address
		),
		queued AS (
			INSERT INTO wallet_refresh_queue (
				wallet_address, job_kind, next_refresh_at, refresh_deadline_at,
				priority_score
			)
			SELECT address, $3, now(), now() + interval '48 hours', 100
			FROM new_wallets
			ON CONFLICT (wallet_address) DO UPDATE SET
				job_kind = EXCLUDED.job_kind,
				next_refresh_at = LEAST(wallet_refresh_queue.next_refresh_at, EXCLUDED.next_refresh_at),
				refresh_deadline_at = LEAST(wallet_refresh_queue.refresh_deadline_at, EXCLUDED.refresh_deadline_at),
				priority_score = GREATEST(wallet_refresh_queue.priority_score, EXCLUDED.priority_score),
				updated_at = now()
			RETURNING wallet_address
		)
		SELECT count(*) FROM new_wallets`,
		encoded, TierCold, jobWallet,
	).Scan(&newWallets); err != nil {
		return 0, err
	}

	if _, err := tx.ExecContext(ctx, `
		WITH input AS (
			SELECT address, seen_at
			FROM jsonb_to_recordset($1::jsonb)
				AS x(address TEXT, seen_at TIMESTAMPTZ)
		)
		UPDATE wallets w
		SET first_seen_at = LEAST(w.first_seen_at, input.seen_at),
			updated_at = now()
		FROM input
		WHERE w.address = input.address`, encoded,
	); err != nil {
		return 0, err
	}

	if _, err := tx.ExecContext(ctx, `
		WITH input AS (
			SELECT address, seen_at
			FROM jsonb_to_recordset($1::jsonb)
				AS x(address TEXT, seen_at TIMESTAMPTZ)
		)
		INSERT INTO wallet_sources (
			wallet_address, source, first_seen_at, last_seen_at
		)
		SELECT address, $2, seen_at, seen_at
		FROM input
		ON CONFLICT (wallet_address, source) DO UPDATE SET
			first_seen_at = LEAST(wallet_sources.first_seen_at, EXCLUDED.first_seen_at),
			last_seen_at = GREATEST(wallet_sources.last_seen_at, EXCLUDED.last_seen_at)`,
		encoded, SourceS3,
	); err != nil {
		return 0, err
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM wallet_candidates
		WHERE address IN (
			SELECT address
			FROM jsonb_to_recordset($1::jsonb)
				AS x(address TEXT, seen_at TIMESTAMPTZ)
		)`, encoded,
	); err != nil {
		return 0, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO s3_ingestion_objects (
			object_key, etag, wallet_count, new_wallet_count
		) VALUES ($1, $2, $3, $4)
		ON CONFLICT (object_key) DO UPDATE SET
			etag = EXCLUDED.etag,
			processed_at = now(),
			wallet_count = EXCLUDED.wallet_count,
			new_wallet_count = EXCLUDED.new_wallet_count`,
		objectKey, etag, len(payload), newWallets,
	); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return newWallets, nil
}
