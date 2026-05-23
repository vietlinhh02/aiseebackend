package supporter

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/team-dang/aiseebackend/internal/middleware"
	"github.com/team-dang/aiseebackend/pkg/response"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) GetOverview(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	var uid pgtype.UUID
	if err := uid.Scan(userID); err != nil {
		response.Err(w, http.StatusInternalServerError, "invalid_user_id", "user ID parse failed")
		return
	}

	overview, err := h.service.GetOverview(r.Context(), uid)
	if err != nil {
		slog.Warn("supporter_overview_failed", "error", err)
		response.Err(w, http.StatusInternalServerError, "overview_failed", "Failed to load supporter overview")
		return
	}
	response.JSON(w, http.StatusOK, overview)
}

func (h *Handler) WatchAd(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	var uid pgtype.UUID
	if err := uid.Scan(userID); err != nil {
		response.Err(w, http.StatusInternalServerError, "invalid_user_id", "user ID parse failed")
		return
	}

	result, err := h.service.WatchAd(r.Context(), uid)
	if err != nil {
		if err == ErrDailyAdLimitReached {
			response.Err(w, http.StatusTooManyRequests, "daily_ad_limit_reached", err.Error())
			return
		}
		slog.Warn("supporter_watch_ad_failed", "error", err)
		response.Err(w, http.StatusInternalServerError, "watch_ad_failed", "Failed to record ad watch")
		return
	}
	response.JSON(w, http.StatusOK, result)
}

func (h *Handler) Redeem(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	var uid pgtype.UUID
	if err := uid.Scan(userID); err != nil {
		response.Err(w, http.StatusInternalServerError, "invalid_user_id", "user ID parse failed")
		return
	}

	var req RedeemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Err(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}

	result, err := h.service.Redeem(r.Context(), uid, req)
	if err != nil {
		switch err {
		case ErrInsufficientBalance:
			response.Err(w, http.StatusBadRequest, "insufficient_balance", err.Error())
		case ErrInvalidRecipientID:
			response.Err(w, http.StatusBadRequest, "invalid_recipient", err.Error())
		default:
			slog.Warn("supporter_redeem_failed", "error", err)
			response.Err(w, http.StatusInternalServerError, "redeem_failed", "Failed to redeem coins")
		}
		return
	}
	response.JSON(w, http.StatusOK, result)
}
