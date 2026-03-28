# LiveKit Server (Self-Hosted)

## Tổng quan

AiSee sử dụng LiveKit Server tự host để:
- Hội thoại AI realtime (Gemini Live qua Agent Worker)
- Gọi VoIP giữa user và người thân
- Truyền video camera on-demand
- Data channel cho tool actions

LiveKit Server là WebRTC SFU (Selective Forwarding Unit) viết bằng Go, open-source.

## Cài đặt bằng Docker

### Pull image

```bash
docker pull livekit/livekit-server:latest
```

### Tạo API key pair

```bash
docker run --rm livekit/generate generate-keys
# Output:
# API Key: APIxxxxxxxxxx
# API Secret: xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

Lưu key pair vào `.env`.

## Ports cần mở

| Port | Protocol | Mục đích |
|---|---|---|
| 443 | TCP | HTTPS + WebSocket (signaling) |
| 80 | TCP | HTTP (redirect to HTTPS) |
| 7881 | TCP | WebRTC over TCP (fallback khi UDP bị block) |
| 3478 | UDP | TURN server (NAT traversal) |
| 50000-60000 | UDP | WebRTC media (audio/video RTP) |

### Firewall setup

```bash
sudo ufw allow 443/tcp
sudo ufw allow 80/tcp
sudo ufw allow 7881/tcp
sudo ufw allow 3478/udp
sudo ufw allow 50000:60000/udp
```

UDP ports 50000-60000 bắt buộc cho audio/video realtime. Nếu firewall chặn UDP, LiveKit fallback sang TCP (port 7881) nhưng latency tăng.

## Configuration (livekit.yaml)

```yaml
# livekit/livekit.yaml

port: 7881
rtc:
  # Dải port cho WebRTC media
  port_range_start: 50000
  port_range_end: 60000
  # Sử dụng UDP ưu tiên
  use_external_ip: true
  # Enable TCP fallback
  tcp_port: 7881

# SSL/TLS
turn:
  enabled: true
  domain: livekit.aisee.vn
  tls_port: 443
  udp_port: 3478
  external_tls: true

# API keys
keys:
  APIxxxxxxxxxx: <api-secret-here>

# Logging
logging:
  level: info
  json: true

# Room settings
room:
  # Tự động xóa room khi không còn participant
  empty_timeout: 300  # 5 phút
  # Tối đa participant per room
  max_participants: 10

# Webhook (thông báo Go Backend khi có event)
webhook:
  urls:
    - https://api.aisee.vn/api/internal/livekit-webhook
  api_key: APIxxxxxxxxxx
```

### SSL cho LiveKit

LiveKit hỗ trợ 2 cách SSL:

**Cách 1: LiveKit tự quản lý (khuyến nghị cho production)**

```yaml
turn:
  enabled: true
  domain: livekit.aisee.vn
  tls_port: 443
  cert_file: /etc/livekit/cert.pem
  key_file: /etc/livekit/key.pem
```

**Cách 2: Nginx reverse proxy**

Nếu LiveKit chạy chung VPS với Nginx, để Nginx xử lý SSL:

```yaml
port: 7881
rtc:
  use_external_ip: true
```

Nginx forward WebSocket tới port 7881.

## Agent Worker Registration

Agent Worker kết nối tới LiveKit Server và tự động dispatch vào room khi có participant mới.

### Cấu hình Agent Worker

```python
# agent_worker/main.py
from livekit.agents import cli, WorkerOptions

if __name__ == "__main__":
    cli.run_app(
        WorkerOptions(
            entrypoint_fnc=entrypoint,
            # Agent tự động join room khi có user
            auto_subscribe=True,
        ),
    )
```

### Docker Compose

```yaml
agent-worker:
  build: ./agent_worker
  environment:
    - LIVEKIT_URL=ws://livekit:7881  # Internal URL (không qua SSL)
    - LIVEKIT_API_KEY=${LIVEKIT_API_KEY}
    - LIVEKIT_API_SECRET=${LIVEKIT_API_SECRET}
    - GOOGLE_API_KEY=${GOOGLE_API_KEY}
  depends_on:
    - livekit
  restart: unless-stopped
```

Agent Worker kết nối qua internal Docker network (ws://livekit:7881), không cần SSL.

### Xác minh Agent Worker đã đăng ký

```bash
# Kiểm tra logs
docker compose logs agent-worker | grep "registered"

# Kiểm tra qua LiveKit CLI
docker run --rm livekit/livekit-server \
  list-agents \
  --url ws://localhost:7881 \
  --api-key $LIVEKIT_API_KEY \
  --api-secret $LIVEKIT_API_SECRET
```

## Benchmark: Audio-Only Server Capacity

### Test scenario

- Audio-only sessions (Gemini conversation)
- Bitrate: ~32 kbps per stream (speech quality)
- 2 participants per room (user + agent)

### Ước tính capacity

| VPS Spec | Concurrent rooms | Concurrent users | Ghi chú |
|---|---|---|---|
| 2 vCPU, 4 GB RAM | 100-150 | 200-300 | Audio-only, SFU overhead thấp |
| 4 vCPU, 8 GB RAM | 300-500 | 600-1000 | Đủ cho MVP scale |

### Yếu tố ảnh hưởng capacity

- **Audio-only** tiêu tốn rất ít CPU so với video. LiveKit SFU forward audio mà không transcode.
- **Bandwidth:** 32 kbps x 2 stream x 150 rooms = ~10 Mbps. VPS 100 Mbps dư sức.
- **Memory:** Mỗi room ~2-5 MB. 150 rooms = ~750 MB.
- **Video on-demand** (camera) tăng CPU/bandwidth đáng kể nhưng chỉ bật tạm thời (30-60 giây).

### Monitoring capacity

```bash
# CPU usage
docker stats livekit

# Room count
docker run --rm livekit/livekit-server \
  list-rooms \
  --url ws://localhost:7881 \
  --api-key $LIVEKIT_API_KEY \
  --api-secret $LIVEKIT_API_SECRET
```

### Khi nào cần scale

- CPU > 70% sustained -> thêm vCPU hoặc tách LiveKit sang VPS riêng
- Memory > 80% -> tăng RAM
- Concurrent rooms > 80% capacity ước tính -> cân nhắc LiveKit Cloud hoặc thêm server

LiveKit hỗ trợ multi-node cluster (post-MVP) nếu cần horizontal scaling.
