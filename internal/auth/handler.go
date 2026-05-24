package auth

import (
	"log/slog"
	"net/http"

	"github.com/team-dang/aiseebackend/pkg/httpinput"
	"github.com/team-dang/aiseebackend/pkg/response"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

type GoogleAuthRequest struct {
	IDToken string `json:"id_token" validate:"required"`
	Role    string `json:"role" validate:"required,oneof=user family"`
}

func (h *Handler) GoogleAuth(w http.ResponseWriter, r *http.Request) {
	var req GoogleAuthRequest
	if inputErr := httpinput.DecodeAndValidate(r, &req); inputErr != nil {
		slog.Warn("auth_google_invalid_body", "error", inputErr)
		switch {
		case inputErr.Kind == httpinput.ErrorKindValidate && inputErr.Field == "IDToken":
			slog.Warn("auth_google_missing_token")
			response.Err(w, http.StatusBadRequest, "missing_token", "id_token is required")
		case inputErr.Kind == httpinput.ErrorKindValidate && inputErr.Field == "Role":
			slog.Warn("auth_google_invalid_role", "role", req.Role)
			response.Err(w, http.StatusBadRequest, "invalid_role", "role must be 'user' or 'family'")
		default:
			response.Err(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		}
		return
	}

	tokens, err := h.service.AuthenticateGoogle(r.Context(), req.IDToken, req.Role)
	if err != nil {
		slog.Warn("auth_google_failed", "error", err, "role", req.Role)
		response.Err(w, http.StatusUnauthorized, "auth_failed", "Google authentication failed")
		return
	}

	slog.Info("auth_google_ok", "role", req.Role)
	response.JSON(w, http.StatusOK, tokens)
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if inputErr := httpinput.DecodeAndValidate(r, &req); inputErr != nil {
		slog.Warn("auth_refresh_invalid_body", "error", inputErr)
		response.Err(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}

	tokens, err := h.service.RefreshToken(r.Context(), req.RefreshToken)
	if err != nil {
		slog.Warn("auth_refresh_failed", "error", err)
		response.Err(w, http.StatusUnauthorized, "invalid_refresh", "Invalid or expired refresh token")
		return
	}

	slog.Info("auth_refresh_ok")
	response.JSON(w, http.StatusOK, tokens)
}
