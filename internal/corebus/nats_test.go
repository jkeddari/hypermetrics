package corebus

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jkeddari/hypermetrics/internal/hypercore"
	"github.com/nats-io/nats.go"
)

func TestProcessRefreshesWalletBeforeReply(t *testing.T) {
	address := "0x0000000000000000000000000000000000000001"
	refreshedAt := time.Now().UTC()
	response := process(mustRequest(t, refreshRequest{
		WalletAddress: address,
		Deadline:      refreshedAt.Add(time.Second),
	}), fakeRefresher{state: hypercore.WalletState{
		Account: hypercore.WalletAccount{Address: address, RefreshedAt: refreshedAt},
	}})

	if response.Status != "ok" || response.WalletAddress != address || !response.RefreshedAt.Equal(refreshedAt) {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestClientDoesNotBufferRequestsWhileDisconnected(t *testing.T) {
	connection, err := nats.Connect("nats://127.0.0.1:1",
		nats.Name("corebus-test"),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
		nats.ReconnectBufSize(-1),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err = NewClient(connection).RequestWalletRefresh(ctx, "0x0000000000000000000000000000000000000001")
	if !errors.Is(err, ErrCoreUnavailable) {
		t.Fatalf("expected unavailable core without buffered publish, got %v", err)
	}
}

type fakeRefresher struct {
	state hypercore.WalletState
}

func (f fakeRefresher) RefreshWallet(context.Context, string) (hypercore.WalletState, error) {
	return f.state, nil
}

func mustRequest(t *testing.T, request refreshRequest) []byte {
	t.Helper()
	payload, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}
