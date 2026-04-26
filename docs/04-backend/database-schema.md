# Database Schema

## Tổng quan

AiSee sử dụng PostgreSQL với 14 tables chinh. Tất cả primary key la UUID (`gen_random_uuid()`). Timestamps dung `TIMESTAMPTZ` (co timezone).

## Tables

### users

Lưu thông tin người dùng (ca user và family member).

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    google_id TEXT UNIQUE NOT NULL,
    email TEXT NOT NULL,
    display_name TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('user', 'family')),
    is_volunteer BOOLEAN DEFAULT false,
    avatar_url TEXT,
    fcm_token TEXT,
    apns_token TEXT,
    apns_voip_token TEXT,
    settings JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);
```

| Column | Giải thích |
|--------|-----------|
| `google_id` | Google account ID, unique, dùng de map với Google OAuth |
| `role` | `'user'` (nguoi suy giảm thị lực) hoặc `'family'` (người thân). Chon luc đăng ký. |
| `is_volunteer` | Bat ky ai cung có thể đăng ký lam tình nguyện viên, bật ke role |
| `fcm_token` | Firebase Cloud Messaging token (Android) |
| `apns_token` | Apple Push Notification token (iOS standard) |
| `apns_voip_token` | Apple VoIP Push token (iOS PushKit) — dùng cho incoming calls |
| `settings` | JSONB lưu các tùy chọn ca nhan (font size, notification preferences) |

### plans

Backend source of truth cho các gói thương mại. App/web chỉ render theo API trả về, không hardcode quyền gói.

```sql
CREATE TABLE plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    price_vnd INTEGER NOT NULL CHECK (price_vnd >= 0),
    billing_period TEXT NOT NULL CHECK (billing_period IN ('month', 'year')),
    status TEXT NOT NULL DEFAULT 'active',
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### plan_entitlements

Quyền và hạn mức theo từng plan. `limit_value` là JSONB để hỗ trợ cả số (`300`) và boolean (`true`) mà không cần client suy luận.

```sql
CREATE TABLE plan_entitlements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_id UUID NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
    feature_key TEXT NOT NULL,
    limit_value JSONB NOT NULL,
    limit_unit TEXT,
    reset_period TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(plan_id, feature_key)
);
```

### subscriptions

Subscription hiện tại/lịch sử của user. Endpoint `/api/v1/me/entitlements` dùng subscription active nếu có, nếu không dùng plan active đầu tiên theo `sort_order`.

```sql
CREATE TABLE subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    plan_id UUID NOT NULL REFERENCES plans(id),
    status TEXT NOT NULL,
    current_period_start TIMESTAMPTZ,
    current_period_end TIMESTAMPTZ,
    cancel_at_period_end BOOLEAN NOT NULL DEFAULT false,
    provider TEXT,
    provider_subscription_id TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### subscription_events

Append-only event log cho webhook, admin action, hoặc thay đổi subscription thủ công.

```sql
CREATE TABLE subscription_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subscription_id UUID REFERENCES subscriptions(id) ON DELETE SET NULL,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    event_type TEXT NOT NULL,
    provider TEXT,
    provider_event_id TEXT,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### family_links

Quan he giua user và family member. Mot user có thể co nhiều family members.

```sql
CREATE TABLE family_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    family_id UUID NOT NULL REFERENCES users(id),
    relationship TEXT NOT NULL,
    is_primary_contact BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    UNIQUE(user_id, family_id)
);
```

| Column | Giải thích |
|--------|-----------|
| `user_id` | ID của người dùng (role = 'user') |
| `family_id` | ID của người thân (role = 'family') |
| `relationship` | Quan he: "con gai", "con trai", "vo", "chong", etc. |
| `is_primary_contact` | Lien he chinh — được goi dau tien khi escalate |
| `deleted_at` | Soft-delete (xem mục "Soft-delete strategy" ben duoi) |

### messages

Tin nhan giua user và family member. Append-only, không xóa.

```sql
CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sender_id UUID NOT NULL REFERENCES users(id),
    receiver_id UUID NOT NULL REFERENCES users(id),
    content_text TEXT,
    voice_url TEXT,
    is_read BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT now()
);
```

