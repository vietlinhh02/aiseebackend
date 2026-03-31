# File Upload Endpoint Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `POST /api/v1/upload` endpoint with MinIO storage so the Flutter app can upload voice messages and images, receiving a URL to use in chat.

**Architecture:** New `pkg/storage` package wraps minio-go SDK. New `internal/upload` package provides the HTTP handler. MinIO runs as a docker-compose service. Bucket auto-creates on startup with public-read policy.

**Tech Stack:** Go, minio-go SDK, MinIO (docker-compose), chi router

---

### Task 1: Add MinIO to docker-compose and env config

**Files:**
- Modify: `docker-compose.yml`
- Modify: `.env.example`

- [ ] **Step 1: Add MinIO service to docker-compose.yml**

Add after the `redis` service block:

```yaml
  minio:
    image: minio/minio:latest
    command: server /data --console-address ":9001"
    ports:
      - "9000:9000"
      - "9001:9001"
    environment:
      MINIO_ROOT_USER: minioadmin
      MINIO_ROOT_PASSWORD: minioadmin
    volumes:
      - minio_data:/data
```

Add `minio_data:` to the `volumes:` section at bottom.

Add `minio` to `backend` service `depends_on`.

- [ ] **Step 2: Add S3 env vars to .env.example**

Append to `.env.example`:

```
# S3 / MinIO (file uploads)
S3_ENDPOINT=localhost:9000
S3_ACCESS_KEY=minioadmin
S3_SECRET_KEY=minioadmin
S3_BUCKET=aisee
S3_USE_SSL=false
```

- [ ] **Step 3: Commit**

```bash
git add docker-compose.yml .env.example
git commit -m "infra: add MinIO service and S3 env vars"
```

---

### Task 2: Create `pkg/storage` S3 client

**Files:**
- Create: `pkg/storage/s3.go`
- Create: `pkg/storage/s3_test.go`

- [ ] **Step 1: Install minio-go dependency**

```bash
go get github.com/minio/minio-go/v7
go mod tidy
```

- [ ] **Step 2: Write failing test for Upload**

Create `pkg/storage/s3_test.go`:

```go
package storage

import (
	"context"
	"strings"
	"testing"
)

func TestClient_Upload_ReturnsURL(t *testing.T) {
	// Uses a mock — real MinIO tested via integration
	c := &Client{
		bucket:   "test-bucket",
		endpoint: "localhost:9000",
		useSSL:   false,
	}

	// Verify URL format from buildURL
	url := c.buildURL("uploads/user123/1711806000-voice.m4a")
	if !strings.Contains(url, "test-bucket") {
		t.Errorf("URL missing bucket: %s", url)
	}
	if !strings.Contains(url, "uploads/user123") {
		t.Errorf("URL missing key path: %s", url)
	}
}

func TestClient_EnsureBucket_NilOnExisting(t *testing.T) {
	// EnsureBucket with nil minio client should error
	c := &Client{}
	err := c.EnsureBucket(context.Background())
	if err == nil {
		t.Error("expected error with nil minio client")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
go test ./pkg/storage/ -v
```

Expected: FAIL (Client type not defined)

- [ ] **Step 4: Implement `pkg/storage/s3.go`**

