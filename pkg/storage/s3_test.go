package storage

import (
	"context"
	"strings"
	"testing"
)

func TestClient_BuildURL_HTTP(t *testing.T) {
	c := &Client{
		bucket:   "test-bucket",
		endpoint: "localhost:9000",
		useSSL:   false,
	}

	url := c.buildURL("uploads/user123/1711806000-voice.m4a")
	if !strings.Contains(url, "test-bucket") {
		t.Errorf("URL missing bucket: %s", url)
	}
	if !strings.Contains(url, "uploads/user123") {
		t.Errorf("URL missing key path: %s", url)
	}
	if !strings.HasPrefix(url, "http://") {
		t.Errorf("expected http:// prefix: %s", url)
	}
}

func TestClient_BuildURL_HTTPS(t *testing.T) {
	c := &Client{
		bucket:   "prod-bucket",
		endpoint: "s3.amazonaws.com",
		useSSL:   true,
	}

	url := c.buildURL("uploads/user123/file.m4a")
	if !strings.HasPrefix(url, "https://") {
		t.Errorf("expected https:// prefix: %s", url)
	}
}

func TestClient_BuildURL_PublicEndpointWithScheme(t *testing.T) {
	c := &Client{
		bucket:         "aisee",
		endpoint:       "localhost:9000",
		publicEndpoint: "https://s3.visme.tech",
		useSSL:         false,
	}

	url := c.buildURL("uploads/user123/file.m4a")
	if url != "https://s3.visme.tech/aisee/uploads/user123/file.m4a" {
		t.Fatalf("unexpected url: %s", url)
	}
}

func TestClient_EnsureBucket_NilClient(t *testing.T) {
	c := &Client{}
	err := c.EnsureBucket(context.Background())
	if err == nil {
		t.Error("expected error with nil minio client")
	}
}
