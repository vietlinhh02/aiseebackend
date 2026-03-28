package user

import (
	"encoding/json"
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

func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	user, err := h.service.GetMe(r.Context(), userID)
	if err != nil {
		response.Err(w, http.StatusNotFound, "user_not_found", "User not found")
		return
	}
	response.JSON(w, http.StatusOK, user)
}

func (h *Handler) GetFamily(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	family, err := h.service.GetFamily(r.Context(), userID)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "fetch_failed", "Failed to fetch family")
		return
	}
	response.JSON(w, http.StatusOK, family)
}

type CreateInviteRequest struct {
	Relationship string `json:"relationship"`
}

func (h *Handler) CreateFamilyLink(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	var req CreateInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Err(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	code := h.service.CreateInvite(r.Context(), userID, req.Relationship)
	response.JSON(w, http.StatusOK, map[string]string{"invite_code": code})
}

type JoinFamilyRequest struct {
	InviteCode string `json:"invite_code"`
}

func (h *Handler) JoinFamily(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	var req JoinFamilyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Err(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	if err := h.service.JoinFamily(r.Context(), userID, req.InviteCode); err != nil {
		response.Err(w, http.StatusBadRequest, "join_failed", err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{"status": "joined"})
}
