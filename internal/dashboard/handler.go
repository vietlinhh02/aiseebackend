package dashboard

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/team-dang/aiseebackend/internal/middleware"
	"github.com/team-dang/aiseebackend/pkg/httpinput"
	"github.com/team-dang/aiseebackend/pkg/response"
)

type Handler struct {
	svc     *Service
	checker middleware.FamilyChecker
}

func NewHandler(
	svc *Service, checker middleware.FamilyChecker,
) *Handler {
	return &Handler{svc: svc, checker: checker}
}

// authorize extracts user_id from the URL and verifies the
// caller has a family link to that user.
func (h *Handler) authorize(
	r *http.Request,
) (string, error) {
	userID := chi.URLParam(r, "user_id")
	return userID, middleware.AuthorizeFamily(
		r.Context(), userID, h.checker,
	)
}

func (h *Handler) Health(
	w http.ResponseWriter, r *http.Request,
) {
	userID, err := h.authorize(r)
	if err != nil {
		response.Err(
			w, http.StatusForbidden, "forbidden", err.Error(),
		)
		return
	}

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

func (h *Handler) Medications(
	w http.ResponseWriter, r *http.Request,
) {
	userID, err := h.authorize(r)
	if err != nil {
		response.Err(
			w, http.StatusForbidden, "forbidden", err.Error(),
		)
		return
	}

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

func (h *Handler) AuditLog(
	w http.ResponseWriter, r *http.Request,
) {
	userID, err := h.authorize(r)
	if err != nil {
		response.Err(
			w, http.StatusForbidden, "forbidden", err.Error(),
		)
		return
	}

	page := parseIntParam(r, "page", 1)
	limit := parseIntParam(r, "limit", 20)
	if limit > 100 {
		limit = 100
	}

	logs, err := h.svc.AuditLog(
		r.Context(), userID, page, limit,
	)
	if err != nil {
		response.Err(
			w, http.StatusInternalServerError,
			"audit_failed", err.Error(),
		)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{
		"logs":  logs,
		"page":  page,
		"limit": limit,
	})
}

func (h *Handler) Location(
	w http.ResponseWriter, r *http.Request,
) {
	userID, err := h.authorize(r)
	if err != nil {
		response.Err(
			w, http.StatusForbidden, "forbidden", err.Error(),
		)
		return
	}

	resp, err := h.svc.Location(r.Context(), userID)
	if err != nil {
		response.Err(
			w, http.StatusInternalServerError,
			"location_failed", err.Error(),
		)
		return
	}
	response.JSON(w, http.StatusOK, resp)
}

func (h *Handler) CreateMedication(
	w http.ResponseWriter, r *http.Request,
) {
	userID, err := h.authorize(r)
	if err != nil {
		response.Err(
			w, http.StatusForbidden, "forbidden", err.Error(),
		)
		return
	}

	var req CreateMedRequest
	if inputErr := httpinput.DecodeAndValidate(r, &req); inputErr != nil {
		switch {
		case inputErr.Kind == httpinput.ErrorKindValidate &&
			(inputErr.Field == "Name" || inputErr.Field == "Dosage"):
			response.Err(
				w, http.StatusBadRequest,
				"validation", "name and dosage are required",
			)
		default:
			response.Err(
				w, http.StatusBadRequest,
				"invalid_body", "Invalid request body",
			)
		}
		return
	}
	if req.Frequency == "" {
		req.Frequency = "daily"
	}

	familyID := middleware.UserIDFromContext(r.Context())
	med, err := h.svc.CreateMedication(
		r.Context(), userID, familyID, req,
	)
	if err != nil {
		response.Err(
			w, http.StatusInternalServerError,
			"create_failed", err.Error(),
		)
		return
	}
	response.JSON(w, http.StatusCreated, med)
}

func (h *Handler) UpdateMedication(
	w http.ResponseWriter, r *http.Request,
) {
	userID, err := h.authorize(r)
	if err != nil {
		response.Err(
			w, http.StatusForbidden, "forbidden", err.Error(),
		)
		return
	}

	medID := chi.URLParam(r, "med_id")
	var req UpdateMedRequest
	if inputErr := httpinput.DecodeAndValidate(r, &req); inputErr != nil {
		response.Err(
			w, http.StatusBadRequest,
			"invalid_body", "Invalid request body",
		)
		return
	}

	familyID := middleware.UserIDFromContext(r.Context())
	med, err := h.svc.UpdateMedication(
		r.Context(), userID, medID, familyID, req,
	)
	if err != nil {
		response.Err(
			w, http.StatusInternalServerError,
			"update_failed", err.Error(),
		)
		return
	}
	response.JSON(w, http.StatusOK, med)
}

func (h *Handler) DeleteMedication(
	w http.ResponseWriter, r *http.Request,
) {
	userID, err := h.authorize(r)
	if err != nil {
		response.Err(
			w, http.StatusForbidden, "forbidden", err.Error(),
		)
		return
	}

	medID := chi.URLParam(r, "med_id")
	familyID := middleware.UserIDFromContext(r.Context())
	if err := h.svc.DeleteMedication(
		r.Context(), userID, medID, familyID,
	); err != nil {
		response.Err(
			w, http.StatusInternalServerError,
			"delete_failed", err.Error(),
		)
		return
	}
	response.JSON(
		w, http.StatusOK,
		map[string]string{"status": "deleted"},
	)
}

func parseIntParam(
	r *http.Request, key string, fallback int32,
) int32 {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return fallback
	}
	return int32(v)
}
