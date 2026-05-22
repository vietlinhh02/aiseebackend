package user

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
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

func (h *Handler) GetCurrentGroup(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	group, err := h.service.GetCurrentGroup(r.Context(), userID)
	if err != nil {
		switch {
		case errors.Is(err, ErrNoActiveGroup):
			response.Err(w, http.StatusNotFound, "no_active_group", err.Error())
		default:
			response.Err(w, http.StatusInternalServerError, "fetch_group_failed", "Failed to fetch current group")
		}
		return
	}
	response.JSON(w, http.StatusOK, group)
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
	invite, err := h.service.CreateInvite(r.Context(), userID, req.Relationship)
	if err != nil {
		if errors.Is(err, ErrInviteCreateForbidden) || errors.Is(err, ErrGroupOwnerOnly) {
			response.Err(w, http.StatusForbidden, "invite_forbidden", err.Error())
			return
		}
		response.Err(w, http.StatusBadRequest, "invite_failed", err.Error())
		return
	}
	response.JSON(w, http.StatusOK, invite)
}

func (h *Handler) RevokeFamilyInvite(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	inviteCode := chi.URLParam(r, "invite_code")
	if err := h.service.RevokeInvite(r.Context(), userID, inviteCode); err != nil {
		switch {
		case errors.Is(err, ErrInviteInvalid):
			response.Err(w, http.StatusBadRequest, "invite_invalid", err.Error())
		case errors.Is(err, ErrInviteRevokeForbidden):
			response.Err(w, http.StatusForbidden, "invite_revoke_forbidden", err.Error())
		default:
			response.Err(w, http.StatusBadRequest, "invite_revoke_failed", err.Error())
		}
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{"status": "revoked"})
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
		switch {
		case errors.Is(err, ErrInviteInvalid):
			response.Err(w, http.StatusBadRequest, "invite_invalid", err.Error())
		case errors.Is(err, ErrInviteExpired):
			response.Err(w, http.StatusBadRequest, "invite_expired", err.Error())
		case errors.Is(err, ErrInviteAlreadyUsed):
			response.Err(w, http.StatusConflict, "invite_already_used", err.Error())
		case errors.Is(err, ErrFamilyLimitReached):
			response.Err(w, http.StatusConflict, "family_members_limit_reached", err.Error())
		case errors.Is(err, ErrCaredUserLimitReached):
			response.Err(w, http.StatusConflict, "cared_users_limit_reached", err.Error())
		case errors.Is(err, ErrGroupJoinRequiresLeaveCurrent):
			response.Err(w, http.StatusConflict, "group_join_requires_leave_current", err.Error())
		case errors.Is(err, ErrGroupMemberAlreadyJoined):
			response.Err(w, http.StatusConflict, "group_member_already_joined", err.Error())
		default:
			response.Err(w, http.StatusBadRequest, "join_failed", err.Error())
		}
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{"status": "joined"})
}

func (h *Handler) LeaveGroup(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if err := h.service.LeaveGroup(r.Context(), userID); err != nil {
		switch {
		case errors.Is(err, ErrGroupOwnerTransferRequired):
			response.Err(w, http.StatusConflict, "group_owner_transfer_required", err.Error())
		default:
			response.Err(w, http.StatusBadRequest, "leave_group_failed", err.Error())
		}
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{"status": "left"})
}

type TransferGroupOwnershipRequest struct {
	TargetUserID string `json:"target_user_id"`
}

func (h *Handler) TransferGroupOwnership(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	var req TransferGroupOwnershipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Err(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	if req.TargetUserID == "" {
		response.Err(w, http.StatusBadRequest, "missing_target_user", "target_user_id is required")
		return
	}
	if err := h.service.TransferGroupOwnership(r.Context(), userID, req.TargetUserID); err != nil {
		switch {
		case errors.Is(err, ErrGroupOwnerOnly):
			response.Err(w, http.StatusForbidden, "group_owner_only", err.Error())
		default:
			response.Err(w, http.StatusBadRequest, "transfer_group_ownership_failed", err.Error())
		}
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{"status": "transferred"})
}

func (h *Handler) DeleteFamilyLink(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	linkID := chi.URLParam(r, "link_id")
	if err := h.service.DeleteFamilyLink(r.Context(), userID, linkID); err != nil {
		if errors.Is(err, ErrDeleteFamilyForbidden) {
			response.Err(w, http.StatusForbidden, "delete_forbidden", err.Error())
			return
		}
		response.Err(w, http.StatusBadRequest, "delete_failed", err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

type UpdateMeRequest struct {
	DisplayName   string `json:"display_name"`
	AvatarURL     string `json:"avatar_url"`
	FcmToken      string `json:"fcm_token"`
	ApnsToken     string `json:"apns_token"`
	ApnsVoipToken string `json:"apns_voip_token"`
	Settings      any    `json:"settings"`
}

func (h *Handler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	var req UpdateMeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Err(
			w, http.StatusBadRequest,
			"invalid_body", "Invalid request body",
		)
		return
	}

	user, err := h.service.UpdateProfile(
		r.Context(), userID, req,
	)
	if err != nil {
		response.Err(
			w, http.StatusInternalServerError,
			"update_failed", "Failed to update profile",
		)
		return
	}
	response.JSON(w, http.StatusOK, user)
}
