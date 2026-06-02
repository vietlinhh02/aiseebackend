# Authentication Flow

## Tổng quan

AiSee sử dụng Google OAuth2 de xác thực người dùng, sau do phát hành JWT cho các request tiep theo. Khong sử dụng Firebase SDK.

Co 2 loai authentication:
1. **Client auth** — Flutter app gửi Google ID token, backend tra JWT
2. **Internal auth** — Agent Worker gửi shared secret trong header `X-Internal-Key`

## Google OAuth2 Flow

### So do toan canh

```
┌─────────────┐     ┌──────────────┐     ┌──────────────┐     ┌────────────┐
│ Flutter App  │     │ Google OAuth  │     │  Go Backend  │     │ PostgreSQL │
└──────┬──────┘     └──────┬───────┘     └──────┬───────┘     └──────┬─────┘
       │                    │                     │                    │
       │  1. google_sign_in │                     │                    │
       │───────────────────>│                     │                    │
       │                    │                     │                    │
       │  2. Google ID Token│                     │                    │
       │<───────────────────│                     │                    │
       │                    │                     │                    │
       │  3. POST /auth/google                    │                    │
       │  { id_token, role }│                     │                    │
       │─────────────────────────────────────────>│                    │
       │                    │                     │                    │
       │                    │  4. Verify ID token  │                    │
       │                    │  (Google public keys)│                    │
       │                    │<────────────────────│                    │
       │                    │─────────────────────>│                    │
       │                    │                     │                    │
       │                    │                     │  5. Find/Create    │
       │                    │                     │     user           │
       │                    │                     │───────────────────>│
       │                    │                     │<───────────────────│
       │                    │                     │                    │
       │  6. { access_token, refresh_token, user }│                    │
       │<─────────────────────────────────────────│                    │
       │                    │                     │                    │
```

### Bước chi tiết

**Bước 1-2: App lấy Google ID Token**

Flutter app sử dụng package `google_sign_in` de hiển thị Google Sign-In dialog:

```dart
final googleUser = await GoogleSignIn(
  scopes: ['email', 'profile'],
).signIn();
final googleAuth = await googleUser.authentication;
final idToken = googleAuth.idToken;
```

User chon tài khoản Google, app nhan được `idToken` (JWT do Google ky).

**Bước 3: App gửi ID token toi backend**

```
POST /api/v1/auth/google
Content-Type: application/json

{
  "id_token": "<google-id-token>",
  "role": "user"
}
```

`role` chi can truyen lan dau đăng ký. Các lan sau backend lấy role tu database.

**Bước 4: Backend verify Google ID token**

Backend verify token bang Google public keys (JWKS):

1. Fetch Google JWKS từ `https://www.googleapis.com/oauth2/v3/certs` (cache 24h)
2. Verify JWT signature
3. Verify `iss` là `accounts.google.com` hoặc `https://accounts.google.com`
4. Verify `aud` khop với `GOOGLE_CLIENT_ID` của ứng dụng
5. Verify `exp` chưa hết hạn
6. Extract `sub` (Google user ID), `email`, `name`, `picture`

**Bước 5: Tim hoặc tạo user**

- Tim user trong database theo `google_id = sub`
- Nếu chưa co: tạo user mỗi với role tu request body
- Nếu da co: cập nhật `email`, `display_name`, `avatar_url` nếu thay doi

**Bước 6: Phat hanh JWT**

Backend tạo 2 tokens:

| Token | Thoi han | Mục đích |
|-------|----------|----------|
| Access token | 1 gio | Xac thuc các API request |
| Refresh token | 10 nam rolling | Lấy access token moi khi hết hạn |

JWT payload:

```json
{
  "sub": "<user-uuid>",
  "email": "user@gmail.com",
  "role": "user",
  "iat": 1711612800,
  "exp": 1711616400
}
```

JWT được ky bằng `HS256` với `JWT_SECRET` tu environment variable.

## Token Refresh Flow

```
┌─────────────┐                    ┌──────────────┐
│ Flutter App  │                    │  Go Backend  │
└──────┬──────┘                    └──────┬───────┘
       │                                  │
       │  API request voi expired token   │
       │─────────────────────────────────>│
       │                                  │
       │  401 Unauthorized                │
       │<─────────────────────────────────│
       │                                  │
       │  POST /auth/refresh              │
       │  { refresh_token: "..." }        │
       │─────────────────────────────────>│
       │                                  │
       │  Verify refresh token            │
       │  Phat hanh access + refresh moi  │
       │                                  │
       │  { access_token, refresh_token } │
       │<─────────────────────────────────│
       │                                  │
       │  Retry API request voi token moi │
       │─────────────────────────────────>│
       │                                  │
```

### Xử lý trong app

Flutter app (Dio interceptor) tự động xử lý refresh:

1. Nhan HTTP 401 tu bat ky API request nào
2. Goi `POST /api/v1/auth/refresh` với refresh token da lưu
3. Nếu thành công: lưu token mỗi vao `flutter_secure_storage`, retry request ban dau
4. Nếu thất bại (refresh token cung hết hạn): dua user ve man hinh đăng nhập

### Refresh token rotation

Moi lan refresh, backend phát hành CA HAI access token và refresh token mới.
Refresh token cũ bị vô hiệu hóa trong `auth_sessions`. Phiên được kéo dài
thêm 10 năm từ lần refresh mới nhất. Điều này giúp:
- Không phụ thuộc Google session trên thiết bị sau lần đăng nhập đầu
- Phat hien token reuse (nếu refresh token cu được dùng lai, có thể la tan cong)

## Internal API Auth (Agent Worker -> Backend)

Agent Worker xác thực bang shared secret, không dung JWT.

```
X-Internal-Key: <INTERNAL_API_KEY>
```

### Cach hoạt động

1. Ca Agent Worker và Go Backend doc cung giá trị `INTERNAL_API_KEY` tu environment variable
2. Agent Worker gửi giá trị này trong header `X-Internal-Key` của mỗi request toi `/api/internal/*`
3. Backend middleware so sanh constant-time (chong timing attack)
4. Nếu không khop: tra 401

### Bao mat mang

Internal endpoints chi accessible trong Docker network:
- Port `/api/internal/*` KHONG expose ra ngoai Docker compose network
- Chi các containers trong cung network mỗi goi được
- Thêm vao do shared secret la lop bảo mật thu hai

### Ví dụ request tu Agent Worker

```python
import httpx

async def call_backend_tool(tool_name, user_id, params):
    resp = await httpx.AsyncClient().post(
        "http://go-backend:8080/api/internal/tool-call",
        headers={"X-Internal-Key": INTERNAL_API_KEY},
        json={
            "tool": tool_name,
            "user_id": user_id,
            "params": params,
        },
    )
    return resp.json()
```

## Bao mat

| Mỗi quan tam | Giải pháp |
|--------------|-----------|
| JWT bi lo | Access token chi 1 gio. Refresh token rotation. |
| Google token gia | Verify bang Google public keys (JWKS), kiểm tra `aud` và `iss` |
| Internal API bi goi tu bên ngoài | Docker network isolation + shared secret |
| Timing attack tren secret comparison | Dùng `crypto/subtle.ConstantTimeCompare` |
| Token lưu tren device | `flutter_secure_storage` (Keychain iOS, Keystore Android) |
