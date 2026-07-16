package handler

import (
	"errors"
	"net/http"

	"predictdestiny/internal/ai"
)

// mapGenericAIError translates the gateway sentinel errors into HTTP
// statuses so every handler gets the right code without redefining
// the same switch. Stage-2 handlers each had a private copy; this is
// the shared version new handlers should use.
func mapGenericAIError(err error) int {
	switch {
	case errors.Is(err, ai.ErrNotConfigured):
		return http.StatusServiceUnavailable
	case errors.Is(err, ai.ErrKeyInvalid):
		return http.StatusUnauthorized
	case errors.Is(err, ai.ErrRateLimited):
		return http.StatusTooManyRequests
	case errors.Is(err, ai.ErrInsufficient):
		return http.StatusPaymentRequired
	case errors.Is(err, ai.ErrModelNotFound):
		return http.StatusNotFound
	case errors.Is(err, ai.ErrTimeout):
		return http.StatusGatewayTimeout
	default:
		return http.StatusBadGateway
	}
}
