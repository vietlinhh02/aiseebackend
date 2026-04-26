package response

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error Error `json:"error"`
}

func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("encode json response", "error", err)
	}
}

func Err(w http.ResponseWriter, status int, code, message string) {
	JSON(w, status, ErrorResponse{
		Error: Error{Code: code, Message: message},
	})
}
