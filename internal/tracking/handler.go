package tracking

import (
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

func (h *Handler) RecordEvent(w http.ResponseWriter, r *http.Request) {
	var input EventInput
	if inputErr := httpinput.DecodeAndValidate(r, &input); inputErr != nil {
		switch {
		case inputErr.Kind == httpinput.ErrorKindValidate && inputErr.Field == "UserID":
			response.Err(w, http.StatusBadRequest, "tracking_event_invalid", "user_id is required")
		case inputErr.Kind == httpinput.ErrorKindValidate && inputErr.Field == "EventType":
			response.Err(w, http.StatusBadRequest, "tracking_event_invalid", "event_type is required")
		case inputErr.Kind == httpinput.ErrorKindValidate && inputErr.Field == "Category":
			response.Err(w, http.StatusBadRequest, "tracking_event_invalid", "category is required")
		case inputErr.Kind == httpinput.ErrorKindValidate && inputErr.Field == "Source":
			response.Err(w, http.StatusBadRequest, "tracking_event_invalid", "source is required")
		case inputErr.Kind == httpinput.ErrorKindValidate && inputErr.Field == "SourceID":
			response.Err(w, http.StatusBadRequest, "tracking_event_invalid", "source_id is required")
		default:
			response.Err(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		}
		return
	}
	event, err := h.service.RecordEvent(r.Context(), input)
	if err != nil {
		response.Err(w, http.StatusBadRequest, "tracking_event_invalid", err.Error())
		return
	}
	response.JSON(w, http.StatusCreated, event)
}
