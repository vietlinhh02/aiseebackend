# Hướng dẫn Cấu hình Deploy trên Coolify với Docker Image (GHCR)

Tài liệu này hướng dẫn cách deploy hệ thống **Aisee Backend** và **Aisee Agent** lên Coolify bằng cách kéo trực tiếp các Docker Image đã được build sẵn từ GitHub Container Registry (GHCR) thông qua GitHub Actions, cùng với việc triển khai các dịch vụ bổ trợ (MinIO, Postgres, Redis).

> [!NOTE]
> Bằng cách sử dụng phương pháp này, Coolify sẽ không cần tốn tài nguyên (CPU/RAM) để build source code nữa. Khi bạn push code lên branch `main`, GitHub Actions sẽ tự động chạy test, build image và push lên GHCR. Coolify chỉ việc kéo image về và chạy.

---

## 1. Chuẩn bị Registry trên Coolify (Vì Repo Private)

Vì repository của bạn là **private**, các Docker image được push lên GHCR cũng sẽ là **private**. Bạn cần cấu hình thông tin đăng nhập GHCR trên Coolify:

1. Truy cập vào **Keys & Sources** > **Registries** > **Add**.
2. Điền các thông tin sau:
   - **Name**: `GitHub Container Registry` (hoặc tên bất kỳ)
   - **Registry URL**: `ghcr.io`
   - **Username**: `<GitHub Username của bạn>` (ví dụ: `vietlinhh02`)
   - **Password / Token**: `<GitHub Personal Access Token (PAT) có quyền read:packages>`

> [!IMPORTANT]
> **Cách tạo GitHub PAT:** Vào **GitHub Settings** > **Developer Settings** > **Personal Access Tokens (Classic)** > **Generate new token**. Chọn duy nhất scope `read:packages` (đọc package).

---

## 2. Hướng dẫn Deploy MinIO (S3 Object Storage) trên Coolify

Backend sử dụng MinIO để lưu trữ file (ghi âm, avatar, tài liệu). Có 2 cách để deploy MinIO trên Coolify:

### Cách 1: Sử dụng Template có sẵn của Coolify (Khuyên dùng)
1. Trong Project của bạn, chọn **New Resource** > **Service** (hoặc **Service Templates**).
2. Tìm kiếm **MinIO** và chọn nó.
3. Cấu hình các thông số:
   - **Console Username (Admin User)**: `minioadmin` (hoặc tên bất kỳ bạn muốn)
   - **Console Password (Admin Password)**: `minioadmin` (nên đổi sang mật khẩu mạnh hơn)
   - **Public Console Domain**: Gán domain nếu bạn muốn truy cập giao diện quản trị ngoài internet (ví dụ: `https://storage-admin.aisee.me`).
   - **Public S3 API Domain**: Gán domain nếu bạn muốn các ứng dụng bên ngoài truy cập file trực tiếp (ví dụ: `https://storage.aisee.me`).

### Cách 2: Deploy dưới dạng Docker Image thông thường
Nếu bạn muốn cấu hình thủ công như file `docker-compose.yml`:
1. Chọn **New Resource** > **Docker Image**.
2. Điền Image: `quay.io/minio/minio:latest` (hoặc `minio/minio:latest`).
3. Trong phần **Start Command**, điền: `server /data --console-address ":9001"`
4. Cấu hình **Exposed Ports**:
   - Port `9000` (API)
   - Port `9001` (Console Web UI)
5. Cấu hình **Volumes**:
   - Source: `minio_data`
   - Destination: `/data`
6. Điền các **Environment Variables**:
   ```env
   MINIO_ROOT_USER=minioadmin
   MINIO_ROOT_PASSWORD=minioadmin
   ```

> [!WARNING]
> **Tạo Bucket:** Sau khi deploy MinIO thành công, bạn hãy truy cập vào Console (Port `9001`), đăng nhập bằng tài khoản admin và tạo một Bucket tên là `aisee` (hoặc trùng với cấu hình `S3_BUCKET` ở Backend) thì hệ thống mới lưu được file.

---

## 3. Hướng dẫn Deploy Postgres & Redis trên Coolify

### Deploy Postgres
1. Chọn **New Resource** > **Database** > **PostgreSQL**.
2. Coolify sẽ tự sinh thông tin kết nối và tạo database.
3. Ghi lại **Internal Connection String** (dạng `postgresql://postgres:...`) để điền vào biến `DATABASE_URL` của Backend.

### Deploy Redis
1. Chọn **New Resource** > **Database** > **Redis**.
2. Ghi lại **Internal Connection String** (dạng `redis://...`) để điền vào biến `REDIS_URL` của Backend.

---

## 4. Deploy Service 1: Aisee Backend

### Cấu hình cơ bản
- **Service Type**: Chọn `Docker Image`
- **Docker Image**: `ghcr.io/vietlinhh02/aiseebackend:main`
- **Exposed Port**: `8080`
- **Domains**: Nhập domain public của bạn (ví dụ: `https://api.aisee.me`)

