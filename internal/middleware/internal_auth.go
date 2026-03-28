package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/team-dang/aiseebackend/pkg/response"
)

func InternalAuth(apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("X-Internal-Key")
			if subtle.ConstantTimeCompare([]byte(key), []byte(apiKey)) != 1 {
				response.Err(w, http.StatusUnauthorized, "invalid_key", "Invalid internal API key")
				return
			}
			next.ServeHTTP(w, r.WithContext(r.Context()))
		})
	}
}
