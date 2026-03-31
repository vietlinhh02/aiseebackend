# File Upload Endpoint Design

**Date:** 2026-03-30
**Status:** Approved

## Problem

Backend accepts `voice_url` in chat messages but has no upload mechanism. The Flutter app needs an endpoint to upload voice messages and images, receiving a URL to pass to `POST /api/v1/messages/send`.

## Decision

Add `POST /api/v1/upload` with MinIO (S3-compatible) storage. MinIO runs locally in docker-compose; swappable to AWS S3 or GCS in production via env vars.

## Endpoint

- **Route:** `POST /api/v1/upload`
- **Auth:** JWT (same as other protected routes)
- **Content-Type:** `multipart/form-data`
- **Form field:** `file`
- **Max size:** 10MB
- **Allowed MIME types:** `audio/*`, `image/jpeg`, `image/png`, `image/webp`

### Request

```
POST /api/v1/upload
Authorization: Bearer <token>
Content-Type: multipart/form-data

--boundary
Content-Disposition: form-data; name="file"; filename="voice.m4a"
Content-Type: audio/mp4
<binary data>
--boundary--
```

### Response

```jsonc
// 200 OK
{ "url": "http://localhost:9000/aisee/uploads/550e8400.../1711806000-voice.m4a" }

// 400 Bad Request
{ "error": { "code": "invalid_file", "message": "File type not allowed" } }

// 413 Request Entity Too Large
{ "error": { "code": "file_too_large", "message": "Max file size is 10MB" } }
```

## Storage Path

```
{bucket}/uploads/{user_id}/{unix_timestamp}-{sanitized_filename}
```

Sanitized filename: lowercase, replace spaces with hyphens, strip non-alphanumeric except `.` and `-`.

## Components

| File | Purpose |
|------|---------|
| `pkg/storage/s3.go` | S3/MinIO client: `Upload(ctx, key, reader, contentType) (string, error)` |
| `internal/upload/handler.go` | Parse multipart, validate MIME + size, call storage, return URL |
| `cmd/server/main.go` | Wire storage client + upload handler + route registration |
| `docker-compose.yml` | Add MinIO service + volume |
| `.env.example` | Add S3_* env vars |

## Infrastructure

### MinIO in docker-compose

```yaml
minio:
  image: minio/minio:latest
  command: server /data --console-address ":9001"
  ports:
    - "9000:9000"   # S3 API
    - "9001:9001"   # Web console
  environment:
    MINIO_ROOT_USER: minioadmin
    MINIO_ROOT_PASSWORD: minioadmin
  volumes:
    - minio_data:/data
```

### Environment Variables

```
S3_ENDPOINT=localhost:9000
S3_ACCESS_KEY=minioadmin
S3_SECRET_KEY=minioadmin
S3_BUCKET=aisee
S3_USE_SSL=false
```

## Validation Rules

1. File size <= 10MB (enforced via `http.MaxBytesReader`)
2. MIME type must match allowed list (detect from content, not just extension)
3. User must be authenticated (JWT middleware)

## S3 Client Interface

```go
type Client struct {
    client *minio.Client
    bucket string
}

func (c *Client) Upload(
    ctx context.Context,
    key string,
    reader io.Reader,
    size int64,
    contentType string,
) (string, error)
```

Returns the full public URL of the uploaded object.

## Bucket Setup

On startup, auto-create bucket if not exists and set public read policy. This allows direct URL access to uploaded files without presigned URLs.

## Production Migration

To switch from MinIO to AWS S3:
1. Change `S3_ENDPOINT` to `s3.amazonaws.com`
2. Set `S3_USE_SSL=true`
3. Update `S3_ACCESS_KEY` / `S3_SECRET_KEY` to AWS credentials
4. Remove MinIO container

No code changes required — minio-go SDK is S3-compatible.
