package storage

import (
	"testing"
)

func TestBuildURL(t *testing.T) {
	cfg := Config{
		Endpoint:       "localhost:9000",
		PublicEndpoint: "https://s3.visme.tech",
		AccessKey:      "minioadmin",
		SecretKey:      "minioadmin",
		Bucket:         "aisee",
		UseSSL:         false,
	}

	client, _ := NewClient(cfg)
	key := "uploads/test-file.mp3"
	expected := "https://s3.visme.tech/aisee/uploads/test-file.mp3"

	got := client.buildURL(key)
	if got != expected {
		t.Errorf("Expected %s, got %s", expected, got)
	} else {
		t.Logf("Success! URL generated: %s", got)
	}
}
