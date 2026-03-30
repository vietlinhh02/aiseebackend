package upload

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/team-dang/aiseebackend/internal/middleware"
)

type mockStorage struct {
	url string
	err error
}

func (m *mockStorage) Upload(
	_ context.Context,
	key string,
	reader io.Reader,
	size int64,
	contentType string,
) (string, error) {
	return m.url, m.err
}

func createMultipartRequest(
	t *testing.T, fieldName, filename, contentType string,
	content []byte,
) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile(fieldName, filename)
	if err != nil {
		t.Fatal(err)
	}
	part.Write(content)
	w.Close()

	req := httptest.NewRequest(
		http.MethodPost, "/api/v1/upload", &buf,
	)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func TestUpload_Success_Audio(t *testing.T) {
	store := &mockStorage{url: "http://minio:9000/aisee/test.m4a"}
	h := NewHandler(store)

	req := createMultipartRequest(
		t, "file", "voice.m4a", "audio/mp4",
		[]byte("fake audio data"),
	)
	req = req.WithContext(
		middleware.WithUserID(req.Context(), "user-123"),
	)

	rr := httptest.NewRecorder()
	h.Upload(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s",
			rr.Code, rr.Body.String())
	}

	var resp map[string]string
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["url"] == "" {
		t.Error("expected url in response")
	}
}

func TestUpload_RejectsDisallowedType(t *testing.T) {
	store := &mockStorage{}
	h := NewHandler(store)

	req := createMultipartRequest(
		t, "file", "malware.exe", "application/octet-stream",
		[]byte("bad data"),
	)
	req = req.WithContext(
		middleware.WithUserID(req.Context(), "user-123"),
	)

	rr := httptest.NewRecorder()
	h.Upload(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestUpload_RejectsMissingFile(t *testing.T) {
	store := &mockStorage{}
	h := NewHandler(store)

	req := httptest.NewRequest(
		http.MethodPost, "/api/v1/upload", nil,
	)
	req.Header.Set(
		"Content-Type",
		"multipart/form-data; boundary=xxx",
	)
	req = req.WithContext(
		middleware.WithUserID(req.Context(), "user-123"),
	)

	rr := httptest.NewRecorder()
	h.Upload(rr, req)

	// ParseMultipartForm fails on nil body → 400
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Voice Message.m4a", "voice-message.m4a"},
		{"hello world.PNG", "hello-world.png"},
		{"../../etc/passwd", "etcpasswd"},
		{"", "file"},
	}
	for _, tt := range tests {
		got := sanitizeFilename(tt.input)
		if got != tt.want {
			t.Errorf(
				"sanitizeFilename(%q) = %q, want %q",
				tt.input, got, tt.want,
			)
		}
	}
}
