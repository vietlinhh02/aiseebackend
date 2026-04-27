package music

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestAvailable(t *testing.T) {
	result := Available()
	t.Logf("yt-dlp available: %v", result)
}

func TestSearch_Timeout(t *testing.T) {
	s := &Service{
		timeout: 1,
		runner: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}
	_, err := s.Search(context.Background(), "test")
	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestSearch_PrefersM4AAndParsesTitleURL(t *testing.T) {
	var gotArgs []string
	s := &Service{
		timeout: time.Second,
		runner: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			gotArgs = append([]string{name}, args...)
			return []byte("Song Title\nhttps://example.test/song.m4a\n"), nil
		},
	}

	result, err := s.Search(context.Background(), "nhac test")
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}

	if result.Title != "Song Title" || result.URL != "https://example.test/song.m4a" {
		t.Fatalf("unexpected result: %+v", result)
	}
	joined := strings.Join(gotArgs, " ")
	if !strings.Contains(joined, "bestaudio[ext=m4a]") {
		t.Fatalf("expected m4a-preferring format args, got %q", joined)
	}
	if !strings.Contains(joined, "ytsearch1:nhac test") {
		t.Fatalf("expected ytsearch query arg, got %q", joined)
	}
}

func TestSearch_ReturnsBackendStreamURLWhenBaseURLConfigured(t *testing.T) {
	s := &Service{
		timeout:       time.Second,
		streamBaseURL: "https://data.visme.tech",
		streams:       make(map[string]streamEntry),
		runner: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("Song Title\nhttps://rr.example.test/source.m4a\n"), nil
		},
	}

	result, err := s.Search(context.Background(), "song")
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}

	if !strings.HasPrefix(result.URL, "https://data.visme.tech/api/v1/music/stream/") {
		t.Fatalf("expected backend stream URL, got %q", result.URL)
	}
	if len(s.streams) != 1 {
		t.Fatalf("expected one cached stream, got %d", len(s.streams))
	}
}

func TestSearch_ReturnsCommandError(t *testing.T) {
	s := &Service{
		timeout: time.Second,
		runner: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return nil, errors.New("boom")
		},
	}

	_, err := s.Search(context.Background(), "song")
	if err == nil || !strings.Contains(err.Error(), "yt-dlp search failed") {
		t.Fatalf("expected wrapped command error, got %v", err)
	}
}