| Column | Giải thích |
|--------|-----------|
| `content_text` | Noi dùng text. Co the NULL nếu la voice message. |
| `voice_url` | URL file voice message. Co the NULL nếu la text message. |
| `is_read` | Danh dau đã đọc. Chuyen từ `false` sang `true` khi nguoi nhan đọc. |

### medications

Thuoc và lich nhac uống thuốc.

```sql
CREATE TABLE medications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    name TEXT NOT NULL,
    dosage TEXT NOT NULL,
    times TEXT[] NOT NULL,
    frequency TEXT NOT NULL DEFAULT 'daily',
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT now(),
    deleted_at TIMESTAMPTZ
);
```

| Column | Giải thích |
|--------|-----------|
| `times` | PostgreSQL text array. Ví dụ: `{"08:00", "20:00"}` |
| `frequency` | "daily", "weekly", "as_needed" |
| `active` | Thuoc con dang sử dụng hay da ngung |

### medication_logs

Ghi lai lịch sử uống thuốc. Append-only.

```sql
CREATE TABLE medication_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    medication_id UUID NOT NULL REFERENCES medications(id),
    taken_at TIMESTAMPTZ DEFAULT now(),
    skipped BOOLEAN DEFAULT false
);
```

| Column | Giải thích |
|--------|-----------|
| `taken_at` | Thoi diem uống thuốc (hoặc thoi diem đánh dấu skip) |
| `skipped` | `true` nếu người dùng bỏ qua, `false` nếu da uong |

### health_symptoms

Ghi lai triệu chứng sức khỏe theo voice. Append-only.

```sql
CREATE TABLE health_symptoms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    symptom TEXT NOT NULL,
    severity TEXT,
    notes TEXT,
    logged_at TIMESTAMPTZ DEFAULT now()
);
```

| Column | Giải thích |
|--------|-----------|
| `symptom` | Ten triệu chứng, ghi lại nguyen van lỗi user noi |
| `severity` | Muc đó: "nhe", "vua", "nang" (optional) |

### health_metrics

Chi so sức khỏe (huyet ap, duong huyet, etc.). Append-only.

```sql
CREATE TABLE health_metrics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    metric_type TEXT NOT NULL,
    value TEXT NOT NULL,
    unit TEXT,
    logged_at TIMESTAMPTZ DEFAULT now()
);
```

| Column | Giải thích |
|--------|-----------|
| `metric_type` | Loai chỉ số: "blood_pressure", "blood_sugar", "weight", "temperature" |
| `value` | Giá trị dang text (ví dụ: "120/80", "5.5") |
| `unit` | Don vi đó (optional): "mmHg", "mmol/L", "kg" |

### appointments

Lich hen kham bac si.

```sql
CREATE TABLE appointments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    title TEXT NOT NULL,
    location TEXT,
    datetime TIMESTAMPTZ NOT NULL,
    notes TEXT,
    reminded BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT now(),
    deleted_at TIMESTAMPTZ
);
```

| Column | Giải thích |
|--------|-----------|
| `reminded` | Da gửi notification nhac hen chưa |
| `deleted_at` | Soft-delete |

### volunteers

Thong tin tình nguyện viên. Primary key là `user_id` (1-1 với `users`).

```sql
CREATE TABLE volunteers (
    user_id UUID PRIMARY KEY REFERENCES users(id),
    available BOOLEAN DEFAULT false,
    languages TEXT[] DEFAULT '{vi}',
    rating REAL DEFAULT 5.0,
    total_calls INTEGER DEFAULT 0
);
```

| Column | Giải thích |
|--------|-----------|
| `available` | Tinh nguyen vien co dang sẵn sàng nhan cuộc gọi không |
| `rating` | Diem danh gia trung bình (1.0 - 5.0) |
| `total_calls` | Tong so cuộc gọi da ho tro — dùng de can bang tại |

### audit_logs

Ghi lai tất cả tool calls tu Agent Worker. Append-only, không bao gio xóa.

```sql
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    tool_name TEXT NOT NULL,
    params JSONB,
    result TEXT,
    created_at TIMESTAMPTZ DEFAULT now()
);
```

