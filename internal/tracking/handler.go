package tracking

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

func (h *Handler) RecordEvent(w http.ResponseWriter, r *http.Request) {
	var input EventInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Err(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	event, err := h.service.RecordEvent(r.Context(), input)
	if err != nil {
		response.Err(w, http.StatusBadRequest, "tracking_event_invalid", err.Error())
		return
	}
	response.JSON(w, http.StatusCreated, event)
}
