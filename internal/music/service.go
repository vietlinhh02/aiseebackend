// Package music proxies audio streams. Usage (music_minutes / music_sessions) is recorded
// when the client calls internal billing APIs: POST /api/internal/usage/session/start and
// POST /api/internal/usage/session/commit with usage_type music_minutes — this service does
// not observe playback duration by itself.
package music

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/team-dang/aiseebackend/pkg/response"
)

type Result struct {
	Title string
	URL   string
}

type Service struct {
	timeout       time.Duration
	runner        commandRunner
	streamBaseURL string
	mu            sync.Mutex
	streams       map[string]streamEntry
}

type streamEntry struct {
	url       string
	expiresAt time.Time
}

func NewService(streamBaseURL ...string) *Service {
	baseURL := ""
	if len(streamBaseURL) > 0 {
		baseURL = strings.TrimRight(strings.TrimSpace(streamBaseURL[0]), "/")
	}
	return &Service{
		timeout:       15 * time.Second,
		runner:        runCommand,
		streamBaseURL: baseURL,
		streams:       make(map[string]streamEntry),
	}
}

type commandRunner func(context.Context, string, ...string) ([]byte, error)

func (s *Service) Search(
	ctx context.Context, query string,
) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	out, err := s.runner(
		ctx,
		"yt-dlp",
		fmt.Sprintf("ytsearch1:%s", query),
		"--format",
		"bestaudio[ext=m4a]/bestaudio[acodec*=mp4a]/bestaudio",
		"--get-title",
		"--get-url",
		"--no-warnings",
		"-q",
	)
	if err != nil {
		return Result{}, fmt.Errorf(
			"yt-dlp search failed: %w", err,
		)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return Result{}, fmt.Errorf(
			"no results found for: %s", query,
		)
	}

	return Result{
		Title: strings.TrimSpace(lines[0]),
		URL:   s.streamURL(strings.TrimSpace(lines[1])),
	}, nil
}

func (s *Service) Stream(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		response.Err(w, http.StatusBadRequest, "missing_stream_id", "missing stream id")
		return
	}

	entry, ok := s.lookupStream(id)
	if !ok {
		response.Err(w, http.StatusNotFound, "stream_not_found", "music stream expired or not found")
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, entry.url, nil)
	if err != nil {
		response.Err(w, http.StatusBadGateway, "stream_error", "invalid upstream music url")
		return
	}
	req.Header.Set(
		"User-Agent",
		"Mozilla/5.0 (Linux; Android 13; Mobile) AppleWebKit/537.36 "+
			"(KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36",
	)
	req.Header.Set("Referer", "https://www.youtube.com/")
	if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Warn("music stream upstream failed", "error", err)
		response.Err(w, http.StatusBadGateway, "stream_error", "failed to open music stream")
		return
	}
	defer resp.Body.Close()

	copyHeader(w, resp.Header, "Content-Type")
	copyHeader(w, resp.Header, "Content-Length")
	copyHeader(w, resp.Header, "Content-Range")
	copyHeader(w, resp.Header, "Accept-Ranges")
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		slog.Warn("music stream copy failed", "error", err)
	}
}

func (s *Service) streamURL(sourceURL string) string {
	if s.streamBaseURL == "" || sourceURL == "" {
		return sourceURL
	}

	id := uuid.NewString()
	expiresAt := time.Now().Add(30 * time.Minute)
	now := time.Now()
	s.mu.Lock()
	s.cleanupExpiredLocked(now)
	s.streams[id] = streamEntry{url: sourceURL, expiresAt: expiresAt}
	s.mu.Unlock()

	return s.streamBaseURL + "/api/v1/music/stream/" + id
}

func (s *Service) lookupStream(id string) (streamEntry, bool) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupExpiredLocked(now)
	entry, ok := s.streams[id]
	if !ok || now.After(entry.expiresAt) {
		delete(s.streams, id)
		return streamEntry{}, false
	}
	return entry, true
}

func (s *Service) cleanupExpiredLocked(now time.Time) {
	for id, entry := range s.streams {
		if now.After(entry.expiresAt) {
			delete(s.streams, id)
		}
	}
}

func copyHeader(w http.ResponseWriter, h http.Header, key string) {
	if value := h.Get(key); value != "" {
		w.Header().Set(key, value)
	}
}

func Available() bool {
	_, err := exec.LookPath("yt-dlp")
	return err == nil
}

func runCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}
