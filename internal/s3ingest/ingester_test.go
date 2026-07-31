package s3ingest

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/jkeddari/hypermetrics/internal/hypercore"
	"github.com/pierrec/lz4/v4"
)

const (
	testAddressA = "0x1111111111111111111111111111111111111111"
	testAddressB = "0x2222222222222222222222222222222222222222"
)

func TestExtractWalletSignalsDeduplicatesAddresses(t *testing.T) {
	input := strings.Join([]string{
		`{"block_time":"2025-07-28T14:00:00.022000","events":[["` + testAddressA + `",{"time":1753711200000}],["` + testAddressB + `",{"time":1753711201000}]]}`,
		`{"block_time":"2025-07-28T14:01:00Z","events":[["` + testAddressA + `",{"time":1753711260000}],["invalid",{"time":1753711260000}]]}`,
	}, "\n")

	signals, fills, err := ExtractWalletSignals(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if fills != 3 {
		t.Fatalf("expected 3 valid fills, got %d", fills)
	}
	if len(signals) != 2 {
		t.Fatalf("expected 2 wallets, got %d", len(signals))
	}
	if signals[0].Address != testAddressA || signals[0].Source != hypercore.SourceS3 {
		t.Fatalf("unexpected first signal: %+v", signals[0])
	}
	expected := time.UnixMilli(1753711260000).UTC()
	if !signals[0].SeenAt.Equal(expected) {
		t.Fatalf("expected latest fill time %s, got %s", expected, signals[0].SeenAt)
	}
}

func TestRunOnceProcessesThenSkipsObject(t *testing.T) {
	key := Prefix + "/20250728/14.lz4"
	body := compressed(t, `{"block_time":"2025-07-28T14:00:00Z","events":[["`+
		testAddressA+`",{"time":1753711200000}]]}`)
	client := &fakeS3{
		key:  key,
		etag: `"etag-1"`,
		body: body,
	}
	store := &fakeStore{processed: make(map[string]string)}
	ingester := &Ingester{
		client: client,
		store:  store,
		cfg: Config{
			Lookback: time.Hour,
		},
	}
	now := time.Date(2025, 7, 28, 15, 0, 0, 0, time.UTC)

	stats, err := ingester.RunOnce(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if stats.ObjectsProcessed != 1 || stats.NewWallets != 1 || stats.WalletsSeen != 1 {
		t.Fatalf("unexpected first pass stats: %+v", stats)
	}
	if client.getCalls != 1 {
		t.Fatalf("expected one download, got %d", client.getCalls)
	}

	stats, err = ingester.RunOnce(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if stats.ObjectsSkipped != 1 || stats.ObjectsProcessed != 0 {
		t.Fatalf("unexpected second pass stats: %+v", stats)
	}
	if client.getCalls != 1 {
		t.Fatalf("processed object was downloaded again: %d calls", client.getCalls)
	}
}

type fakeS3 struct {
	key      string
	etag     string
	body     []byte
	getCalls int
}

func (f *fakeS3) ListObjectsV2(
	context.Context,
	*s3.ListObjectsV2Input,
	...func(*s3.Options),
) (*s3.ListObjectsV2Output, error) {
	return &s3.ListObjectsV2Output{
		Contents: []types.Object{{
			Key:  aws.String(f.key),
			ETag: aws.String(f.etag),
		}},
	}, nil
}

func (f *fakeS3) GetObject(
	context.Context,
	*s3.GetObjectInput,
	...func(*s3.Options),
) (*s3.GetObjectOutput, error) {
	f.getCalls++
	return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(f.body))}, nil
}

type fakeStore struct {
	processed map[string]string
}

func (f *fakeStore) S3ObjectProcessed(_ context.Context, key, etag string) (bool, error) {
	return f.processed[key] == etag, nil
}

func (f *fakeStore) ImportS3WalletSignals(
	_ context.Context,
	key string,
	etag string,
	signals []hypercore.WalletSignal,
) (int, error) {
	f.processed[key] = etag
	return len(signals), nil
}

func compressed(t *testing.T, input string) []byte {
	t.Helper()
	var output bytes.Buffer
	writer := lz4.NewWriter(&output)
	if _, err := writer.Write([]byte(input)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}
