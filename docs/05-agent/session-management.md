# Session Management

## Tổng quan

Gemini Live API co giới hạn session phia server. Agent Worker phai quản lý session lifecycle: phát hiện sap hết hạn, reconnect trong suot, và bao toan ngu canh hội thoại.

## Session Limits

| Che đó | Thoi gian tối đa | Khi nào |
|--------|-----------------|---------|
| Audio-only | 15 phút | Mac dinh — camera tắt |
| Audio + Video | 2 phút | Khi camera dang bật |

**Hau het users se không cham giới hạn.** Các tương tác thong thuong (hoi cau hoi, gọi điện, kiểm tra thuoc) mat 1-5 phút. Gioi han 15 phút chi anh huong hội thoại keo dai.

## Reconnect Strategy

### Timeline

```
Phut 0          Phut 13              Phut 15
  │               │                    │
  │  Session      │  Canh bao          │  Session het han
  │  binh thuong  │  "Toi can ket      │  (Gemini ngat)
  │               │   noi lai..."      │
  │               │                    │
  │               └──> Bat dau         │
  │                    reconnect       │
  │                    flow            │
```

### Flow chi tiết

```
Buoc 1: Phat hien sap het han
    Agent Worker theo doi thoi gian session.
    Tai phut 13 (con 2 phut):
    Agent noi: "Toi can ket noi lai mot chut, xin cho."

Buoc 2: Tao conversation summary
    Agent Worker tom tat hoi thoai hien tai:
    - 5-10 luot noi gan nhat
    - Cac task dang thuc hien
    - Ket qua tool call chua hoan thanh

Buoc 3: Dong session cu
    Agent Worker dong Gemini Live session hien tai.

Buoc 4: Mo session moi
    Agent Worker tao Gemini Live session moi.
    Inject summary vao session moi qua:
    - Phuong an 1: send_client_content (uu tien)
    - Phuong an 2: Them vao system instruction

Buoc 5: Tiep tuc hoi thoai
    Agent noi: "Da ket noi lai. Chung ta dang noi ve [topic]."
    Tong thoi gian downtime: 1-3 giay.
```

### Xử lý camera trong luc reconnect

Nếu camera dang bật khi reconnect:
1. Video track VAN được publish tren LiveKit (không ngat)
2. Chi Gemini session bi recycle
3. Session mỗi tự động nhan video frames tu LiveKit
4. User không thay su khac biet

## Context Preservation

### Rolling Summary

Agent Worker duy tri mot ban tom tắt lien tuc của hội thoại:

```python
class ConversationContext:
    def __init__(self):
        self.turns = []        # 10 luot noi gan nhat
        self.active_tasks = [] # Task dang thuc hien
        self.pending_tools = [] # Tool call chua co ket qua

    def add_turn(self, role, content):
        self.turns.append({"role": role, "content": content})
        if len(self.turns) > 10:
            self.turns.pop(0)

    def to_summary(self):
        summary = "Ngu canh hoi thoai truoc:\n"
        for turn in self.turns:
            summary += f"- {turn['role']}: {turn['content']}\n"
        if self.active_tasks:
            summary += f"\nTask dang thuc hien: {self.active_tasks}\n"
        return summary
```

### Inject context vao session mỗi

```python
# Phuong an 1: send_client_content (uu tien)
new_session = google.realtime.RealtimeModel(
    model="gemini-2.5-flash-live-preview",
    system_instructions=SYSTEM_PROMPT_VI,
)
await new_session.send_client_content(context.to_summary())

# Phuong an 2: System instruction (fallback)
new_session = google.realtime.RealtimeModel(
    model="gemini-2.5-flash-live-preview",
    system_instructions=SYSTEM_PROMPT_VI + "\n\n" + context.to_summary(),
)
```

**Can validate o Phase 0:** `send_client_content` co ho tro inject context vao Gemini Live session mỗi không. Nếu không, dùng phương án 2.

