# Agent Worker Setup Guide

## Tổng quan

Agent Worker la mot Python process nhe (~200-500 LOC). Nhiem vu duy nhat: chay Gemini Live qua LiveKit và route tool calls toi Go Backend. Tất cả business logic nam trong Go Backend.

## Prerequisites

| Tool | Version | Ghi chú |
|------|---------|---------|
| Python | 3.12+ | `python3 --version` |
| uv | Latest | `curl -LsSf https://astral.sh/uv/install.sh \| sh` |

## Cài đặt

### 1. Tạo virtual environment và cai dependencies

```bash
cd agent_worker
uv venv
source .venv/bin/activate
uv pip install livekit-agents livekit-plugins-google httpx
```

### 2. Cấu hình environment variables

Tạo file `.env` trong thư mục `agent_worker/`:

```env
# Gemini API
GEMINI_API_KEY=<your-gemini-api-key>

# LiveKit
LIVEKIT_URL=ws://localhost:7880
LIVEKIT_API_KEY=devkey
LIVEKIT_API_SECRET=devsecret

# Go Backend (internal API)
GO_BACKEND_URL=http://localhost:8080/api/internal
INTERNAL_API_KEY=<shared-secret-giong-voi-go-backend>
```

### Giải thích tung bien

| Bien | Mo ta |
|------|-------|
| `GEMINI_API_KEY` | API key tu Google AI Studio. Free tier du cho MVP. |
| `LIVEKIT_URL` | URL của LiveKit Server. Trong Docker: `ws://livekit:7880`. |
| `LIVEKIT_API_KEY` | API key của LiveKit Server (cấu hình trong LiveKit config). |
| `LIVEKIT_API_SECRET` | API secret của LiveKit Server. |
| `GO_BACKEND_URL` | URL của Go Backend internal API. Trong Docker: `http://go-backend:8080/api/internal`. |
| `INTERNAL_API_KEY` | Shared secret de xác thực với Go Backend. Phai giong giá trị trong `.env` của Go Backend. |

## Chạy Agent Worker

### Development

```bash
cd agent_worker
source .venv/bin/activate
python main.py dev
```

Lenh `dev` khởi động Agent Worker o che do development: tự động dispatch agent khi co room mỗi.

### Production

```bash
python main.py start
```

### Kiem tra Agent Worker hoạt động

1. Khoi dong Go Backend và LiveKit Server truoc
2. Khoi dong Agent Worker
3. Mo Flutter app (hoặc dùng LiveKit CLI de tao room)
4. Agent Worker se tự động join room và bật dau Gemini Live session
5. Noi tieng Viết — Gemini se tra lỗi

### Log output

Agent Worker in log ra stdout. Ví dụ:

```
INFO  agent_worker - Connected to LiveKit server
INFO  agent_worker - Joined room: user:550e8400-...
INFO  agent_worker - Gemini Live session started
INFO  agent_worker - Tool call: make_voice_call(contact_name="con gai")
INFO  agent_worker - Tool result: {"status": "room_created", "room_name": "call-..."}
```

## Cấu trúc file

```
agent_worker/
├── main.py              # Entry point, entrypoint function, tool definitions
├── tools.py             # Tool handler functions (optional tach rieng)
├── prompts.py           # System prompt tieng Viet
├── .env                 # Environment variables (git-ignored)
└── pyproject.toml       # Project metadata va dependencies
```

Agent Worker chi co ~200-500 dong code. Khong nen phức tạp hon mục này.

## Troubleshooting

| Vấn đề | Nguyen nhan | Cach xử lý |
|--------|-------------|------------|
| `Connection refused` toi LiveKit | LiveKit Server chưa chay | `docker compose up -d livekit` |
| `Connection refused` toi Go Backend | Backend chưa chay | Khoi dong backend truoc |
| `PERMISSION_DENIED` tu Gemini | API key sai hoặc het quota | Kiem tra `GEMINI_API_KEY` |
| Agent không join room | LiveKit API key/secret sai | Kiem tra `LIVEKIT_API_KEY` và `LIVEKIT_API_SECRET` |
| Tool call thất bại với 401 | `INTERNAL_API_KEY` không khop | Dam bao giong nhau giua Agent và Backend |
| Gemini không noi tieng Viết | System prompt thieu | Kiem tra `prompts.py` co system prompt tieng Viết |
