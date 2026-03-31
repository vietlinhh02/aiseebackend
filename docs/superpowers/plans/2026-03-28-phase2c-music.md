# Phase 2c: Music — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add 2 music tools (`play_music` via yt-dlp, `control_music` via data channel) to the AiSee backend.

**Architecture:** `internal/music/` wraps yt-dlp subprocess to extract YouTube audio URLs. Tool-call router handles `play_music`, returning a `play_action` for the agent worker to forward via data channel. `control_music` is data-channel-only (no backend).

**Tech Stack:** Go 1.26+, yt-dlp (system binary), Python 3.12+, livekit-agents

**Spec:** `docs/superpowers/specs/2026-03-28-phase2c-music-design.md`

---

## File Structure

### New files

```
internal/music/
├── service.go       # yt-dlp subprocess wrapper
└── service_test.go  # Tests
```

### Modified files

```
internal/toolcall/handler.go  # Add play_music route + music field
cmd/server/main.go            # Wire music service
agent_worker/tools.py         # 2 new tools + generalize _call_and_forward
```

---

## Task 1: Music Service (`internal/music/`)

**Files:**
- Create: `internal/music/service.go`
- Create: `internal/music/service_test.go`

- [ ] **Step 1: Create `internal/music/service.go`**

```go
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
```

- [ ] **Step 2: Create `internal/music/service_test.go`**

```go
package music

import (
	"context"
	"testing"
)

func TestAvailable(t *testing.T) {
	// Just verify the function runs without panic
	result := Available()
	t.Logf("yt-dlp available: %v", result)
}

func TestSearch_InvalidQuery(t *testing.T) {
	if !Available() {
		t.Skip("yt-dlp not installed")
	}

	s := NewService()
	// Empty query should still work (yt-dlp handles it)
	// Just verify no panic
	_, err := s.Search(context.Background(), "")
	// May succeed or fail — we just test it doesn't panic
	t.Logf("empty query result: err=%v", err)
}

func TestSearch_Timeout(t *testing.T) {
	s := &Service{timeout: 1} // 1 nanosecond — will timeout
	_, err := s.Search(context.Background(), "test")
	if err == nil {
		t.Error("expected timeout error")
	}
}
```

- [ ] **Step 3: Verify build and tests**

```bash
go build ./internal/music/...
go test ./internal/music/... -v
```

- [ ] **Step 4: Commit**

```bash
git add internal/music/
git commit -m "feat: add music service with yt-dlp YouTube audio extraction"
```

---

## Task 2: Tool-Call Router + Wire main.go

**Files:**
- Modify: `internal/toolcall/handler.go`
- Modify: `cmd/server/main.go`

Single commit to avoid broken build.

- [ ] **Step 1: Update `internal/toolcall/handler.go`**

Add import:
```go
"github.com/team-dang/aiseebackend/internal/music"
```

Add field to Handler:
```go
music *music.Service
```

Update NewHandler — add `musicSvc *music.Service` param (after `emergSvc`, before `queries`):
```go
func NewHandler(
	chatSvc *chat.Service,
	callSvc *call.Service,
	userSvc *user.Service,
	medSvc *medication.Service,
	healthSvc *health.Service,
	aptSvc *appointment.Service,
	emergSvc *emergency.Service,
	musicSvc *music.Service,
	queries *dbgen.Queries,
) *Handler {
	return &Handler{
		chat:        chatSvc,
		call:        callSvc,
		user:        userSvc,
		medication:  medSvc,
		health:      healthSvc,
		appointment: aptSvc,
		emergency:   emergSvc,
		music:       musicSvc,
		queries:     queries,
	}
}
```

Add dispatch case after `"escalate_to_family"`:
```go
case "play_music":
	return h.handlePlayMusic(ctx, req)
```

Add handler method:
```go
func (h *Handler) handlePlayMusic(
	ctx context.Context, req ToolCallRequest,
) (any, error) {
	query, _ := req.Params["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}

	result, err := h.music.Search(ctx, query)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"status": "found",
		"title":  result.Title,
		"play_action": map[string]any{
			"action": "play_audio",
			"params": map[string]string{
				"url":   result.URL,
				"title": result.Title,
			},
		},
	}, nil
}
```

- [ ] **Step 2: Update `cmd/server/main.go`**

Add import:
```go
"github.com/team-dang/aiseebackend/internal/music"
```

Add after `emergService`:
```go
	musicService := music.NewService()
```

Update toolHandler:
```go
	toolHandler := toolcall.NewHandler(
		chatService, callService, userService,
		medService, healthService, aptService,
		emergService, musicService,
		queries,
	)
```

- [ ] **Step 3: Verify build**

```bash
go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add internal/toolcall/handler.go cmd/server/main.go
git commit -m "feat: add play_music tool route and wire music service"
```

---

## Task 3: Agent Worker — 2 Music Tools

**Files:**
- Modify: `agent_worker/tools.py`

- [ ] **Step 1: Update docstring**

```python
"""AiSee agent tool definitions — Phase 1 + Phase 2a + Phase 2b + Phase 2c."""
```

- [ ] **Step 2: Generalize `_call_and_forward`**

Replace the existing `_call_and_forward` to handle any `*_action` key (not just `schedule_action`):

```python
async def _call_and_forward(tool_name: str, params: dict) -> dict:
    """Call backend and forward any *_action fields to app via data channel."""
    result = await _backend.tool_call(tool_name, _user_id, params)
    for key in list(result.keys()):
        if key.endswith("_action") and isinstance(result[key], dict):
            action = result.pop(key)
            try:
                await _bridge.send_action(
                    _local_participant,
                    action["action"],
                    action["params"],
                )
            except (DataChannelTimeoutError, Exception) as exc:
                logger.debug("%s forwarding failed: %s", key, exc)
    return result
```

- [ ] **Step 3: Add 2 music tools**

Add after the Phase 2b emergency section:

```python
# --- Music tools (Phase 2c) ---


@function_tool()
async def play_music(query: str) -> dict:
    """Tim va phat nhac theo yeu cau (VD: 'nhac Trinh Cong Son', 'bai hat Diem Xua')."""
    return await _call_and_forward(
        "play_music",
        {"query": query},
    )


@function_tool()
async def control_music(action: str) -> dict:
    """Dieu khien nhac dang phat: pause, resume, stop."""
    try:
        result = await _bridge.send_action(
            _local_participant,
            "control_audio",
            {"command": action},
        )
        return {"status": result.get("status", "ok")}
    except DataChannelTimeoutError:
        return {
            "status": "error",
            "error": "App did not respond.",
        }
```

- [ ] **Step 4: Update ALL_TOOLS**

Add at the end:
```python
    # Phase 2c — Music
    play_music,
    control_music,
```

- [ ] **Step 5: Verify syntax**

```bash
cd agent_worker && python3 -c "import ast; ast.parse(open('tools.py').read()); print('OK')"
```

- [ ] **Step 6: Commit**

```bash
git add agent_worker/tools.py
git commit -m "feat: add play_music and control_music agent tools"
```

---

## Verification

```bash
go build ./...
go test ./... -short
cd agent_worker && python3 -c "import ast; ast.parse(open('tools.py').read()); print('OK')"
```