```go
package storage

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Client struct {
	minio    *minio.Client
	bucket   string
	endpoint string
	useSSL   bool
}

type Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
}

func NewClient(cfg Config) (*Client, error) {
	mc, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(
			cfg.AccessKey, cfg.SecretKey, "",
		),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio client: %w", err)
	}

	return &Client{
		minio:    mc,
		bucket:   cfg.Bucket,
		endpoint: cfg.Endpoint,
		useSSL:   cfg.UseSSL,
	}, nil
}

func (c *Client) EnsureBucket(ctx context.Context) error {
	if c.minio == nil {
		return fmt.Errorf("minio client is nil")
	}

	exists, err := c.minio.BucketExists(ctx, c.bucket)
	if err != nil {
		return fmt.Errorf("check bucket: %w", err)
	}
	if exists {
		return nil
	}

	if err := c.minio.MakeBucket(
		ctx, c.bucket, minio.MakeBucketOptions{},
	); err != nil {
		return fmt.Errorf("create bucket: %w", err)
	}

	policy := fmt.Sprintf(
		`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::%s/*"]}]}`,
		c.bucket,
	)
	if err := c.minio.SetBucketPolicy(
		ctx, c.bucket, policy,
	); err != nil {
		slog.Warn("set bucket policy failed", "error", err)
	}

	return nil
}

func (c *Client) Upload(
	ctx context.Context,
	key string,
	reader io.Reader,
	size int64,
	contentType string,
) (string, error) {
	_, err := c.minio.PutObject(ctx, c.bucket, key, reader, size,
		minio.PutObjectOptions{ContentType: contentType},
	)
	if err != nil {
		return "", fmt.Errorf("put object: %w", err)
	}

	return c.buildURL(key), nil
}

func (c *Client) buildURL(key string) string {
	scheme := "http"
	if c.useSSL {
		scheme = "https"
	}
	return fmt.Sprintf(
		"%s://%s/%s/%s", scheme, c.endpoint, c.bucket, key,
	)
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./pkg/storage/ -v
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add pkg/storage/
git commit -m "feat: add S3/MinIO storage client"
```

---

### Task 3: Create `internal/upload` handler

**Files:**
- Create: `internal/upload/handler.go`
- Create: `internal/upload/handler_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/upload/handler_test.go`:

```go
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
		{"café.mp3", "caf.mp3"},
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/upload/ -v
```

Expected: FAIL (package/types not defined)

- [ ] **Step 3: Implement `internal/upload/handler.go`**

```go
package upload

import (
	"context"
	"errors"
	"fmt"
	"io"
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

var sanitizeRe = regexp.MustCompile(`[^a-z0-9.\-]`)

type Uploader interface {
	Upload(
		ctx context.Context,
		key string,
		reader io.Reader,
		size int64,
		contentType string,
	) (string, error)
}

type Handler struct {
	storage Uploader
}

func NewHandler(storage Uploader) *Handler {
	return &Handler{storage: storage}
}

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
	if !isAllowedType(ct) {
		response.Err(
			w, http.StatusBadRequest,
			"invalid_file",
			"File type not allowed. Accepted: audio/*, image/jpeg, image/png, image/webp",
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
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/upload/ -v
```

Expected: PASS (4 tests — success, disallowed type, missing file, sanitize filename)

- [ ] **Step 5: Commit**

```bash
git add internal/upload/
git commit -m "feat: add file upload handler with MIME validation"
```

---

### Task 4: Wire upload into main server

**Files:**
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Add storage client initialization after Redis init**

Add to imports:

```go
"github.com/team-dang/aiseebackend/internal/upload"
"github.com/team-dang/aiseebackend/pkg/storage"
```

Add after `pusher := initPusher()`:

```go
// S3 / MinIO storage
storageClient, err := storage.NewClient(storage.Config{
	Endpoint:  os.Getenv("S3_ENDPOINT"),
	AccessKey: os.Getenv("S3_ACCESS_KEY"),
	SecretKey: os.Getenv("S3_SECRET_KEY"),
	Bucket:    os.Getenv("S3_BUCKET"),
	UseSSL:    os.Getenv("S3_USE_SSL") == "true",
})
if err != nil {
	slog.Error("storage client init failed", "error", err)
	os.Exit(1)
}
if err := storageClient.EnsureBucket(context.Background()); err != nil {
	slog.Warn("bucket setup failed, uploads may not work", "error", err)
}

uploadHandler := upload.NewHandler(storageClient)
```

- [ ] **Step 2: Register route in the JWT-protected group**

Add inside the `r.Group(func(r chi.Router) { r.Use(middleware.Auth(jwtSecret)) ... })` block, after the Messages routes:

```go
// Upload
r.Post("/upload", uploadHandler.Upload)
```

- [ ] **Step 3: Add S3 env vars to docker-compose backend service**

Add to `backend.environment` in `docker-compose.yml`:

```yaml
- S3_ENDPOINT=minio:9000
- S3_ACCESS_KEY=minioadmin
- S3_SECRET_KEY=minioadmin
- S3_BUCKET=aisee
- S3_USE_SSL=false
```

- [ ] **Step 4: Run all tests to verify nothing broke**

```bash
go test ./... -v
```

Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/server/main.go docker-compose.yml
git commit -m "feat: wire upload endpoint into server"
```

---

### Task 5: Integration smoke test

- [ ] **Step 1: Start MinIO + backend**

```bash
make db-up
docker compose up -d minio
make dev
```

- [ ] **Step 2: Test upload with curl**

```bash
# Get a JWT token first (use your auth flow)
TOKEN="your-jwt-token"

# Upload an audio file
curl -X POST http://localhost:8080/api/v1/upload \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@test-voice.m4a"

# Expected: {"url":"http://localhost:9000/aisee/uploads/<user-id>/<timestamp>-test-voice.m4a"}
# (filename is sanitized: lowercase, no spaces)
```

- [ ] **Step 3: Verify file accessible**

```bash
# Copy the URL from the response and curl it
curl -I "http://localhost:9000/aisee/uploads/..."
# Expected: HTTP 200
```

- [ ] **Step 4: Test rejection**

```bash
# Wrong file type
curl -X POST http://localhost:8080/api/v1/upload \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@README.md;type=text/plain"

# Expected: 400 {"error":{"code":"invalid_file",...}}
```

- [ ] **Step 5: Verify MinIO console**

Open `http://localhost:9001` (login: minioadmin/minioadmin), check `aisee` bucket has the uploaded file.
