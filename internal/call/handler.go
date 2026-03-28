package call

import (
	"net/http"

	"github.com/team-dang/aiseebackend/internal/middleware"
	"github.com/team-dang/aiseebackend/pkg/response"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) GetToken(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	token, err := h.service.GetUserToken(userID, "User")
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "token_failed", "Failed to create room token")
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{
		"token":     token,
		"room_name": "user-" + userID,
	})
}
