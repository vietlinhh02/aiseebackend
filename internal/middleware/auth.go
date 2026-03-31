package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"github.com/team-dang/aiseebackend/pkg/response"
)

type contextKey string

const (
	userIDKey contextKey = "user_id"
	roleKey   contextKey = "role"
)

func UserIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(userIDKey).(string)
	return v
}

func RoleFromContext(ctx context.Context) string {
	v, _ := ctx.Value(roleKey).(string)
	return v
}

func Auth(secret string, rdb *redis.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				response.Err(w, http.StatusUnauthorized, "missing_token", "Authorization header required")
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
				return []byte(secret), nil
			})
			if err != nil || !token.Valid {
				response.Err(w, http.StatusUnauthorized, "invalid_token", "Token is invalid or expired")
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				response.Err(w, http.StatusUnauthorized, "invalid_claims", "Cannot parse token claims")
				return
			}

			sub, _ := claims["sub"].(string)
			role, _ := claims["role"].(string)

			// Fast-path: check if user is deactivated or role changed via Redis
			// If key doesn't exist, we allow for now (it will be re-cached on next login/refresh)
			// For stricter security, we could look up the DB if not in Redis.
			if rdb != nil {
				cachedRole, err := rdb.Get(r.Context(), "user:status:"+sub).Result()
				if err == nil && cachedRole != role {
					response.Err(w, http.StatusUnauthorized, "role_changed", "User role has changed or account deactivated")
					return
				}
			}

			ctx := context.WithValue(r.Context(), userIDKey, sub)
			ctx = context.WithValue(ctx, roleKey, role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
