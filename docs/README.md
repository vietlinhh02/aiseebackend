# AiSee — Documentation

> **"Không chỉ nhìn thay bạn — hành động cùng bạn"**

AiSee là ứng dụng di động tích hợp AI Agent cho người suy giảm thị lực tại Việt Nam.

**Team Đắng** | VentureX Season 3 | VinUniversity ELAB | 2026

---

## Hướng dẫn đọc

Tài liệu được sắp xếp từ **tổng quan → chi tiết**. Bạn không cần đọc hết — chọn phần phù hợp với vai trò của bạn.

| Bạn là ai? | Đọc phần nào? |
|---|---|
| **Investor / Mentor / Giám khảo** | 01-overview, 02-product |
| **Product Manager / Designer** | 01-overview, 02-product, 03-architecture |
| **Team Lead / Tech Lead** | 03-architecture, 08-plans |
| **Backend Developer (Go)** | 04-backend, 03-architecture |
| **AI/ML Engineer (Python)** | 05-agent, 03-architecture |
| **Mobile Developer (Flutter)** | 06-app, 03-architecture |
| **DevOps** | 07-infrastructure |
| **Người mới join team** | 01-overview → 03-architecture → phần chuyên môn |
| **Không biết gì về tech** | 01-overview, 02-product, 09-references/glossary |

---

## Mục lục

### 01 — Tổng quan (Ai đọc cũng hiểu)

- [AiSee là gì?](01-overview/what-is-aisee.md) — Giới thiệu sản phẩm, vấn đề giải quyết
- [Người dùng mục tiêu](01-overview/target-users.md) — Persona, ai sẽ dùng AiSee
- [Nghiên cứu thị trường](01-overview/market-research.md) — Quy mô, đối thủ, cơ hội
- [Tầm nhìn sản phẩm](01-overview/product-vision.md) — Roadmap 1-3-5 năm

### 02 — Sản phẩm (PM, Designer, Stakeholder)

- [Tổng quan tính năng](02-product/features-overview.md) — Tất cả tính năng, giải thích đơn giản
- [Hành trình người dùng](02-product/user-journeys.md) — Một ngày với AiSee
- [20 công cụ AI](02-product/ai-tools.md) — AI có thể làm gì, ví dụ cụ thể
- [Hai giao diện](02-product/two-interfaces.md) — Giao diện Người dùng vs Người thân
- [Mô hình kinh doanh](02-product/pricing-model.md) — Miễn phí cho user, ads trên giao diện gia đình

### 03 — Kiến trúc (Semi-technical)

- [Tổng quan hệ thống](03-architecture/system-overview.md) — Kiến trúc, các thành phần
- [Công nghệ sử dụng](03-architecture/tech-stack.md) — Stack và lý do lựa chọn
- [Luồng dữ liệu](03-architecture/data-flow.md) — Dữ liệu chạy thế nào
- [Bảo mật & Quyền riêng tư](03-architecture/security-and-privacy.md)
- [Phụ thuộc Google](03-architecture/google-dependency.md) — Rủi ro và giảm thiểu

### 04 — Backend (Go Developer)

- [Cài đặt môi trường](04-backend/setup.md)
- [Cấu trúc dự án](04-backend/project-structure.md)
- [Database Schema](04-backend/database-schema.md)
- [API Reference](04-backend/api-reference.md)
- [Luồng xác thực](04-backend/auth-flow.md)
- [Push Notifications](04-backend/push-notifications.md)
- [Xử lý lỗi](04-backend/error-handling.md)

### 05 — Agent Worker (Python AI/ML Engineer)

- [Cài đặt môi trường](05-agent/setup.md)
- [Tích hợp Gemini Live](05-agent/gemini-live-integration.md)
- [Định nghĩa 20 Tools](05-agent/tool-definitions.md)
- [Giao thức Data Channel](05-agent/data-channel-protocol.md)
- [Quản lý Session](05-agent/session-management.md)
- [System Prompt](05-agent/system-prompt.md) — Prompt tiếng Việt cho Gemini

### 06 — Mobile App (Flutter Developer)

- [Cài đặt môi trường](06-app/setup.md)
- [Cấu trúc dự án](06-app/project-structure.md)
- [Tích hợp LiveKit](06-app/livekit-integration.md)
- [Giao diện Người dùng](06-app/user-ui-guide.md) — Voice-first design
- [Giao diện Người thân](06-app/family-ui-guide.md) — Dashboard design
- [Accessibility](06-app/accessibility.md) — VoiceOver, TalkBack
- [Hoạt động Offline](06-app/offline-behavior.md)

### 07 — Hạ tầng (DevOps)

- [Hướng dẫn Deploy](07-infrastructure/deployment-guide.md)
- [LiveKit Server](07-infrastructure/livekit-server.md)
- [Monitoring](07-infrastructure/monitoring.md)
- [Ước tính Chi phí](07-infrastructure/cost-estimation.md)

### 08 — Kế hoạch triển khai (Implementation Plans)

- [Phase 0: Foundation](08-plans/phase-0-foundation.md) — Tuần 1-2
- [Phase 1: Voice + Vision](08-plans/phase-1-voice-vision.md) — Tuần 2-4
- [Phase 2: Agent Mode](08-plans/phase-2-agent-mode.md) — Tuần 5-7
- [Phase 3: Family + Support](08-plans/phase-3-family-support.md) — Tuần 8-9
- [Phase 4: Polish + Demo](08-plans/phase-4-polish-demo.md) — Tuần 10-12

### 09 — Tham khảo

- [Nguồn nghiên cứu](09-references/research-sources.md) — Links đã verify
- [Phân tích đối thủ](09-references/competitor-analysis.md) — So sánh chi tiết
- [Thuật ngữ](09-references/glossary.md) — Giải thích cho người không tech

### Specs (Technical Design)

- [MVP Design Spec](superpowers/specs/2026-03-28-aisee-mvp-design.md) — Spec kỹ thuật đầy đủ (24 sections)

---

## Quick Links

| Cần gì? | Xem đâu? |
|---|---|
| Hiểu AiSee nhanh trong 5 phút | [what-is-aisee.md](01-overview/what-is-aisee.md) |
| Xem tất cả tính năng | [features-overview.md](02-product/features-overview.md) |
| Kiến trúc hệ thống | [system-overview.md](03-architecture/system-overview.md) |
| API endpoints | [api-reference.md](04-backend/api-reference.md) |
| System prompt cho Gemini | [system-prompt.md](05-agent/system-prompt.md) |
| Cách deploy | [deployment-guide.md](07-infrastructure/deployment-guide.md) |
| Chi phí vận hành | [cost-estimation.md](07-infrastructure/cost-estimation.md) |
| Giải thích thuật ngữ | [glossary.md](09-references/glossary.md) |
