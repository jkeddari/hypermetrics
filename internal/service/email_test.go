package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func NewEmailSender(apiKey, from string, devMode bool) *emailSender {
	return newEmailSender(apiKey, from, devMode)
}

func (s *emailSender) Send(ctx context.Context, to, subject, textBody string, htmlBody ...string) error {
	return s.send(ctx, to, subject, textBody, htmlBody...)
}

func TestEmailSenderUsesResendAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer re_test" {
			t.Fatalf("authorization = %q", got)
		}
		var payload struct {
			From    string   `json:"from"`
			To      []string `json:"to"`
			Subject string   `json:"subject"`
			Text    string   `json:"text"`
			HTML    string   `json:"html"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.From != "Hypermetrics <hello@example.com>" || len(payload.To) != 1 || payload.To[0] != "trader@example.com" || payload.Subject != "Sign in" || payload.Text != "secure link" || payload.HTML != "<a href=\"https://example.com\">Sign in</a>" {
			t.Fatalf("unexpected payload: %+v", payload)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewEmailSender("re_test", "Hypermetrics <hello@example.com>", false)
	sender.resendURL = server.URL
	sender.httpClient = server.Client()
	if err := sender.Send(context.Background(), "trader@example.com", "Sign in", "secure link", "<a href=\"https://example.com\">Sign in</a>"); err != nil {
		t.Fatal(err)
	}
}
