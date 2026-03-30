package upload

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/team-dang/aiseebackend/internal/middleware"
	"github.com/team-dang/aiseebackend/pkg/response"
)

const maxUploadSize = 10 << 20 // 10MB

var allowedTypes = map[string]bool{
	"audio/mpeg":  true,
	"audio/mp4":   true,
	"audio/mp3":   true,
	"audio/wav":   true,
	"audio/ogg":   true,
	"audio/aac":   true,
	"audio/x-m4a": true,
	"audio/webm":  true,
	"image/jpeg":  true,
	"image/png":   true,
	"image/webp":  true,
}

var sanitizeRe = regexp.MustCompile(`[^a-z0-9\-]`)

// Uploader abstracts object storage so the handler can be
// tested without a real S3/MinIO backend.
type Uploader interface {
	Upload(
		ctx context.Context,
		key string,
		reader io.Reader,
		size int64,
		contentType string,
	) (string, error)
}

// Handler serves file upload requests.
type Handler struct {
	storage Uploader
}

// NewHandler returns an upload handler backed by storage.
func NewHandler(storage Uploader) *Handler {
	return &Handler{storage: storage}
}

// Upload accepts a multipart file upload, validates MIME type,
// and stores it in object storage.
func (h *Handler) Upload(
	w http.ResponseWriter, r *http.Request,
) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			response.Err(
				w, http.StatusRequestEntityTooLarge,
				"file_too_large", "Max file size is 10MB",
			)
		} else {
			response.Err(
				w, http.StatusBadRequest,
				"invalid_body", "Invalid multipart form",
			)
		}
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		response.Err(
			w, http.StatusBadRequest,
			"missing_file", "file field is required",
		)
		return
	}
	defer file.Close()

	ct := header.Header.Get("Content-Type")
	if ct == "" || ct == "application/octet-stream" {
		ct = mime.TypeByExtension(filepath.Ext(header.Filename))
	}
	if !isAllowedType(ct) {
		response.Err(
			w, http.StatusBadRequest,
			"invalid_file",
			"File type not allowed. "+
				"Accepted: audio/*, image/jpeg, "+
				"image/png, image/webp",
		)
		return
	}

	userID := middleware.UserIDFromContext(r.Context())
	sanitized := sanitizeFilename(header.Filename)
	key := fmt.Sprintf(
		"uploads/%s/%d-%s",
		userID, time.Now().Unix(), sanitized,
	)

	url, err := h.storage.Upload(
		r.Context(), key, file, header.Size, ct,
	)
	if err != nil {
		response.Err(
			w, http.StatusInternalServerError,
			"upload_failed", "Failed to upload file",
		)
		return
	}

	response.JSON(
		w, http.StatusOK, map[string]string{"url": url},
	)
}

func isAllowedType(ct string) bool {
	if allowedTypes[ct] {
		return true
	}
	return strings.HasPrefix(ct, "audio/")
}

func sanitizeFilename(name string) string {
	name = strings.ToLower(name)
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	base = strings.ReplaceAll(base, " ", "-")
	base = sanitizeRe.ReplaceAllString(base, "")
	if base == "" {
		base = "file"
	}
	return base + ext
}
