package auth

import (
	"encoding/json"
	"net/http"

	"github.com/team-dang/aiseebackend/pkg/response"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

type GoogleAuthRequest struct {
	IDToken string `json:"id_token"`
	Role    string `json:"role"`
}

func (h *Handler) GoogleAuth(w http.ResponseWriter, r *http.Request) {
	var req GoogleAuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Err(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}

	if req.IDToken == "" {
		response.Err(w, http.StatusBadRequest, "missing_token", "id_token is required")
		return
	}

	if req.Role != "user" && req.Role != "family" {
		response.Err(w, http.StatusBadRequest, "invalid_role", "role must be 'user' or 'family'")
		return
	}

	tokens, err := h.service.AuthenticateGoogle(r.Context(), req.IDToken, req.Role)
	if err != nil {
		response.Err(w, http.StatusUnauthorized, "auth_failed", "Google authentication failed")
		return
	}

	response.JSON(w, http.StatusOK, tokens)
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Err(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}

	tokens, err := h.service.RefreshToken(r.Context(), req.RefreshToken)
	if err != nil {
		response.Err(w, http.StatusUnauthorized, "invalid_refresh", "Invalid or expired refresh token")
		return
	}

	response.JSON(w, http.StatusOK, tokens)
}
