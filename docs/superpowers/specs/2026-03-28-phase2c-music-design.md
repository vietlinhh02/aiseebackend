# Phase 2c: Music — Technical Design Spec

**Date:** 2026-03-28
**Team:** Dang | VentureX Season 3 | VinUniversity ELAB
**Status:** Draft
**Depends on:** Phase 1 (completed), Phase 2a (completed), Phase 2b (completed)

---

## 1. Goal

Add music playback tools to the AI agent via YouTube audio extraction. After Phase 2c, a user can:

- Ask the AI to play music by voice ("phát nhạc Trịnh Công Sơn")
- Control playback: pause, resume, stop

## 2. Scope

### In scope

- `internal/music/` package: yt-dlp subprocess wrapper for YouTube audio URL extraction
- Tool-call router: 1 new tool route (`play_music`)
- Agent worker: 2 new tool functions (`play_music` via backend, `control_music` via data channel)
- Data channel: `play_audio` and `control_audio` actions

### Out of scope

- `add_to_queue` / queue management (no queue for MVP)
- Radio streams / podcast RSS (deferred)
- Audio file download/caching
- Volume control (app-side UX decision)

## 3. Architecture

```
play_music:
  Agent Worker → POST /api/internal/tool-call {tool: "play_music", params: {query}}
    → Go Backend → yt-dlp subprocess → extract audio URL + title
    → Response includes play_action
  Agent Worker → data channel "play_audio" {url, title} → App plays via just_audio

control_music:
  Agent Worker → data channel "control_audio" {command} → App handles locally
  (no backend involved)
```

## 4. Music Service

### 4.1 File structure

```
internal/music/
├── service.go       # yt-dlp subprocess wrapper
└── service_test.go  # Tests
```

### 4.2 Service

```go
type Result struct {
    Title string
    URL   string
}

type Service struct {
    timeout time.Duration // 15s default
}

func NewService() *Service

func (s *Service) Search(ctx context.Context, query string) (Result, error)
```

### 4.3 Search method

Uses `yt-dlp` subprocess:

```bash
yt-dlp "ytsearch1:{query}" --extract-audio --get-url --get-title --no-warnings -q
```

Output: 2 lines — title on line 1, audio URL on line 2.

Flow:
1. Create `exec.CommandContext` with `s.timeout` context
2. Run command, capture stdout
3. Parse: split by newline, first line = title, second line = URL
4. If exit code != 0 or no output: return error "no results found"
5. Return `Result{Title, URL}`

### 4.4 Error handling

- yt-dlp not installed: `exec.LookPath("yt-dlp")` check at startup, log warning
- Search timeout (15s): context deadline exceeded
- No results: return descriptive error for agent to relay to user
- Network error: yt-dlp exits non-zero, return error

## 5. Tool-Call Router

### 5.1 New handler

```go
func (h *Handler) handlePlayMusic(ctx, req) (any, error)
```

Extracts `query` from params, calls `music.Search`, returns:

```json
{
  "status": "found",
  "title": "Diem Xua - Trinh Cong Son",
  "play_action": {
    "action": "play_audio",
    "params": {
      "url": "https://...",
      "title": "Diem Xua - Trinh Cong Son"
    }
  }
}
```

### 5.2 Handler dependencies

Add `music *music.Service` to Handler struct. Update `NewHandler` signature (9th param before `queries`).

### 5.3 Wiring in main.go

```go
musicService := music.NewService()

toolHandler := toolcall.NewHandler(
    chatService, callService, userService,
    medService, healthService, aptService,
    emergService, musicService,
    queries,
)
```

## 6. Agent Worker

### 6.1 `play_music` tool (backend + data channel)

```python
@function_tool()
async def play_music(query: str) -> dict:
    """Tim va phat nhac theo yeu cau."""
    return await _call_and_forward(
        "play_music",
        {"query": query},
    )
```

Uses `_call_and_forward` to forward `play_action` via data channel (same pattern as `schedule_action`).

Need to update `_call_and_forward` to also handle `play_action` key (not just `schedule_action`). Generalize to forward any `*_action` key.

Updated helper:

```python
async def _call_and_forward(tool_name: str, params: dict) -> dict:
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

### 6.2 `control_music` tool (data channel only)

```python
@function_tool()
async def control_music(
    action: str,
) -> dict:
    """Dieu khien nhac: pause, resume, stop."""
    try:
        result = await _bridge.send_action(
            _local_participant, "control_audio",
            {"command": action},
        )
        return {"status": result.get("status", "ok")}
    except DataChannelTimeoutError:
        return {"status": "error", "error": "App did not respond."}
```

### 6.3 Tool registration

Add both to `ALL_TOOLS`.

## 7. Data Channel Protocol

### `play_audio` action

**Agent → App:**
```json
{
  "type": "tool_action",
  "request_id": "uuid",
  "action": "play_audio",
  "params": {"url": "https://...", "title": "Diem Xua"}
}
```

**App ACK:**
```json
{"type": "tool_action_ack", "request_id": "uuid", "status": "ok"}
```

### `control_audio` action

**Agent → App:**
```json
{
  "type": "tool_action",
  "request_id": "uuid",
  "action": "control_audio",
  "params": {"command": "pause"}
}
```

Valid commands: `pause`, `resume`, `stop`

## 8. Dependencies

### System

`yt-dlp` must be installed on the server. Add to Dockerfile:

```dockerfile
RUN pip install yt-dlp
```

Or download binary:

```dockerfile
RUN curl -L https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp -o /usr/local/bin/yt-dlp && chmod +x /usr/local/bin/yt-dlp
```

### Go / Python

No new Go or Python dependencies.

## 9. Implementation Order

1. `internal/music/` — yt-dlp service + tests
2. `internal/toolcall/handler.go` + `cmd/server/main.go` — add play_music route + wire (single commit)
3. `agent_worker/tools.py` — update `_call_and_forward`, add 2 tools, update ALL_TOOLS
