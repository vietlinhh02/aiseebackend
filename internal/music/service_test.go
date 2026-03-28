package music

import (
	"context"
	"testing"
)

func TestAvailable(t *testing.T) {
	result := Available()
	t.Logf("yt-dlp available: %v", result)
}

func TestSearch_Timeout(t *testing.T) {
	s := &Service{timeout: 1}
	_, err := s.Search(context.Background(), "test")
	if err == nil {
		t.Error("expected timeout error")
	}
}
