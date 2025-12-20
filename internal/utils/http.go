package utils

import (
	"context"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

type customRoundTripper struct {
	transport http.Transport
	limiter   *rate.Limiter
	timeout   time.Duration
}

// NewRoundTripper returns http.RoundTripper with limiter and custom TLS Config if not nil,
// else default http.Transport is used.
func NewRoundTripper(limiter *rate.Limiter, timeout time.Duration) http.RoundTripper {
	tripper := &customRoundTripper{
		limiter: limiter,
		timeout: timeout,
	}

	return tripper
}

func (rt *customRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(req.Context(), rt.timeout)
	defer cancel()

	if err := rt.limiter.Wait(ctx); err != nil {
		return nil, err
	}

	return rt.transport.RoundTrip(req)
}
