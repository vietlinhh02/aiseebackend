# Hướng dẫn Deploy

## Yêu cầu VPS

### Cấu hình tối thiểu (MVP, <50 users)

| Component | CPU | RAM | Disk | Ghi chú |
|---|---|---|---|---|
| VPS chính (Backend + DB + Redis) | 2 vCPU | 4 GB | 40 GB SSD | Chạy Go Backend, PostgreSQL, Redis |
| LiveKit Server + Agent Worker | 2 vCPU | 4 GB | 20 GB SSD | Có thể chung VPS nếu ngân sách hạn chế |

### Cấu hình khuyến nghị (50-200 users)

| Component | CPU | RAM | Disk |
|---|---|---|---|
| VPS chính | 4 vCPU | 8 GB | 80 GB SSD |
| LiveKit Server | 2-4 vCPU | 4 GB | 20 GB SSD |
| Agent Worker | 2 vCPU | 2 GB | 10 GB SSD |

### Nhà cung cấp VPS phù hợp

- **Hetzner** -- Giá tốt cho châu Âu, có datacenter Singapore
- **Vultr** -- Có datacenter Singapore/Tokyo, giá hợp lý
- **DigitalOcean** -- Dễ sử dụng, có datacenter Singapore
- **Contabo** -- Giá rẻ nhất, đủ cho MVP

Ưu tiên datacenter gần Việt Nam (Singapore, Tokyo, Hong Kong) để giảm latency.

## Docker Compose Setup

### Cấu trúc file

```
aiseebackend/
├── docker-compose.yml
├── Dockerfile                  # Go Backend
├── agent_worker/
│   └── Dockerfile             # Python Agent Worker
├── livekit/
│   └── livekit.yaml           # LiveKit server config
└── .env                       # Environment variables
```

### docker-compose.yml

```yaml
services:
  backend:
    build: .
    ports:
      - "8080:8080"
    env_file: .env
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    restart: unless-stopped
    networks:
      - aisee

  postgres:
    image: postgres:16-alpine
    volumes:
      - postgres_data:/var/lib/postgresql/data
    environment:
      POSTGRES_DB: aisee
      POSTGRES_USER: aisee
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U aisee"]
      interval: 5s
      timeout: 5s
      retries: 5
    restart: unless-stopped
    networks:
      - aisee

  redis:
    image: redis:7-alpine
    volumes:
      - redis_data:/var/lib/redis/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 5s
      retries: 5
    restart: unless-stopped
    networks:
      - aisee

  livekit:
    image: livekit/livekit-server:latest
    ports:
      - "7881:7881"
      - "443:443"
      - "80:80"
    volumes:
      - ./livekit/livekit.yaml:/etc/livekit.yaml
    command: ["--config", "/etc/livekit.yaml"]
    restart: unless-stopped
    networks:
      - aisee

  agent-worker:
    build: ./agent_worker
    env_file: .env
    depends_on:
      - livekit
      - backend
    restart: unless-stopped
    networks:
      - aisee

volumes:
  postgres_data:
  redis_data:

networks:
  aisee:
    driver: bridge
```

## Domain + SSL Setup

### 1. Cấu hình DNS

Trỏ domain tới VPS IP:

```
api.aisee.vn     A    <VPS_IP>
livekit.aisee.vn A    <VPS_IP>
```

Nếu dùng chung VPS, cả hai trỏ cùng IP. Nginx reverse proxy phân biệt traffic.

### 2. Nginx reverse proxy

```nginx
# /etc/nginx/sites-available/aisee

server {
    listen 80;
    server_name api.aisee.vn;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    server_name api.aisee.vn;

    ssl_certificate /etc/letsencrypt/live/api.aisee.vn/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/api.aisee.vn/privkey.pem;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location /api/v1/ws/ {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

### 3. SSL với Let's Encrypt

```bash
sudo apt install certbot python3-certbot-nginx
sudo certbot --nginx -d api.aisee.vn -d livekit.aisee.vn
```

Certbot tự động renew certificate. LiveKit xử lý SSL riêng (xem docs/07-infrastructure/livekit-server.md).

## Environment Variables Checklist

File `.env` cần có tất cả biến sau:

```dotenv
# ===== Go Backend =====

# Server
PORT=8080
ENV=production

# Database
DATABASE_URL=postgres://aisee:${POSTGRES_PASSWORD}@postgres:5432/aisee?sslmode=disable
POSTGRES_PASSWORD=<strong-random-password>

# Redis
REDIS_URL=redis://redis:6379

# JWT
JWT_SECRET=<random-256-bit-secret>
JWT_EXPIRY=24h
JWT_REFRESH_EXPIRY=30d

# Google OAuth2
GOOGLE_CLIENT_ID_ANDROID=<from-google-cloud-console>
GOOGLE_CLIENT_ID_IOS=<from-google-cloud-console>

# LiveKit
LIVEKIT_URL=wss://livekit.aisee.vn
LIVEKIT_API_KEY=<from-livekit-config>
LIVEKIT_API_SECRET=<from-livekit-config>

# Push Notifications
APNS_KEY_ID=<apple-key-id>
APNS_TEAM_ID=<apple-team-id>
APNS_KEY_PATH=/etc/secrets/apns-key.p8
FCM_SERVICE_ACCOUNT_PATH=/etc/secrets/fcm-service-account.json

# Mapbox
MAPBOX_ACCESS_TOKEN=<from-mapbox-dashboard>

# Internal API (Agent Worker -> Backend)
INTERNAL_API_KEY=<random-secret>

# ===== Agent Worker =====

# LiveKit (same as above)
LIVEKIT_URL=wss://livekit.aisee.vn
LIVEKIT_API_KEY=<same-key>
LIVEKIT_API_SECRET=<same-secret>

# Gemini
GOOGLE_API_KEY=<gemini-api-key>

# Go Backend (internal)
GO_BACKEND_URL=http://backend:8080/api/internal
GO_BACKEND_API_KEY=<same-as-INTERNAL_API_KEY>
```

## First Deploy Steps

### 1. Chuẩn bị VPS

```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Install Docker
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER

# Install Docker Compose plugin
sudo apt install docker-compose-plugin

# Install Nginx
sudo apt install nginx
```

### 2. Clone repository

```bash
git clone <repo-url> /opt/aisee
cd /opt/aisee
```

### 3. Tạo file .env

```bash
cp .env.example .env
# Điền tất cả giá trị trong .env
nano .env
```

### 4. Chạy database migration

```bash
# Khởi động PostgreSQL trước
docker compose up -d postgres

# Chạy migration
docker compose run --rm backend migrate up
```

### 5. Khởi động tất cả services

```bash
docker compose up -d
```

### 6. Kiểm tra

```bash
# Kiểm tra tất cả container đang chạy
docker compose ps

# Kiểm tra health endpoint
curl https://api.aisee.vn/healthz

# Kiểm tra logs
docker compose logs -f backend
docker compose logs -f agent-worker
docker compose logs -f livekit
```

### 7. Cấu hình firewall

```bash
sudo ufw allow 22/tcp     # SSH
sudo ufw allow 80/tcp     # HTTP (redirect to HTTPS)
sudo ufw allow 443/tcp    # HTTPS
sudo ufw allow 7881/tcp   # LiveKit WebRTC
sudo ufw allow 3478/udp   # TURN server
sudo ufw allow 50000:60000/udp  # WebRTC media
sudo ufw enable
```

### 8. Setup UptimeRobot

1. Tạo account tại uptimerobot.com
2. Thêm monitor cho `https://api.aisee.vn/healthz`
3. Cấu hình alert qua email/Telegram
4. Interval: 5 phút (free tier)