| Column | Giải thích |
|--------|-----------|
| `tool_name` | Ten tool: "make_voice_call", "emergency_alert", etc. |
| `params` | Tham số JSON của tool call |
| `result` | Kết quả: "success", "failed", hoặc mô tả lỗi |

## Indexes

| Index | Table | Mục đích |
|-------|-------|----------|
| `idx_messages_receiver_unread` | messages | Tim nhanh tin nhắn chưa đọc của mot user. Partial index chi gom `WHERE NOT is_read`. |
| `idx_audit_logs_user_time` | audit_logs | Xem audit log theo user, sap xep theo thời gian mỗi nhat. |
| `idx_medications_user` | medications | Lấy danh sách thuoc dang active của user. Partial index `WHERE active`. |
| `idx_health_symptoms_user_time` | health_symptoms | L��ch sử triệu chứng theo user và thời gian. |
| `idx_health_metrics_user_type_time` | health_metrics | L��ch sử chỉ số theo user, loai chỉ số, và thời gian. Composite index cho dashboard charts. |
| `idx_appointments_user_time` | appointments | Lich hen sap toi của user. Partial index `WHERE datetime > now()`. |
| `idx_family_links_user` | family_links | Tim nhanh family members của mot user. |
| `idx_family_links_family` | family_links | Tim nhanh users ma mot family member được link toi. |
| `idx_messages_sender` | messages | Lấy tin nhắn da gửi boi mot user. |
| `idx_plans_status_sort` | plans | Lấy danh sách plan active đúng thứ tự hiển thị. |
| `idx_plan_entitlements_plan` | plan_entitlements | Lấy entitlement theo plan. |
| `idx_subscriptions_user_status` | subscriptions | Tìm subscription active của user. |
| `idx_subscription_events_subscription` | subscription_events | Audit event theo subscription. |
| `idx_subscription_events_user` | subscription_events | Audit event theo user. |

**Nguyên tắc chung:** Mỗi index phai phuc vu ít nhất mot query thuong xuyen. Partial indexes (`WHERE` clause) giam kich thuoc index và tang hiệu suất cho các queries cu the.

## Soft-delete strategy

Ba tables sử dụng soft-delete bang column `deleted_at TIMESTAMPTZ`:
- `family_links`
- `medications`
- `appointments`

**Cach hoạt động:**
- Khi "xóa", set `deleted_at = now()` thay vi `DELETE`.
- Tất cả queries loc `WHERE deleted_at IS NULL` de chi lấy ban ghi con hoạt động.
- Du lieu van con trong database de audit và recovery.

**Tables KHONG soft-delete (append-only):**
- `messages` — lịch sử tin nhắn không xóa
- `health_symptoms` — lịch sử triệu chứng không xóa
- `health_metrics` — lịch sử chỉ số không xóa
- `medication_logs` — lịch sử uống thuốc không xóa
- `audit_logs` — audit trail không bao gio xóa

## Migration workflow

### Tạo migration mỗi

```bash
migrate create -ext sql -dir db/migrations -seq <ten_migration>
```

Ví dụ:

```bash
migrate create -ext sql -dir db/migrations -seq add_emergency_contacts
```

Tạo 2 files:
- `002_add_emergency_contacts.up.sql` — apply changes
- `002_add_emergency_contacts.down.sql` — rollback changes

### Apply migrations

```bash
# Apply tat ca migrations chua chay
migrate -path db/migrations -database "$DATABASE_URL" up

# Apply N migrations tiep theo
migrate -path db/migrations -database "$DATABASE_URL" up N

# Rollback 1 migration
migrate -path db/migrations -database "$DATABASE_URL" down 1

# Xem version hien tai
migrate -path db/migrations -database "$DATABASE_URL" version
```

### Quy tắc viet migration

1. Mỗi migration la mot thay doi logic duy nhat (thêm table, thêm column, thêm index).
2. File `down.sql` phai rollback chính xác nhưng gì `up.sql` da lam.
3. Khong bao gio sửa migration da apply tren production. Tạo migration mỗi de sửa.
4. Test migration tren local trước khi push.
