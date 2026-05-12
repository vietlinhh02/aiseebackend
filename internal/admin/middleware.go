package admin

import (
	"context"
	"net/http"

	"github.com/team-dang/aiseebackend/internal/middleware"
	"github.com/team-dang/aiseebackend/pkg/response"
)

func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role := middleware.RoleFromContext(r.Context())
		if role != "admin" {
			response.Err(w, http.StatusForbidden, "admin_required", "Admin access required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

type AdminChecker struct {
	store *Store
}

func NewAdminChecker(store *Store) *AdminChecker {
	return &AdminChecker{store: store}
}

func (c *AdminChecker) IsAdmin(ctx context.Context, userID string) (bool, error) {
	return c.store.IsAdmin(ctx, userID)
}