## GoAway Message Handling

Gemini server có thể gửi GoAway message bat ky luc nào (server maintenance, overload):

```
Gemini Server gui GoAway
    │
    ▼
Agent Worker nhan event
    │
    ├─ Luu conversation context
    │
    ├─ Agent noi: "Xin cho mot chut..."
    │
    ├─ Tao session moi (co the voi server khac)
    │
    └─ Inject context va tiep tuc
```

### Phan biet GoAway và session timeout

| | Session timeout | GoAway |
|---|---|---|
| Khi nào | Chinh xac tại 15 min / 2 min | Bat ky luc nào |
| Du doan được | Co — Agent Worker biet truoc | Khong |
| Xử lý | Reconnect proactive tại phút 13 | Reconnect reactive khi nhan |

## Session Resumption Tokens

Nếu Gemini Live API ho tro session resumption tokens (can validate):

```python
# Khi dong session cu, luu token
resumption_token = old_session.get_resumption_token()

# Khi tao session moi, truyen token
new_session = google.realtime.RealtimeModel(
    model="gemini-2.5-flash-live-preview",
    system_instructions=SYSTEM_PROMPT_VI,
    resumption_token=resumption_token,
)
```

Nếu co resumption token, Gemini có thể phuc hoi context tu phia server, giam luong context phai inject tu Agent Worker.

**Trạng thái:** Can validate o Phase 0.

## Context Window Compression

Khi hội thoại keo dai, context có thể vuot qua giới hạn của Gemini. Strategy:

### Config

```python
CONTEXT_CONFIG = {
    "max_turns": 10,           # Giu toi da 10 luot noi gan nhat
    "max_summary_tokens": 500, # Tom tat khong qua 500 tokens
    "compress_after": 8,       # Nen khi vuot 8 luot
}
```

### Compression flow

```
Hoi thoai co 10 luot
    │
    ├─ Luot 1-5: Cu — tom tat thanh 1-2 cau
    │  "User hoi ve thuoc Amlodipine. Da doc nhan va xac
    │   nhan lieu 5mg/ngay."
    │
    └─ Luot 6-10: Moi — giu nguyen van
```

### Khi nào nen context

1. **Truoc reconnect:** Tom tắt 10 luot thanh dang compact
2. **Khi context qua lon:** Khi tong tokens vuot `max_summary_tokens`, nen các luot cu
3. **Khi chuyen topic:** Nếu user chuyen sang topic mỗi, có thể drop context cu

### Ví dụ summary sau compression

```
Truoc: 10 luot hoi thoai day du (2000 tokens)

Sau:
"Tong ket hoi thoai:
- User hoi doc nhan thuoc → Da bat camera, doc Amlodipine 5mg
- User dat nhac thuoc 8h sang va 8h toi
- Dang nghe dai VOV1
Topic hien tai: User hoi ve lich kham ngay mai"
(~100 tokens)
```

## Bang thời gian session lifecycle

```
t=0:00   Session bat dau (audio-only, 15 min limit)
         Gemini chao: "Xin chao [name]..."

t=1:00   User hoi cau hoi, Gemini tra loi

t=3:00   User yeu cau bat camera
         → Session chuyen sang audio+video (2 min limit)
         → Limit moi: t=3:00 + 2:00 = t=5:00

t=4:30   Gemini doc xong, tat camera
         → Session chuyen lai audio-only
         → Limit reset: con 15 min tu t=4:30

t=13:00  Canh bao sap het han (con 2 phut)

t=13:05  Bat dau reconnect flow
         - Luu context
         - Dong session cu
         - Mo session moi
         - Inject context

t=13:08  Session moi san sang (downtime ~3 giay)
         Agent: "Da ket noi lai. Chung ta dang noi ve..."

t=28:00  Lai sap het han → Reconnect lan 2
         (co the lap vo han neu user muon)
```
