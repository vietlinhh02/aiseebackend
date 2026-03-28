package music

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type Result struct {
	Title string
	URL   string
}

type Service struct {
	timeout time.Duration
}

func NewService() *Service {
	return &Service{timeout: 15 * time.Second}
}

func (s *Service) Search(
	ctx context.Context, query string,
) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"yt-dlp",
		fmt.Sprintf("ytsearch1:%s", query),
		"--extract-audio",
		"--get-url",
		"--get-title",
		"--no-warnings",
		"-q",
	)

	out, err := cmd.Output()
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
		URL:   strings.TrimSpace(lines[1]),
	}, nil
}

func Available() bool {
	_, err := exec.LookPath("yt-dlp")
	return err == nil
}
