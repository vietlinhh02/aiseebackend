package dashboard

import (
	"net/http"

	"github.com/team-dang/aiseebackend/internal/middleware"
	"github.com/team-dang/aiseebackend/pkg/response"
)

// MeHandler serves /api/v1/me/* endpoints where the
// authenticated user fetches their own health data.
type MeHandler struct {
	svc *Service
}

func NewMeHandler(svc *Service) *MeHandler {
	return &MeHandler{svc: svc}
}

func (h *MeHandler) Medications(
	w http.ResponseWriter, r *http.Request,
) {
	userID := middleware.UserIDFromContext(r.Context())

	resp, err := h.svc.Medications(r.Context(), userID)
	if err != nil {
		response.Err(
			w, http.StatusInternalServerError,
			"medications_failed", err.Error(),
		)
		return
	}
	response.JSON(w, http.StatusOK, resp)
}

func (h *MeHandler) Appointments(
	w http.ResponseWriter, r *http.Request,
) {
	userID := middleware.UserIDFromContext(r.Context())

	apts, err := h.svc.Appointments(r.Context(), userID)
	if err != nil {
		response.Err(
			w, http.StatusInternalServerError,
			"appointments_failed", err.Error(),
		)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{
		"appointments": apts,
		"count":        len(apts),
	})
}

func (h *MeHandler) Health(
	w http.ResponseWriter, r *http.Request,
) {
	userID := middleware.UserIDFromContext(r.Context())

	resp, err := h.svc.Health(r.Context(), userID)
	if err != nil {
		response.Err(
			w, http.StatusInternalServerError,
			"health_failed", err.Error(),
		)
		return
	}
	response.JSON(w, http.StatusOK, resp)
}
