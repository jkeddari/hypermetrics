package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type EmailSender struct {
	apiKey     string
	from       string
	resendURL  string
	httpClient *http.Client
	devMode    bool
}

func NewEmailSender(apiKey, from string, devMode bool) *EmailSender {
	return &EmailSender{
		apiKey: apiKey, from: from, devMode: devMode,
		resendURL: "https://api.resend.com/emails", httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *EmailSender) Send(ctx context.Context, to, subject, textBody string, htmlBody ...string) error {
	if s.apiKey == "" || s.from == "" {
		if s.devMode {
			slog.Info("email skipped (development)", "to", to, "subject", subject, "body", textBody)
			return nil
		}
		return errors.New("email service is not configured")
	}
	payload := map[string]any{
		"from": s.from, "to": []string{to}, "subject": subject, "text": textBody,
	}
	if len(htmlBody) > 0 && htmlBody[0] != "" {
		payload["html"] = htmlBody[0]
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.resendURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("send email: Resend returned %s: %s", resp.Status, strings.TrimSpace(string(message)))
	}
	return nil
}
