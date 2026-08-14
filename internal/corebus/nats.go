package corebus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jkeddari/hypermetrics/internal/hypercore"
	"github.com/nats-io/nats.go"
)

const (
	Subject        = "core.wallet.refresh"
	QueueGroup     = "core-refreshers"
	RequestTimeout = 15 * time.Second
	startupTimeout = 5 * time.Second
)

var (
	ErrCoreUnavailable = errors.New("core unavailable")
	ErrCoreTimeout     = errors.New("core timeout")
	ErrRefreshFailed   = errors.New("wallet refresh failed")
)

type Client struct {
	conn *nats.Conn
}

type refreshRequest struct {
	WalletAddress string    `json:"wallet_address"`
	Deadline      time.Time `json:"deadline"`
}

type refreshResponse struct {
	WalletAddress string    `json:"wallet_address"`
	Status        string    `json:"status"`
	RefreshedAt   time.Time `json:"refreshed_at,omitempty"`
	Error         string    `json:"error,omitempty"`
}

type WalletRefresher interface {
	RefreshWallet(context.Context, string) (hypercore.WalletState, error)
}

func NewClient(conn *nats.Conn) *Client {
	return &Client{conn: conn}
}

func Connect(url, name string) (*nats.Conn, error) {
	conn, err := nats.Connect(url,
		nats.Name(name),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
		nats.ReconnectBufSize(-1),
	)
	if err != nil {
		return nil, err
	}
	if err := conn.FlushTimeout(startupTimeout); err != nil {
		conn.Close()
		return nil, fmt.Errorf("NATS startup health check failed: %w", err)
	}
	return conn, nil
}

func (c *Client) RequestWalletRefresh(ctx context.Context, address string) error {
	if c == nil || c.conn == nil {
		return ErrCoreUnavailable
	}

	requestCtx, cancel := context.WithTimeout(ctx, RequestTimeout)
	defer cancel()
	deadline, _ := requestCtx.Deadline()
	payload, err := json.Marshal(refreshRequest{WalletAddress: address, Deadline: deadline})
	if err != nil {
		return err
	}

	message, err := c.conn.RequestWithContext(requestCtx, Subject, payload)
	if err != nil {
		switch {
		case errors.Is(err, nats.ErrNoResponders), errors.Is(err, nats.ErrConnectionClosed), errors.Is(err, nats.ErrDisconnected), errors.Is(err, nats.ErrNoServers), errors.Is(err, nats.ErrReconnectBufExceeded):
			return ErrCoreUnavailable
		case errors.Is(err, context.DeadlineExceeded), errors.Is(err, nats.ErrTimeout):
			return ErrCoreTimeout
		default:
			return fmt.Errorf("request core wallet refresh: %w", err)
		}
	}

	var response refreshResponse
	if err := json.Unmarshal(message.Data, &response); err != nil {
		return fmt.Errorf("decode core wallet refresh response: %w", err)
	}
	if response.Status != "ok" {
		return fmt.Errorf("%w: %s", ErrRefreshFailed, response.Error)
	}
	return nil
}

func Subscribe(conn *nats.Conn, refresher WalletRefresher) (*nats.Subscription, error) {
	return conn.QueueSubscribe(Subject, QueueGroup, func(message *nats.Msg) {
		go respond(message, refresher)
	})
}

func respond(message *nats.Msg, refresher WalletRefresher) {
	response := process(message.Data, refresher)
	payload, err := json.Marshal(response)
	if err == nil {
		_ = message.Respond(payload)
	}
}

func process(payload []byte, refresher WalletRefresher) refreshResponse {
	var request refreshRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		return refreshResponse{Status: "error", Error: "invalid refresh request"}
	}
	request.WalletAddress = hypercore.NormalizeAddress(request.WalletAddress)
	if !hypercore.IsAddress(request.WalletAddress) {
		return refreshResponse{WalletAddress: request.WalletAddress, Status: "error", Error: "invalid wallet address"}
	}
	if refresher == nil {
		return refreshResponse{WalletAddress: request.WalletAddress, Status: "error", Error: "core unavailable"}
	}

	ctx := context.Background()
	if !request.Deadline.IsZero() {
		if !request.Deadline.After(time.Now()) {
			return refreshResponse{WalletAddress: request.WalletAddress, Status: "error", Error: "refresh request expired"}
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, request.Deadline)
		defer cancel()
	}

	state, err := refresher.RefreshWallet(ctx, request.WalletAddress)
	if err != nil {
		return refreshResponse{WalletAddress: request.WalletAddress, Status: "error", Error: err.Error()}
	}
	return refreshResponse{
		WalletAddress: request.WalletAddress,
		Status:        "ok",
		RefreshedAt:   state.Account.RefreshedAt,
	}
}
