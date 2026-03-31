package volunteer

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/team-dang/aiseebackend/internal/middleware"
	"github.com/team-dang/aiseebackend/pkg/response"
)

// Handler exposes volunteer REST endpoints.
type Handler struct {
	svc *Service
}

// NewHandler creates a volunteer Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Register handles POST /api/v1/volunteers/register.
func (h *Handler) Register(
	w http.ResponseWriter, r *http.Request,
) {
	userID := middleware.UserIDFromContext(r.Context())

	var body struct {
		Languages []string `json:"languages"`
	}
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			response.Err(
				w, http.StatusBadRequest,
				"invalid_body", "Invalid request body",
			)
			return
		}
	}
	if len(body.Languages) == 0 {
		body.Languages = []string{"vi"}
	}

	if err := h.svc.Register(
		r.Context(), userID, body.Languages,
	); err != nil {
		response.Err(
			w, http.StatusInternalServerError,
			"register_failed", err.Error(),
		)
		return
	}

	response.JSON(
		w, http.StatusCreated,
		map[string]string{"status": "registered"},
	)
}

// GetStatus handles GET /api/v1/volunteers/status.
func (h *Handler) GetStatus(
	w http.ResponseWriter, r *http.Request,
) {
	userID := middleware.UserIDFromContext(r.Context())

	vol, isRegistered, err := h.svc.GetRegistrationStatus(
		r.Context(), userID,
	)
	if err != nil {
		response.Err(
			w, http.StatusInternalServerError,
			"status_failed", err.Error(),
		)
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"is_registered": isRegistered,
		"volunteer":     vol,
	})
}

// SetAvailability handles
// PUT /api/v1/volunteers/availability.
func (h *Handler) SetAvailability(
	w http.ResponseWriter, r *http.Request,
) {
	userID := middleware.UserIDFromContext(r.Context())

	var body struct {
		Available bool `json:"available"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Err(
			w, http.StatusBadRequest,
			"invalid_body", "Invalid request body",
		)
		return
	}

	if err := h.svc.SetAvailability(
		r.Context(), userID, body.Available,
	); err != nil {
		response.Err(
			w, http.StatusInternalServerError,
			"availability_failed", err.Error(),
		)
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"status":    "updated",
		"available": body.Available,
	})
}

// StartMatch handles POST /api/v1/volunteers/match.
func (h *Handler) StartMatch(
	w http.ResponseWriter, r *http.Request,
) {
	userID := middleware.UserIDFromContext(r.Context())

	result, err := h.svc.StartMatch(r.Context(), userID)
	if err != nil {
		response.Err(
			w, http.StatusInternalServerError,
			"match_failed", err.Error(),
		)
		return
	}

	response.JSON(w, http.StatusAccepted, result)
}

// MatchStatus handles
// GET /api/v1/volunteers/match/{match_id}/status.
func (h *Handler) MatchStatus(
	w http.ResponseWriter, r *http.Request,
) {
	matchID := chi.URLParam(r, "match_id")

	status, err := h.svc.GetMatchStatus(
		r.Context(), matchID,
	)
	if err != nil {
		response.Err(
			w, http.StatusInternalServerError,
			"match_status_failed", err.Error(),
		)
		return
	}

	response.JSON(w, http.StatusOK, status)
}

// Rate handles POST /api/v1/volunteers/rate.
func (h *Handler) Rate(
	w http.ResponseWriter, r *http.Request,
) {
	raterID := middleware.UserIDFromContext(r.Context())

	var body struct {
		VolunteerID string `json:"volunteer_id"`
		MatchID     string `json:"match_id"`
		Score       int16  `json:"score"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Err(
			w, http.StatusBadRequest,
			"invalid_body", "Invalid request body",
		)
		return
	}

	if body.Score < 1 || body.Score > 5 {
		response.Err(
			w, http.StatusBadRequest,
			"invalid_score", "Score must be 1-5",
		)
		return
	}

	if err := h.svc.Rate(
		r.Context(),
		body.VolunteerID, raterID, body.MatchID, body.Score,
	); err != nil {
		response.Err(
			w, http.StatusInternalServerError,
			"rate_failed", err.Error(),
		)
		return
	}

	response.JSON(
		w, http.StatusOK,
		map[string]string{"status": "rated"},
	)
}