### Environment Variables chính cần set trên Coolify:
```env
# Kết nối Postgres & Redis (Sử dụng thông tin kết nối nội bộ lấy từ bước 3)
DATABASE_URL=postgres://<user>:<password>@<postgres-internal-host>:5432/<db_name>?sslmode=disable
REDIS_URL=redis://<redis-internal-host>:6379/0

# Kết nối LiveKit
LIVEKIT_URL=https://<your-livekit-domain>
LIVEKIT_API_KEY=<your-livekit-api-key>
LIVEKIT_API_SECRET=<your-livekit-api-secret>

# Kết nối với Agent Worker (Sử dụng Internal FQDN của Coolify hoặc Private IP)
# Ví dụ: ws://aisee-agent:8765/ws
ASSISTANT_BRIDGE_URL=ws://<agent-service-internal-fqdn-or-ip>:8765/ws

# Cấu hình kết nối MinIO (Lấy từ bước 2)
# Sử dụng Internal FQDN của MinIO để tối ưu hóa tốc độ và bảo mật
S3_ENDPOINT=<minio-internal-fqdn-or-ip>:9000
S3_ACCESS_KEY=minioadmin
S3_SECRET_KEY=minioadmin
S3_BUCKET=aisee
S3_USE_SSL=false
```

---

## 5. Deploy Service 2: Aisee Agent Worker

### Cấu hình cơ bản
- **Service Type**: Chọn `Docker Image`
- **Docker Image**: `ghcr.io/vietlinhh02/aiseebackend-agent:main`
- **Exposed Port**: `8765`
- **Domains**: Không cần gán domain public (hoặc gán nếu cần thiết).

### Environment Variables chính cần set trên Coolify:
```env
# Cấu hình WebSocket Server
ASSISTANT_WS_HOST=0.0.0.0
ASSISTANT_WS_PORT=8765
AI_TRANSPORT=websocket

# Kết nối ngược lại Go Backend (Sử dụng Internal FQDN của Backend)
# Ví dụ: http://aisee-backend:8080/api/internal
GO_BACKEND_URL=http://<backend-service-internal-fqdn-or-ip>:8080/api/internal
INTERNAL_API_KEY=<your-internal-api-key>

# LiveKit Credentials
LIVEKIT_URL=https://<your-livekit-domain>
LIVEKIT_API_KEY=<your-livekit-api-key>
LIVEKIT_API_SECRET=<your-livekit-api-secret>

# Gemini API Credentials
GEMINI_API_KEY=<your-gemini-api-key>
GEMINI_MODEL=gemini-3.1-flash-live-preview
GEMINI_VOICE=Aoede

# Các cấu hình tuỳ chọn khác (nếu cần)
EXA_API_KEY=<your-exa-key>
```

---

## 6. Cơ chế Kết nối Giữa các Services thông qua Internal FQDN

Để các service (`backend`, `agent`, `minio`, `postgres`, `redis`) giao tiếp cực nhanh và bảo mật mà không cần đi vòng ra internet public:

> [!TIP]
> **Cách lấy Internal FQDN trong Coolify:**
> Với mỗi service bạn tạo ra, trong phần cấu hình **General** của service đó trên Coolify, kéo xuống sẽ thấy mục **Internal FQDN** (ví dụ: `http://minio:9000` hoặc `http://172.18.0.5:9000`).
> Hãy copy giá trị này điền vào các biến tương ứng của các service khác:
> - `S3_ENDPOINT` của Backend = `<Internal FQDN của MinIO>`
> - `ASSISTANT_BRIDGE_URL` của Backend = `ws://<Internal FQDN của Agent>/ws`
> - `GO_BACKEND_URL` của Agent = `http://<Internal FQDN của Backend>/api/internal`

---

## 7. Quy trình Kiểm tra và Vận hành (Checklist)

- [ ] **Bước 1**: Truy cập **Actions** tab trên repo GitHub `aiseebackend` xem workflow đã chạy xanh.
- [ ] **Bước 2**: Xác nhận 2 image đã xuất hiện trong **Packages** của GitHub.
- [ ] **Bước 3**: Deploy **Postgres**, **Redis**, **MinIO** trên Coolify trước.
- [ ] **Bước 4**: Đăng nhập vào giao diện quản trị MinIO (Console) và tạo bucket `aisee`.
- [ ] **Bước 5**: Deploy **Backend** và **Agent** với các cấu hình và biến kết nối nội bộ.
- [ ] **Bước 6**: Kích hoạt và kiểm tra log.

---

## 8. Tự động hóa Deploy khi có code mới (GitOps / Webhook)

Khi cấu hình service dưới dạng **Docker Image**, Coolify sẽ không tự phát hiện khi bạn push code lên GitHub nữa. Để tự động hóa quy trình kéo image mới và redeploy:

1. **Lấy Webhook URL từ Coolify:**
   * Vào phần cấu hình của **Backend** service trên Coolify > **Webhooks** > Copy URL tại phần **Deploy Webhook** (dạng `https://<coolify-domain>/api/v1/deploy?uuid=<service-uuid>&force=true`).
   * Làm tương tự để lấy Deploy Webhook cho **Agent** service.

2. **Cấu hình trên GitHub Secrets:**
   * Truy cập repository GitHub `aiseebackend` của bạn.
   * Vào **Settings** > **Secrets and variables** > **Actions** > **New repository secret**.
   * Tạo 2 Secrets:
     * `COOLIFY_BACKEND_WEBHOOK_URL` = `<URL Webhook của Backend>`
     * `COOLIFY_AGENT_WEBHOOK_URL` = `<URL Webhook của Agent>`

3. **Cơ chế hoạt động:**
   * Sau khi GitHub Actions chạy test và build + push 2 image thành công lên GHCR, nó sẽ tự động dùng lệnh `curl` gọi tới 2 webhook trên.
   * Coolify nhận tín hiệu sẽ lập tức kéo các image mới nhất `:main` từ GHCR về và tự khởi động lại ứng dụng mượt mà không cần bạn bấm deploy thủ công!

