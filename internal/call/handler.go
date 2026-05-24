package call

import (
	"errors"
	"net/http"

	"github.com/team-dang/aiseebackend/internal/billing"
	"github.com/team-dang/aiseebackend/internal/middleware"
	"github.com/team-dang/aiseebackend/internal/user"
	"github.com/team-dang/aiseebackend/pkg/httpinput"
	"github.com/team-dang/aiseebackend/pkg/response"
)

type Handler struct {
	service     *Service
	userService *user.Service
}

func NewHandler(
	service *Service, userService *user.Service,
) *Handler {
	return &Handler{
		service:     service,
		userService: userService,
	}
}

func (h *Handler) GetToken(
	w http.ResponseWriter, r *http.Request,
) {
	userID := middleware.UserIDFromContext(r.Context())
	token, err := h.service.GetUserToken(r.Context(), userID, "User")
	if err != nil {
		if writeQuotaError(w, err) {
			return
		}
		response.Err(
			w, http.StatusInternalServerError,
			"token_failed",
			"Failed to create room token",
		)
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{
		"token":     token,
		"room_name": "user-" + userID,
	})
}

type CreateRoomRequest struct {
	CalleeID string `json:"callee_id" validate:"required"`
	CallType string `json:"call_type" validate:"required,oneof=voice video"`
}

func (h *Handler) CreateRoom(
	w http.ResponseWriter, r *http.Request,
) {
	userID := middleware.UserIDFromContext(r.Context())
	var req CreateRoomRequest
	if inputErr := httpinput.DecodeAndValidate(r, &req); inputErr != nil {
		switch {
		case inputErr.Kind == httpinput.ErrorKindValidate && inputErr.Field == "CalleeID":
			response.Err(
				w, http.StatusBadRequest,
				"missing_callee", "callee_id is required",
			)
		case inputErr.Kind == httpinput.ErrorKindValidate && inputErr.Field == "CallType":
			response.Err(
				w, http.StatusBadRequest,
				"invalid_call_type",
				"call_type must be 'voice' or 'video'",
			)
		default:
			response.Err(
				w, http.StatusBadRequest,
				"invalid_body", "Invalid request body",
			)
		}
		return
	}

	linked, err := h.userService.CheckFamilyLink(
		r.Context(), userID, req.CalleeID,
	)
	if err != nil || !linked {
		response.Err(
			w, http.StatusForbidden,
			"not_family",
			"You can only call family members",
		)
		return
	}

	caller, err := h.userService.GetMe(r.Context(), userID)
	if err != nil {
		response.Err(
			w, http.StatusInternalServerError,
			"caller_not_found", "Caller not found",
		)
		return
	}

	callee, err := h.userService.GetMe(
		r.Context(), req.CalleeID,
	)
	if err != nil {
		response.Err(
			w, http.StatusNotFound,
			"callee_not_found", "Callee not found",
		)
		return
	}

	result, err := h.service.CreateRoom(
		r.Context(), caller, callee, req.CallType,
	)
	if err != nil {
		if writeQuotaError(w, err) {
			return
		}
		response.Err(
			w, http.StatusInternalServerError,
			"create_room_failed",
			"Failed to create call room",
		)
		return
	}
	response.JSON(w, http.StatusOK, result)
}

func writeQuotaError(w http.ResponseWriter, err error) bool {
	var quotaErr *billing.QuotaExceededError
	if !errors.As(err, &quotaErr) {
		return false
	}
	response.JSON(w, http.StatusPaymentRequired, map[string]any{
		"error": map[string]any{
			"code":         quotaErr.Code,
			"message":      "Quota exceeded",
			"usage_type":   quotaErr.UsageType,
			"required":     quotaErr.Required,
			"remaining":    quotaErr.Remaining,
			"plan_code":    quotaErr.PlanCode,
			"reset_period": quotaErr.ResetPeriod,
		},
	})
	return true
}

type EndRoomRequest struct {
	RoomName string `json:"room_name" validate:"required"`
}

func (h *Handler) EndRoom(
	w http.ResponseWriter, r *http.Request,
) {
	userID := middleware.UserIDFromContext(r.Context())
	var req EndRoomRequest
	if inputErr := httpinput.DecodeAndValidate(r, &req); inputErr != nil {
		response.Err(
			w, http.StatusBadRequest,
			"invalid_body", "Invalid request body",
		)
		return
	}

	if err := h.service.EndRoom(
		r.Context(), userID, req.RoomName,
	); err != nil {
		response.Err(
			w, http.StatusInternalServerError,
			"end_room_failed", "Failed to end call",
		)
		return
	}
	response.JSON(
		w, http.StatusOK, map[string]bool{"ended": true},
	)
}
