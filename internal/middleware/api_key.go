package middleware

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	apimodel "github.com/jkeddari/hypermetrics/internal/model/api"
	"github.com/jkeddari/hypermetrics/internal/service"
)

func RequireAPIKey(apiKeyService *service.APIKeyService) func(http.HandlerFunc) http.HandlerFunc {
	limiter := NewRateLimiter(600, time.Minute)
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			apiKey := strings.TrimSpace(r.Header.Get("HM-API-KEY"))
			if apiKey == "" {
				writeAPIJSON(w, http.StatusUnauthorized, apimodel.ResponseEnvelope[any]{
					Code: "1000",
					Msg:  "missing api key",
					Data: nil,
				})
				return
			}

			key, err := apiKeyService.ValidateAPIKey(apiKey)
			if err != nil {
				switch err {
				case service.ErrInvalidAPIKey:
					writeAPIJSON(w, http.StatusUnauthorized, apimodel.ResponseEnvelope[any]{
						Code: "1000",
						Msg:  "invalid api key",
						Data: nil,
					})
				default:
					writeAPIJSON(w, http.StatusInternalServerError, apimodel.ResponseEnvelope[any]{
						Code: "1006",
						Msg:  "internal error during api key validation",
						Data: nil,
					})
				}
				return
			}
			if !limiter.AllowLimit(key.ID, planRateLimit(key.PlanID)) {
				w.Header().Set("Retry-After", "60")
				writeAPIJSON(w, http.StatusTooManyRequests, apimodel.ResponseEnvelope[any]{
					Code: "1007", Msg: "API rate limit exceeded", Data: nil,
				})
				return
			}

			next.ServeHTTP(w, r)
		}
	}
}

func planRateLimit(planID string) int {
	if planID == "builder" {
		return 60
	}
	return 600
}

func writeAPIJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
