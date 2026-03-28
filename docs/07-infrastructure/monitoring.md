# Monitoring

## MVP Monitoring Stack

MVP sử dụng monitoring đơn giản, chi phí $0:

| Component | Tool | Chi phí |
|---|---|---|
| Uptime check | UptimeRobot (free tier) | $0 |
| Health endpoints | `/healthz` (Backend), LiveKit built-in | $0 |
| Logs | Docker stdout + `docker logs` | $0 |
| Alerts | UptimeRobot email/Telegram | $0 |

## /healthz Endpoint

Go Backend expose health check endpoint:

```
GET /healthz

Response 200:
{
  "status": "ok",
  "checks": {
    "database": "ok",
    "redis": "ok"
  }
}

Response 503:
{
  "status": "unhealthy",
  "checks": {
    "database": "ok",
    "redis": "error: connection refused"
  }
}
```

### Logic kiểm tra

Backend kiểm tra khi nhận request `/healthz`:

1. **PostgreSQL** -- `SELECT 1` query, timeout 3 giây
2. **Redis** -- `PING` command, timeout 3 giây

Nếu bất kỳ check nào fail, trả về HTTP 503 Service Unavailable.

### Health check cho Docker Compose

```yaml
backend:
  healthcheck:
    test: ["CMD", "curl", "-f", "http://localhost:8080/healthz"]
    interval: 30s
    timeout: 10s
    retries: 3
    start_period: 10s
```

## UptimeRobot Setup

### 1. Tạo account

Đăng ký tại [uptimerobot.com](https://uptimerobot.com) (free tier: 50 monitors, 5 phút interval).

### 2. Thêm monitors

| Monitor | URL | Type | Interval |
|---|---|---|---|
| Backend API | `https://api.aisee.vn/healthz` | HTTP(s) | 5 phút |
| LiveKit Server | `https://livekit.aisee.vn` | Port (443) | 5 phút |

### 3. Cấu hình alerts

**Email:**
- Thêm email team vào Alert Contacts
- Nhận email khi monitor down và khi recover

**Telegram (khuyến nghị):**
1. Tạo Telegram bot qua @BotFather
2. Lấy chat ID của group/channel
3. Thêm Telegram Alert Contact trong UptimeRobot
4. Nhận alert realtime trên Telegram

### 4. Status page (tùy chọn)

UptimeRobot cung cấp public status page miễn phí:
- URL: `https://stats.uptimerobot.com/aisee`
- Hiển thị uptime percentage, response time chart
- Chia sẻ cho team và stakeholders

## Docker Logs

### Xem logs

```bash
# Tất cả services
docker compose logs -f

# Service cụ thể
docker compose logs -f backend
docker compose logs -f agent-worker
docker compose logs -f livekit
docker compose logs -f postgres

# Giới hạn số dòng
docker compose logs --tail=100 backend

# Logs trong khoảng thời gian
docker compose logs --since="2026-03-28T10:00:00" backend
```

### Log format

Tất cả services log JSON ra stdout. Docker capture tự động.

Go Backend (slog):
```json
{
  "time": "2026-03-28T10:30:00Z",
  "level": "INFO",
  "msg": "request completed",
  "method": "POST",
  "path": "/api/v1/calls/token",
  "status": 200,
  "duration_ms": 12
}
```

Agent Worker (Python logging):
```json
{
  "time": "2026-03-28T10:30:01Z",
  "level": "INFO",
  "msg": "tool_call executed",
  "tool": "make_voice_call",
  "user_id": "uuid",
  "duration_ms": 150
}
```

### Log rotation

Docker mặc định không giới hạn log size. Cấu hình log rotation:

```yaml
# docker-compose.yml
services:
  backend:
    logging:
      driver: json-file
      options:
        max-size: "50m"
        max-file: "5"
```

Mỗi service giữ tối đa 5 file x 50MB = 250MB logs.

### Tìm lỗi trong logs

```bash
# Lọc ERROR level
docker compose logs backend | grep '"level":"ERROR"'

# Tìm tool call failures
docker compose logs agent-worker | grep '"level":"ERROR"'

# Tìm database errors
docker compose logs backend | grep "database"
```

## Post-MVP: Grafana + Loki

Khi scale lên 100+ users, chuyển sang Grafana + Loki cho log aggregation và dashboard.

### Architecture

```
Docker containers (stdout logs)
    │
    ▼
Promtail (log shipper, chạy trên mỗi VPS)
    │
    ▼
Loki (log storage + query engine)
    │
    ▼
Grafana (dashboard + alerts)
```

### Docker Compose bổ sung

```yaml
# docker-compose.monitoring.yml

services:
  loki:
    image: grafana/loki:latest
    ports:
      - "3100:3100"
    volumes:
      - loki_data:/loki
    restart: unless-stopped

  promtail:
    image: grafana/promtail:latest
    volumes:
      - /var/log:/var/log
      - /var/lib/docker/containers:/var/lib/docker/containers:ro
    restart: unless-stopped

  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    volumes:
      - grafana_data:/var/lib/grafana
    environment:
      GF_SECURITY_ADMIN_PASSWORD: ${GRAFANA_PASSWORD}
    restart: unless-stopped

volumes:
  loki_data:
  grafana_data:
```

### Dashboards cần thiết

| Dashboard | Metrics |
|---|---|
| API Overview | Request rate, error rate, latency p50/p95/p99 |
| LiveKit Rooms | Active rooms, participants, audio/video bandwidth |
| Agent Worker | Tool call rate, success rate, Gemini session duration |
| Health | CPU, memory, disk usage per container |
| Business | Active users, calls/day, messages/day, SOS events |

### Alerts cần thiết

| Alert | Điều kiện | Severity |
|---|---|---|
| API down | `/healthz` fail > 2 phút | Critical |
| High error rate | Error rate > 5% trong 5 phút | Warning |
| High latency | p95 latency > 2 giây trong 5 phút | Warning |
| Disk full | Disk usage > 85% | Warning |
| Agent Worker crash | Container restart > 3 lần trong 10 phút | Critical |
| LiveKit overload | CPU > 80% sustained 10 phút | Warning |
