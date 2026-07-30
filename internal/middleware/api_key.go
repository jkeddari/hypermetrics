package middleware

import (
	"encoding/json"
	"net/http"
	"strings"

	apimodel "github.com/jkeddari/hypermetrics/internal/model/api"
	"github.com/jkeddari/hypermetrics/internal/service"
)

func RequireAPIKey(apiKeyService *service.APIKeyService) func(http.HandlerFunc) http.HandlerFunc {
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

			_, err := apiKeyService.ValidateAPIKey(apiKey)
			if err != nil {
				switch err {
				case service.ErrInvalidAPIKey:
					writeAPIJSON(w, http.StatusUnauthorized, apimodel.ResponseEnvelope[any]{
						Code: "1000",
						Msg:  "invalid api key",
						Data: nil,
					})
				case service.ErrAPIKeyValidationNotImplemented:
					writeAPIJSON(w, http.StatusNotImplemented, apimodel.ResponseEnvelope[any]{
						Code: "1006",
						Msg:  "api key validation not implemented",
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

			next.ServeHTTP(w, r)
		}
	}
}

func writeAPIJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
