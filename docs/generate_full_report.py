# -*- coding: utf-8 -*-
"""Generate comprehensive AiSee Data Analysis Report with web research."""

from docx import Document
from docx.shared import Pt, Cm, RGBColor
from docx.enum.text import WD_ALIGN_PARAGRAPH
from docx.enum.table import WD_TABLE_ALIGNMENT
import datetime


def h(doc, text, level=1):
    heading = doc.add_heading(text, level=level)
    for run in heading.runs:
        run.font.color.rgb = RGBColor(0x1A, 0x32, 0x63)
    return heading


def tbl(doc, headers, rows):
    table = doc.add_table(rows=1 + len(rows), cols=len(headers))
    table.style = "Light Grid Accent 1"
    table.alignment = WD_TABLE_ALIGNMENT.CENTER
    for i, hdr in enumerate(headers):
        c = table.rows[0].cells[i]
        c.text = hdr
        for p in c.paragraphs:
            for r in p.runs:
                r.bold = True
                r.font.size = Pt(9)
    for ri, row in enumerate(rows):
        for ci, val in enumerate(row):
            c = table.rows[ri + 1].cells[ci]
            c.text = str(val)
            for p in c.paragraphs:
                for r in p.runs:
                    r.font.size = Pt(9)
    return table


def bullets(doc, items):
    for item in items:
        doc.add_paragraph(item, style="List Bullet")


def numbered(doc, items):
    for item in items:
        doc.add_paragraph(item, style="List Number")


def para(doc, text):
    doc.add_paragraph(text)


def build():
    doc = Document()
    for s in doc.sections:
        s.top_margin = Cm(2)
        s.bottom_margin = Cm(2)
        s.left_margin = Cm(2.5)
        s.right_margin = Cm(2.5)
    style = doc.styles["Normal"]
    style.font.name = "Calibri"
    style.font.size = Pt(11)

    # ===== TRANG BÌA =====
    for _ in range(6):
        doc.add_paragraph()
    t = doc.add_paragraph()
    t.alignment = WD_ALIGN_PARAGRAPH.CENTER
    r = t.add_run("BÁO CÁO PHÂN TÍCH DỮ LIỆU TOÀN DIỆN")
    r.bold = True
    r.font.size = Pt(28)
    r.font.color.rgb = RGBColor(0x1A, 0x32, 0x63)

    st = doc.add_paragraph()
    st.alignment = WD_ALIGN_PARAGRAPH.CENTER
    r = st.add_run(
        "ỨNG DỤNG AISEE — TRỢ LÝ AI CHO NGƯỜI KHIẾM THỊ TẠI VIỆT NAM"
    )
    r.font.size = Pt(16)
    r.font.color.rgb = RGBColor(0x4A, 0x6F, 0xA5)

    doc.add_paragraph()
    m = doc.add_paragraph()
    m.alignment = WD_ALIGN_PARAGRAPH.CENTER
    r = m.add_run(
        f"Ngày lập: {datetime.date.today().strftime('%d/%m/%Y')}\n"
        "Đội ngũ: Team Đắng — VentureX S3 / VinUni ELAB\n"
        "Phiên bản: v1.0.0 (MVP)\n"
        "Báo cáo: Phân tích dữ liệu toàn diện (Codebase + Market Research)"
    )
    r.font.size = Pt(12)
    r.font.color.rgb = RGBColor(0x66, 0x66, 0x66)
    doc.add_page_break()

    # ===== MỤC LỤC =====
    h(doc, "MỤC LỤC", 1)
    toc = [
        "1. Tóm tắt điều hành (Executive Summary)",
        "2. Tổng quan sản phẩm & công nghệ",
        "3. Phân tích thị trường & đối tượng người dùng",
        "4. Phân tích cạnh tranh chi tiết",
        "5. Phân tích kiến trúc hệ thống & dữ liệu",
        "6. Phân tích tính năng & mức độ hoàn thiện",
        "7. Phân tích trải nghiệm người dùng (UX) & Accessibility",
        "8. Phân tích công nghệ & chi phí AI",
        "9. Phân tích hiệu suất & khả năng mở rộng",
        "10. Phân tích bảo mật & quyền riêng tư",
        "11. Phân tích mô hình kinh doanh & monetization",
        "12. Cơ hội tài trợ & funding",
        "13. KPIs & metrics đề xuất",
        "14. Phân tích rủi ro",
        "15. Đề xuất & lộ trình phát triển",
        "16. Kết luận",
    ]
    for item in toc:
        p = doc.add_paragraph(item)
        p.paragraph_format.space_after = Pt(4)
    doc.add_page_break()

    # ===== 1. TÓM TẮT ĐIỀU HÀNH =====
    h(doc, "1. Tóm tắt điều hành (Executive Summary)", 1)
    para(doc,
        "AiSee là ứng dụng trợ lý AI dành cho người khiếm thị tại Việt Nam, "
        "được phát triển bởi Team Đắng (VentureX S3 / VinUni ELAB). "
        "Ứng dụng sử dụng Gemini Live real-time voice + LiveKit WebRTC "
        "để hỗ trợ người dùng trong giao tiếp, quản lý sức khỏe, điều hướng và kết nối gia đình."
    )

    h(doc, "Các phát hiện chính", 2)
    bullets(doc, [
        "THỊ TRƯỜNG: ~1 triệu người khiếm thị tại VN (VDS 2016: 15.7% của 6.2M NKT). "
        "Chỉ 42.5% NKT có internet (vs 84% dân số chung) — cơ hội lớn.",
        "CẠNH TRANH: Không có đối thủ trực tiếp tại VN. Be My Eyes (900K users, $6.1M funding) "
        "và Seeing AI (Microsoft) không hỗ trợ tiếng Việt native, không có real-time voice.",
        "CÔNG NGHỆ: Gemini 3.1 Flash Live hiện MIỄN PHÍ (preview). Khi GA: ~$0.02–0.05/phút, "
        "rẻ hơn OpenAI Realtime 6–15x. Chi phí thực tế ~$5.25/user/tháng (5 phút/ngày).",
        "MONETIZATION: Ads-only không khả thi (Vietnam CPM chỉ ~$1.83). "
        "Mức giá Family: 149K / 399K / 599K VND/tháng + B2B tổ chức $10–20/user.",
        "FUNDING: USAID INCLUSION $65M (đến 2026), Google.org ~$1.5M, "
        "The Accessibility Fund, VinUni Seed Grant — nhiều cơ hội non-dilutive.",
        "PHÁP LÝ: Luật AI Việt Nam (134/2025/QH15) có hiệu lực 01/03/2026. "
        "Healthcare AI là high-risk, cần compliance trước 09/2027.",
        "SẢN PHẨM: MVP hoàn thiện với 20 AI tools, nhưng THIẾU HOÀN TOÀN analytics tracking "
        "— không thể ra quyết định data-driven.",
    ])
    doc.add_page_break()

    # ===== 2. TỔNG QUAN SẢN PHẨM =====
    h(doc, "2. Tổng quan sản phẩm & công nghệ", 1)

    h(doc, "2.1 Mô tả sản phẩm", 2)
    para(doc,
        'AiSee ("AI See") là ứng dụng di động cross-platform (iOS/Android) '
        'cung cấp trợ lý AI voice-first cho người khiếm thị. '
        'Slogan: "Không chỉ nhìn thay bạn — hành động cùng bạn."'
    )

    h(doc, "2.2 Tech Stack", 2)
    tbl(doc, ["Thành phần", "Công nghệ", "Vai trò"], [
        ["Ứng dụng di động", "Flutter 3.11.3+ (Dart)",
         "UI voice-first, wake-word, LiveKit client"],
        ["Backend", "Go 1.26.1 (Chi router)",
         "Business logic, API, xác thực, push, cơ sở dữ liệu"],
        ["Agent Worker", "Python / google-genai",
         "Gemini Live API, xử lý giọng nói, điều phối tool calls"],
        ["Real-time", "LiveKit Cloud (WebRTC SFU)",
         "Quản lý audio/video tracks, data channel"],
        ["Cơ sở dữ liệu", "PostgreSQL 16",
         "10 bảng, UUID PKs, JSONB"],
        ["Bộ nhớ đệm", "Redis 7",
         "Nhắc thuốc, trạng thái phiên, ghép tình nguyện viên"],
        ["Lưu trữ", "MinIO (tương thích S3)",
         "Tin nhắn thoại, ảnh đại diện, tệp tải lên"],
        ["Thông báo đẩy", "APNs + FCM",
         "iOS VoIP (CallKit) + Android notifications"],
        ["Từ khoá đánh thức", "Davoice + TFLite",
         "Nhận diện 'cần giúp' trên thiết bị"],
        ["Bản đồ", "Mapbox",
         "Xem trước vị trí, điều hướng"],
    ])

    h(doc, "2.3 Hệ thống vai trò", 2)
    tbl(doc, ["Vai trò", "Đối tượng", "Tính năng chính", "Chi phí"], [
        ["User", "Người khiếm thị",
         "Trợ lý AI voice, nhắc thuốc, gọi điện, SOS", "Miễn phí"],
        ["Family", "Gia đình/người chăm sóc",
         "Dashboard giám sát, chat, quản lý thuốc", "Có thể có quảng cáo"],
    ])

    h(doc, "2.4 Bộ 20 công cụ AI", 2)
    tbl(doc, ["Nhóm", "Công cụ", "Số lượng"], [
        ["Giao tiếp",
         "make_voice_call, make_video_call, send_chat_message, read_unread_messages", "4"],
        ["Điều khiển thiết bị",
         "activate_camera, deactivate_camera", "2"],
        ["Sức khỏe",
         "set_medication_reminder, list_medication_reminders, mark_medication_taken, get_health_summary", "4"],
        ["Khẩn cấp",
         "emergency_alert, escalate_emergency", "2"],
        ["Tình nguyện viên",
         "start_volunteer_match, get_match_status, rate_volunteer", "3"],
        ["Vị trí",
         "get_location, navigate_to", "2"],
        ["Giải trí",
         "play_music", "1"],
        ["Tiện ích",
         "get_time, get_weather (placeholder)", "2"],
    ])
    doc.add_page_break()

    # ===== 3. THỊ TRƯỜNG =====
    h(doc, "3. Phân tích thị trường & đối tượng người dùng", 1)

    h(doc, "3.1 Thị trường người khiếm thị Việt Nam", 2)
    tbl(doc, ["Chỉ số", "Giá trị", "Nguồn"], [
        ["Tổng số NKT tại VN", "6.2 triệu (7% dân số từ 2 tuổi)", "VDS 2016 / UNICEF"],
        ["Người khiếm thị", "~970,000 (15.7% tổng NKT)", "VDS 2016"],
        ["Ước tính mở rộng", "1.5–2 triệu (bao gồm chưa báo cáo)", "Ước tính từ các NGO"],
        ["NKT ở nông thôn", "75.7%", "VDS 2016"],
        ["NKT có internet", "42.5% (vs 84% dân số chung)", "VN Law Magazine 2024"],
        ["Trẻ em khuyết tật", "~556,000", "UNICEF Việt Nam"],
        ["Tỷ lệ smartphone (chung)", "84.4% (2024)", "DataReportal"],
        ["Tỷ lệ smartphone (NKT, ước tính)", "~40–50%", "Suy luận từ khoảng cách internet"],
    ])

    h(doc, "3.2 Thị trường công nghệ hỗ trợ toàn cầu", 2)
    tbl(doc, ["Chỉ số", "Giá trị", "Nguồn"], [
        ["Thị trường Assistive Technology (2024)", "$22.98–23.75 tỷ", "Custom Market Insights"],
        ["Dự kiến 2030", "$32.25–43.88 tỷ", "Coherent MI / Transparency MR"],
        ["CAGR", "5–13.8%", "Nhiều nguồn"],
        ["Phân khúc khiếm thị AT", "$4.2 tỷ (2024) → $13.2 tỷ (2032)", "Mordor Intelligence"],
        ["Tăng trưởng sản phẩm AI-powered AT", "+46% sản phẩm mới", "360iResearch"],
        ["Thị trường Voice AI Agents", "$2.4 tỷ (2024) → $47.5 tỷ (2034)", "Market.us"],
        ["Conversational AI", "$17.97 tỷ (2026) → $82.46 tỷ (2034)", "Nextiva"],
    ])

    h(doc, "3.3 Thị trường sức khỏe số Việt Nam", 2)
    tbl(doc, ["Chỉ số", "Giá trị", "Nguồn"], [
        ["Digital health VN (2025)", "$2.84 tỷ", "IMARC Group"],
        ["Dự kiến 2034", "$9.52 tỷ (CAGR 14.39%)", "IMARC Group"],
        ["Tỷ trọng telehealth", "28.04%", "IMARC Group"],
        ["Startup healthtech VN", "293 (28 được đầu tư, 8 Series A+)", "Tracxn"],
        ["mHealth châu Á–Thái Bình Dương (2025)", "$26.0 tỷ → $76.9 tỷ (2030)", "Mordor Intelligence"],
    ])

    h(doc, "3.4 Môi trường số Việt Nam 2025–2026", 2)
    tbl(doc, ["Chỉ số", "Giá trị"], [
        ["Người dùng internet", "85.6 triệu (84.2%)"],
        ["Kết nối di động", "137 triệu (134% dân số)"],
        ["Người dùng mạng xã hội", "79 triệu (77.6%)"],
        ["Người dùng smartphone", "~85 triệu (86%)"],
        ["Kinh tế số", "$43 tỷ (2025)"],
        ["Hệ sinh thái startup", "5,500+ startup, $3.2 tỷ tổng funding, 6 kỳ lân"],
    ])

    h(doc, "3.5 Chân dung người dùng (Persona)", 2)

    h(doc, "Persona 1: Người khiếm thị (Chính)", 3)
    tbl(doc, ["Đặc điểm", "Chi tiết"], [
        ["Số lượng tiềm năng", "~1–2 triệu tại VN"],
        ["Nhu cầu chính", "Hỗ trợ sinh hoạt, giao tiếp, di chuyển, sức khỏe"],
        ["Thách thức", "42.5% có internet, phụ thuộc người thân, ít dùng smartphone"],
        ["Hành vi", "VoiceOver/TalkBack, ưu tiên giọng nói, ít tương tác màn hình"],
    ])

    h(doc, "Persona 2: Gia đình / Người chăm sóc", 3)
    tbl(doc, ["Đặc điểm", "Chi tiết"], [
        ["Nhu cầu", "Giám sát sức khỏe, biết vị trí, nhận cảnh báo khẩn cấp"],
        ["Động lực", "An tâm về người thân, theo dõi tuân thủ thuốc"],
        ["Khả năng chi trả", "Thu nhập trung bình VN: VND 8 triệu ($315)/tháng"],
    ])
    doc.add_page_break()

    # ===== 4. CẠNH TRANH =====
    h(doc, "4. Phân tích cạnh tranh chi tiết", 1)

    h(doc, "4.1 Bảng so sánh đối thủ", 2)
    tbl(doc,
        ["Sản phẩm", "Users", "Funding", "Giá",
         "AI", "Tiếng Việt", "Voice thời gian thực", "Sức khỏe"],
        [
            ["Be My Eyes", "900K + 9.3M tình nguyện", "$8.9M",
             "Miễn phí", "GPT-4 (Be My AI)", "Không native", "Không (ảnh)", "Không"],
            ["Seeing AI", "N/A (Microsoft)", "Nội bộ Microsoft",
             "Miễn phí", "Microsoft CV", "Không", "Không", "Không"],
            ["Google Lookout", "N/A", "Nội bộ Google",
             "Miễn phí", "Google CV", "Không", "Hạn chế", "Không"],
            ["Sullivan+", "N/A", "TUAT Hàn Quốc",
             "Miễn phí", "AI tùy chỉnh", "Không (17 ngôn ngữ)", "Không", "Không"],
            ["Envision AI", "N/A", "Tư nhân",
             "$599–1,099 + $10/tháng", "GPT-5", "Không", "Có (kính)", "Không"],
            ["Aira", "N/A", "Tư nhân",
             "$89+/tháng", "Không (người thật)", "Không", "Có (video call)", "Không"],
            ["AiSee", "MVP", "VentureX/ELAB",
             "Miễn phí", "Gemini Live", "CÓ (native)", "CÓ", "CÓ"],
        ]
    )

    h(doc, "4.2 Lợi thế cạnh tranh của AiSee", 2)
    bullets(doc, [
        "DUY NHẤT tại VN: Không có đối thủ trực tiếp với giải pháp AI voice cho người khiếm thị Việt Nam",
        "Voice-first tiếng Việt native: Tối ưu cho ngữ cảnh VN, không cần dịch",
        "Hội thoại giọng nói thời gian thực: LiveKit WebRTC, không phải photo-based như Be My Eyes",
        "Hệ sinh thái toàn diện: AI + Sức khỏe + Gia đình + Khẩn cấp = tất cả trong một (đối thủ chỉ có 1–2 tính năng)",
        "Miễn phí: Rẻ hơn Aira ($89/tháng) và Envision ($599+)",
        "Chi phí AI tốt: Gemini 3.1 Flash Live ~$0.02–0.05/phút (hiện miễn phí preview), rẻ hơn OpenAI 6–15 lần",
        "Nhận diện từ khoá trên thiết bị: Không cần internet để kích hoạt, tiết kiệm pin",
    ])

    h(doc, "4.3 Thị trường VN hiện tại", 2)
    para(doc,
        "Tại Việt Nam, các giải pháp hỗ trợ người khiếm thị hiện có chủ yếu đến từ NGO:"
    )
    tbl(doc, ["Tổ chức / Sản phẩm", "Loại", "Mô tả"], [
        ["Hội Người mù VN", "NGO quốc gia",
         "Đào tạo nghề (massage, thủ công), hòa nhập cộng đồng"],
        ["Trung tâm Sao Mai", "NGO công nghệ",
         "NVDA tiếng Việt, engine TTS, thư viện accessible"],
        ["UNICEF Vi vu", "Dự án chính phủ/NGO",
         "Phần mềm TTS tiếng Việt (giọng miền Nam)"],
        ["NVDA tiếng Việt", "Phần mềm miễn phí",
         "Trình đọc màn hình tiếng Việt (Sao Mai bản địa hoá)"],
    ])
    para(doc,
        "=> Không có ứng dụng AI-powered, tiếng Việt, voice thời gian thực nào cho người khiếm thị. "
        "AiSee là SẢN PHẨM ĐẦU TIÊN TRÊN THỊ TRƯỜNG."
    )
    doc.add_page_break()

    # ===== 5. KIẾN TRÚC & DỮ LIỆU =====
    h(doc, "5. Phân tích kiến trúc hệ thống & dữ liệu", 1)

    h(doc, "5.1 Kiến trúc tổng quan", 2)
    para(doc,
        "Hệ thống gồm 4 thành phần kết nối qua 1 phòng LiveKit duy nhất cho mỗi người dùng, "
        "tối ưu tiết kiệm pin và giảm độ phức tạp:"
    )
    bullets(doc, [
        "Flutter App → LiveKit Audio Track → Agent Worker → Gemini Live API (giọng nói)",
        "Agent Worker → HTTP POST /api/internal/tool-call → Go Backend → DB/Redis (gọi công cụ)",
        "App → WebSocket /ws/chat → Go Backend → DB + Push (nhắn tin)",
        "Go Backend → APNs/FCM → App (thông báo đẩy)",
    ])

    h(doc, "5.2 Lược đồ cơ sở dữ liệu (10 bảng)", 2)
    tbl(doc, ["Bảng", "Mục đích", "Dữ liệu nhạy cảm"], [
        ["users", "Thông tin người dùng", "Email, tên, GPS, push tokens"],
        ["family_links", "Quan hệ gia đình (xoá mềm)", "Quan hệ giữa users"],
        ["messages", "Tin nhắn text/voice/image (chỉ thêm)", "Nội dung, URL voice/image"],
        ["medications", "Định nghĩa thuốc (xoá mềm)", "Tên thuốc, liều lượng"],
        ["medication_logs", "Lịch sử uống thuốc (chỉ thêm)", "Tuân thủ thuốc"],
        ["health_symptoms", "Triệu chứng sức khỏe (chỉ thêm)", "Mô tả, mức độ"],
        ["health_metrics", "Chỉ số sức khỏe (chỉ thêm)", "Huyết áp, đường huyết, cân nặng"],
        ["appointments", "Lịch hẹn khám (xoá mềm)", "Thời gian, địa điểm"],
        ["volunteers", "Hồ sơ tình nguyện viên", "Trạng thái, đánh giá"],
        ["audit_logs", "Nhật ký tool calls AI (chỉ thêm, không xoá)", "Toàn bộ tương tác AI"],
    ])

    h(doc, "5.3 Lỗ hổng dữ liệu (Data Gaps)", 2)
    para(doc,
        "Các loại dữ liệu CHƯA thu thập nhưng CẦN THIẾT cho analytics:"
    )
    tbl(doc, ["Dữ liệu thiếu", "Tại sao cần", "Ưu tiên"], [
        ["Thời lượng phiên / DAU / MAU", "Đo mức độ tương tác, giữ chân người dùng", "CAO"],
        ["Sự kiện sử dụng tính năng", "Biết tính năng nào được dùng nhiều/ít", "CAO"],
        ["Tỷ lệ hoàn thành onboarding", "Đo phễu chuyển đổi", "CAO"],
        ["Nhật ký lỗi/crash", "Theo dõi độ ổn định", "CAO"],
        ["Retention D1/D7/D30", "Đo mức độ gắn bó, tỷ lệ rời bỏ", "CAO"],
        ["Tỷ lệ nhận diện wake-word chính xác", "Đo chất lượng nhận dạng giọng nói", "TRUNG BÌNH"],
        ["Độ trễ phản hồi AI", "Đo trải nghiệm người dùng", "TRUNG BÌNH"],
        ["Tỷ lệ thành công/thất bại tool calls", "Đo độ tin cậy công cụ AI", "TRUNG BÌNH"],
        ["Điểm NPS/hài lòng", "Đo mức độ hài lòng", "THẤP (MVP)"],
    ])
    doc.add_page_break()

    # ===== 6. TÍNH NĂNG =====
    h(doc, "6. Phân tích tính năng & mức độ hoàn thiện", 1)
    tbl(doc, ["Tính năng", "Trạng thái", "Hoàn thiện", "Ghi chú"], [
        ["Đăng nhập Google OAuth", "Hoàn thành", "95%",
         "Chưa có đăng nhập bằng số điện thoại (phổ biến ở VN)"],
        ["Trợ lý giọng nói (Gemini Live)", "Hoàn thành", "90%",
         "Phiên 15 phút, có session resumption"],
        ["Nhận diện từ khoá đánh thức (Davoice)", "Hoàn thành", "85%",
         "Ngưỡng 0.88, tỷ lệ nhận diện 99.2%"],
        ["Chat text/voice/image", "Hoàn thành", "90%",
         "WebSocket thời gian thực, phân trang"],
        ["Gọi thoại/video (LiveKit)", "Hoàn thành", "85%",
         "Push VoIP iOS, CallKit"],
        ["Nhắc uống thuốc", "Hoàn thành", "90%",
         "Thông báo cục bộ + đẩy"],
        ["Theo dõi sức khỏe", "Hoàn thành", "80%",
         "Triệu chứng, chỉ số, tổng hợp, tuân thủ"],
        ["Dashboard gia đình", "Hoàn thành", "85%",
         "Sức khỏe, thuốc, vị trí, nhật ký kiểm toán"],
        ["Liên kết gia đình (mã mời)", "Hoàn thành", "90%",
         "Xoá mềm, kiểm soát truy cập"],
        ["SOS khẩn cấp", "Hoàn thành", "80%",
         "Thời gian chờ 60 giây, đẩy đến toàn bộ gia đình"],
        ["Chia sẻ vị trí (Mapbox)", "Hoàn thành", "75%",
         "Ảnh tĩnh, reverse geocode"],
        ["Nhạc/Radio", "Hoàn thành", "70%",
         "Tìm kiếm yt-dlp, đài VOV"],
        ["Ghép tình nguyện viên", "Beta", "60%",
         "Bất đồng bộ Redis, hệ thống đánh giá"],
        ["Điều hướng", "Cơ bản", "50%",
         "Chỉ mở Google Maps bên ngoài"],
        ["Theo dõi analytics", "CHƯA", "0%",
         "Không có SDK analytics nào"],
        ["Tích hợp thanh toán", "CHƯA", "0%",
         "Không có MoMo/ZaloPay/Stripe"],
        ["Bảng quản trị (Admin)", "CHƯA", "0%",
         "Cần cho quản trị hệ thống"],
        ["Đa ngôn ngữ (i18n)", "CHƯA", "0%",
         "Mã cứng tiếng Việt"],
        ["Chế độ ngoại tuyến", "CHƯA", "0%",
         "Cần internet cho mọi tính năng"],
    ])
    doc.add_page_break()

    # ===== 7. UX & ACCESSIBILITY =====
    h(doc, "7. Phân tích UX & Accessibility", 1)

    h(doc, "7.1 Điểm mạnh", 2)
    bullets(doc, [
        "Thiết kế voice-first: Tương tác hoàn toàn bằng giọng nói, không cần nhìn màn hình",
        "Từ khoá đánh thức 'cần giúp': Kích hoạt tự nhiên, trên thiết bị (Davoice 99.2%)",
        "Phản hồi xúc giác: Rung khi chuyển tab, xác nhận hành động",
        "Nhãn ngữ nghĩa (Semantic labels): Trình đọc màn hình đọc được mọi phần tử UI",
        "Phông chữ lớn (Plus Jakarta Sans), tương phản cao (navy #1A3263 / nền #F6F6F6)",
        "Onboarding đơn giản: Google Sign-In → Chọn vai trò → Cấp quyền → Dùng ngay",
    ])

    h(doc, "7.2 Điểm yếu cần cải thiện", 2)
    tbl(doc, ["Vấn đề", "Mô tả", "Đề xuất"], [
        ["Phiên giới hạn 15 phút", "Gemini Live giới hạn, người dùng phải kết nối lại",
         "Tự động kết nối lại liền mạch (đã có session resumption)"],
        ["Chỉ đăng nhập Google", "Người không có Google không dùng được",
         "Thêm đăng nhập OTP qua số điện thoại (phổ biến hơn ở VN)"],
        ["Không có chế độ ngoại tuyến", "Mất internet = không dùng được gì",
         "Lưu nhắc thuốc cục bộ, TTS ngoại tuyến cơ bản"],
        ["Video chỉ 2 phút", "Gemini giới hạn phiên video",
         "Thông báo rõ, tự động tắt nhẹ nhàng"],
        ["Không có hướng dẫn", "Người dùng mới không biết nói gì với AI",
         "Hướng dẫn onboarding bằng giọng nói, gợi ý câu lệnh"],
    ])

    h(doc, "7.3 Đánh giá WCAG 2.1", 2)
    tbl(doc, ["Tiêu chí", "Trạng thái", "Chi tiết"], [
        ["1.1 Văn bản thay thế", "Đạt", "Nhãn ngữ nghĩa cho trình đọc màn hình"],
        ["1.3 Có thể thích ứng", "Đạt", "Voice-first, không phụ thuộc bố cục phức tạp"],
        ["1.4 Có thể phân biệt", "Đạt", "Tỷ lệ tương phản cao, phông chữ lớn"],
        ["3.1 Có thể đọc được", "Đạt", "100% tiếng Việt, ngôn ngữ đơn giản"],
        ["3.2 Có thể dự đoán", "Đạt", "Điều hướng nhất quán"],
        ["4.1 Tương thích", "Một phần", "Cần kiểm tra VoiceOver/TalkBack thực tế"],
    ])
    doc.add_page_break()

    # ===== 8. CÔNG NGHỆ & CHI PHÍ AI =====
    h(doc, "8. Phân tích công nghệ & chi phí AI", 1)

    h(doc, "8.1 Giá Gemini 3.1 Flash Live (mô hình AiSee đang dùng)", 2)
    para(doc,
        "AiSee sử dụng mô hình gemini-3.1-flash-live-preview qua Live API. "
        "Hiện tại mô hình đang ở giai đoạn preview và MIỄN PHÍ. "
        "Khi chuyển sang GA (General Availability), giá chính thức như sau:"
    )
    tbl(doc,
        ["Loại token", "Giá / 1M tokens", "Tương đương / phút"],
        [
            ["Audio input", "$3.00", "$0.0045/phút"],
            ["Audio output", "$12.00", "$0.018/phút"],
            ["Text input", "$0.75", "—"],
            ["Text output", "$4.50", "—"],
            ["Image/video input", "$1.00", "$0.002/phút"],
        ]
    )
    para(doc,
        "Tốc độ audio: 25 tokens/giây = 1,500 tokens/phút. "
        "Chi phí token mới mỗi phút: ~$0.0225 ($1.35/giờ). "
        "QUAN TRỌNG: Live API tính phí TÍCH LŨY — mỗi lượt phản hồi sẽ tính lại "
        "toàn bộ token trong cửa sổ ngữ cảnh (context window), không chỉ token mới. "
        "Thực tế cho cuộc hội thoại 10 phút: $0.45–0.70 (gấp 2–3x so với token mới). "
        "Ước tính thực tế: $3–5/giờ mỗi người dùng hoạt động."
    )

    h(doc, "8.2 So sánh chi phí các mô hình AI", 2)
    tbl(doc,
        ["Mô hình", "Audio Input/M tokens", "Audio Output/M tokens",
         "Ước tính/phút", "So với 3.1 Flash Live"],
        [
            ["Gemini 3.1 Flash Live (AiSee)", "$3.00", "$12.00",
             "~$0.02–0.05", "1x (đang dùng, hiện MIỄN PHÍ preview)"],
            ["Gemini 2.5 Flash (standard)", "$1.00", "N/A (không native audio out)",
             "~$0.045", "Rẻ hơn nhưng không có speech-to-speech"],
            ["Gemini 2.5 Flash-Lite", "$0.30", "N/A",
             "~$0.013", "Rẻ nhất Gemini, không có audio output"],
            ["OpenAI GPT-4o Realtime", "$100", "$200",
             "~$0.30", "Đắt gấp 6–15 lần"],
            ["OpenAI GPT-4o-mini Realtime", "$10", "$20",
             "~$0.03", "Tương đương, chất lượng kém hơn"],
            ["Tự triển khai (Whisper+Llama)", "Chi phí GPU", "Chi phí GPU",
             "~$0.005–0.01", "Rẻ nhất nhưng độ trễ cao (1–2s)"],
        ]
    )

    h(doc, "8.3 Chi phí Gemini 3.1 Flash Live theo quy mô", 2)
    para(doc,
        "Giả định: trung bình 5 phút hội thoại/người dùng/ngày, "
        "chi phí thực tế ~$0.035/phút (bao gồm tích luỹ context):"
    )
    tbl(doc,
        ["Quy mô (MAU)", "Phút/ngày", "Phút/tháng",
         "Chi phí Gemini/tháng", "Chi phí/người dùng/tháng"],
        [
            ["100", "5", "15,000", "~$525", "~$5.25"],
            ["500", "5", "75,000", "~$2,625", "~$5.25"],
            ["1,000", "5", "150,000", "~$5,250", "~$5.25"],
            ["5,000", "5", "750,000", "~$26,250", "~$5.25"],
            ["10,000", "5", "1,500,000", "~$52,500", "~$5.25"],
        ]
    )
    para(doc,
        "LƯU Ý: Hiện tại preview MIỄN PHÍ hoàn toàn. Khi GA, chi phí ~$5.25/user/tháng "
        "cho 5 phút/ngày. Nếu bật nén context window, có thể giảm 30–50%."
    )
    para(doc,
        "=> Gemini 3.1 Flash Live vẫn là lựa chọn tốt nhất cho real-time voice. "
        "OpenAI Realtime đắt gấp 6–15 lần. Tự triển khai rẻ nhất nhưng mất lợi thế "
        "độ trễ speech-to-speech (160–400ms so với 1000–2000ms pipeline phân tầng)."
    )

    h(doc, "8.3 Bảng giá LiveKit", 2)
    tbl(doc, ["Gói", "Giá/tháng", "Giới hạn chính"], [
        ["Build (miễn phí)", "$0", "1,000 phút agent, 5,000 phút WebRTC, 50GB"],
        ["Ship", "$50", "Gấp 30 lần Build"],
        ["Scale", "$500", "Đủ điều kiện HIPAA, SOC 2, giảm 15–25%"],
        ["Enterprise", "Tuỳ chỉnh", "Hạ tầng riêng, SLA"],
    ])

    h(doc, "8.4 Luật AI Việt Nam (134/2025/QH15)", 2)
    bullets(doc, [
        "Thông qua 10/12/2025, có hiệu lực 01/03/2026 — luật AI đầu tiên ở Đông Nam Á",
        "AI y tế là NGUY CƠ CAO: cần đánh giá trước thị trường và đăng ký với chính phủ",
        "Thời gian ân hạn 18 tháng: hệ thống hiện tại phải tuân thủ trước 01/09/2027",
        "Yêu cầu: bảo vệ dữ liệu, minh bạch thuật toán, cơ chế sandbox cho đổi mới",
        "AiSee cần xác định chính xác phân loại: công nghệ hỗ trợ hay chẩn đoán y tế",
    ])

    h(doc, "8.5 Chuẩn đo độ trễ voice AI", 2)
    tbl(doc, ["Chỉ số", "Mục tiêu", "AiSee (ước tính)"], [
        ["Khoảng cách hội thoại con người", "200–400ms", "—"],
        ["Chuẩn sản xuất", "<800ms", "500ms–2s (Gemini + LiveKit)"],
        ["Nhận diện từ khoá đánh thức", "<100ms", "<100ms (TFLite trên thiết bị)"],
        ["Gửi tin nhắn chat", "<500ms", "<500ms (WebSocket)"],
        ["TTFA (Thời gian đến âm thanh đầu tiên)", "<800ms", "~500ms–1.5s"],
    ])
    doc.add_page_break()

    # ===== 9. HIỆU SUẤT & MỞ RỘNG =====
    h(doc, "9. Phân tích hiệu suất & khả năng mở rộng", 1)

    h(doc, "9.1 Chi phí vận hành theo quy mô (Gemini 3.1 Flash Live)", 2)
    para(doc,
        "Lưu ý: Gemini 3.1 Flash Live hiện MIỄN PHÍ (preview). "
        "Bảng dưới tính theo giá GA (~$0.035/phút thực tế bao gồm tích luỹ context):"
    )
    tbl(doc,
        ["Quy mô", "Hạ tầng", "AI (Gemini 3.1 Live)", "LiveKit", "Khác", "TỔNG/tháng"],
        [
            ["0–50 users", "$24–48 (VPS)", "$0 (preview miễn phí)", "$0 (free tier)", "$0–10", "$24–58"],
            ["50–200", "$48–80", "$0 (preview) → $525–1,050 (GA)", "$0–50", "$10–20", "$58–1,200"],
            ["200–1,000", "$100–200", "$1,050–5,250 (GA)", "$50–200", "$50–100", "$1,250–5,750"],
            ["1,000–5,000", "$300–600", "$5,250–26,250 (GA)", "$200–500", "$100–200", "$5,850–27,550"],
            ["5,000–10,000", "$600–1,200", "$26,250–52,500 (GA)", "$500–1,000", "$200–500", "$27,550–55,200"],
        ]
    )

    h(doc, "9.2 Điểm nghẽn tiềm năng", 2)
    bullets(doc, [
        "Gemini Live: 15 phút/phiên, giới hạn tốc độ khi mở rộng lớn chưa rõ",
        "Phòng LiveKit: Mỗi người dùng = 1 phòng luôn mở, tốn tài nguyên máy chủ",
        "yt-dlp (nhạc): Chạy tiến trình con, không mở rộng tốt",
        "PostgreSQL đơn thể: Cần bản sao đọc khi >1,000 người dùng",
        "Tiêu hao pin: Micro luôn lắng nghe + LiveKit = tốn pin đáng kể",
    ])
    doc.add_page_break()

    # ===== 10. BẢO MẬT =====
    h(doc, "10. Phân tích bảo mật & quyền riêng tư", 1)

    h(doc, "10.1 Đánh giá bảo mật", 2)
    tbl(doc, ["Khía cạnh", "Triển khai", "Đánh giá"], [
        ["Xác thực", "Google OAuth2 → JWT (HS256, 1h/30d)", "Tốt"],
        ["Lưu trữ token", "flutter_secure_storage (Keychain/Keystore)", "Tốt"],
        ["Truyền tải", "HTTPS/WSS (TLS)", "Tốt"],
        ["API nội bộ", "Khoá bí mật chung (so sánh thời gian hằng)", "Chấp nhận được"],
        ["SQL injection", "pgx truy vấn tham số hoá", "Tốt"],
        ["Giới hạn tốc độ", "Chỉ có khẩn cấp 60 giây", "YẾU — cần giới hạn toàn API"],
        ["Mã hoá dữ liệu lưu trữ", "Không", "CẦN CẢI THIỆN"],
        ["Nhật ký kiểm toán", "Bảng audit_logs, chỉ thêm", "Tốt"],
    ])

    h(doc, "10.2 Rủi ro quyền riêng tư", 2)
    tbl(doc, ["Rủi ro", "Mức độ", "Giải pháp"], [
        ["Dữ liệu sức khỏe (thuốc, triệu chứng)", "CAO",
         "Mã hoá lưu trữ, kiểm soát truy cập, chính sách lưu giữ dữ liệu"],
        ["Vị trí GPS", "CAO",
         "Chỉ thu thập khi cần, xoá sau 30 ngày, đồng ý rõ ràng"],
        ["Bản ghi âm trên MinIO", "CAO",
         "Mã hoá, tự động xoá, không dùng cho huấn luyện"],
        ["Nhật ký kiểm toán chứa nội dung AI", "TRUNG BÌNH",
         "Ẩn danh sau 90 ngày"],
        ["Khoá API trong app_config.env", "TRUNG BÌNH",
         "Làm rối mã, không commit vào git"],
    ])

    h(doc, "10.3 Tuân thủ pháp luật", 2)
    bullets(doc, [
        "Luật AI VN (134/2025): AI y tế = nguy cơ cao, tuân thủ trước 09/2027",
        "Nghị định 13/2023 (Bảo vệ DLCN): Đồng ý rõ ràng, quyền xoá dữ liệu",
        "Luật An ninh mạng 2018: Lưu trữ dữ liệu trong nước",
        "Luật Người khuyết tật 2010: Đảm bảo tiếp cận công nghệ",
        "GDPR (nếu mở rộng quốc tế): Quyền được quên, tính di động dữ liệu",
    ])
    doc.add_page_break()

    # ===== 11. MONETIZATION =====
    h(doc, "11. Phân tích mô hình kinh doanh & monetization", 1)

    h(doc, "11.1 Bài học từ đối thủ", 2)
    tbl(doc, ["Đối thủ", "Mô hình", "Kết quả"], [
        ["Be My Eyes",
         "Miễn phí cho người dùng + B2B Enterprise ($60/user/tháng Workplace)",
         "Thành công — $6.1M funding"],
        ["Aira",
         "Đăng ký $89+/tháng + Access Locations (B2B)",
         "Hoạt động nhưng khó mở rộng"],
        ["Envision",
         "Phần cứng $599–1,099 + đăng ký $10/tháng",
         "Thị trường ngách, giá trị cao"],
        ["Seeing AI / Lookout",
         "Miễn phí (CSR của Microsoft/Google)",
         "Không có mô hình doanh thu"],
    ])

    h(doc, "11.2 Mô hình monetization AiSee: Ads-Subsidized Usage Pool", 2)
    para(doc,
        "AiSee áp dụng mô hình sáng tạo kết hợp quảng cáo + subscription + usage limits, "
        "được thiết kế đặc biệt cho bối cảnh accessibility — "
        "người khiếm thị (User) không bao giờ phải xem quảng cáo, "
        "nhưng gia đình và cộng đồng có thể xem quảng cáo để tài trợ usage cho họ."
    )

    h(doc, "A. Cơ chế Usage Limits theo gói", 3)
    para(doc,
        "Mỗi người dùng khiếm thị (User) có giới hạn sử dụng hàng tháng "
        "dựa trên gói mà gia đình họ đăng ký. Giới hạn áp dụng cho:"
    )
    bullets(doc, [
        "Số phút hội thoại AI (Gemini Live sessions)",
        "Số lượt gọi tool calls nâng cao (video call, music, navigation)",
        "Dung lượng lưu trữ (voice messages, hình ảnh)",
    ])
    para(doc,
        "Các tool cơ bản luôn miễn phí không giới hạn: SOS khẩn cấp, nhắc thuốc, "
        "xem giờ, đọc tin nhắn."
    )

    h(doc, "B. Hai loại quảng cáo (Ads Pool)", 3)

    h(doc, "Pool 1: Quảng cáo Gia đình (Family Ads)", 4)
    bullets(doc, [
        "AI XEM: Thành viên gia đình (vai trò Family) trên ứng dụng",
        "LỢI ÍCH: Mỗi quảng cáo xem = tăng usage cho người thân khiếm thị trong nhà",
        "VÍ DỤ: Mẹ xem 5 quảng cáo → con (User khiếm thị) được thêm 10 phút hội thoại AI",
        "ƯU TIÊN: Usage từ quảng cáo gia đình được cộng VÀO gói hiện tại của User trước",
        "CẢM XÚC: Gia đình cảm thấy đang trực tiếp giúp đỡ người thân — "
        "tạo engagement cao hơn quảng cáo thông thường",
    ])

    h(doc, "Pool 2: Quảng cáo Cộng đồng (Supporter Ads Pool)", 4)
    bullets(doc, [
        "AI XEM: Người hỗ trợ / tình nguyện viên / bất kỳ ai muốn đóng góp",
        "CƠ CHẾ: Tất cả quảng cáo xem được đổ vào một POOL TỔNG dùng chung",
        "LỢI ÍCH: Pool tổng tài trợ usage cho TẤT CẢ người dùng miễn phí",
        "VÍ DỤ: 100 người hỗ trợ xem tổng 1,000 quảng cáo/ngày → "
        "Pool tổng cung cấp thêm 2,000 phút AI cho toàn bộ cộng đồng Free",
        "GAMIFICATION: Bảng xếp hạng người hỗ trợ, huy hiệu đóng góp, "
        "thống kê 'Bạn đã giúp X người dùng Y phút hôm nay'",
    ])

    h(doc, "C. Luồng phân bổ Usage khi chạm giới hạn", 3)
    para(doc, "Khi người dùng khiếm thị chạm giới hạn gói:")
    numbered(doc, [
        "Kiểm tra usage bonus từ quảng cáo gia đình → dùng trước nếu có",
        "Nếu hết bonus gia đình → kiểm tra gói đăng ký (149K/399K/599K)",
        "Nếu có gói trả phí → tiếp tục dùng từ Pool Cộng đồng (Supporter Ads Pool)",
        "Nếu là user FREE và hết giới hạn → bị giới hạn cho đến khi "
        "Pool Cộng đồng được bổ sung hoặc gia đình xem thêm quảng cáo",
    ])
    para(doc,
        "=> Người trả phí KHÔNG BAO GIỜ bị khoá hoàn toàn — "
        "luôn có thể dùng từ Pool Cộng đồng. "
        "Người miễn phí bị giới hạn nhưng vẫn giữ nguyên các tính năng cơ bản "
        "(SOS, nhắc thuốc, đọc tin nhắn)."
    )

    h(doc, "11.3 Bảng giá & giới hạn theo gói", 2)
    para(doc,
        "Thu nhập trung bình VN: VND 8 triệu ($315)/tháng. "
        "Thu nhập trung vị: VND 4 triệu ($157). "
        "Chi phí Gemini Live thực tế: ~$5.25/user/tháng (~VND 132K) cho 5 phút/ngày."
    )
    tbl(doc,
        ["Gói", "Giá/tháng", "Phút AI/tháng", "Tool calls nâng cao",
         "Truy cập Pool CĐ", "Xem ads tặng usage"],
        [
            ["Free (User)", "0",
             "30 phút (~1 phút/ngày)", "5 lượt/ngày (chỉ cơ bản)",
             "Không", "Không (User không xem ads)"],
            ["Family Basic", "VND 149K (~$5.9)",
             "150 phút (~5 phút/ngày)", "Tất cả tools, 20 lượt/ngày",
             "Có (khi hết gói)", "Gia đình xem ads → +bonus"],
            ["Family Plus", "VND 399K (~$15.8)",
             "450 phút (~15 phút/ngày)", "Tất cả tools, không giới hạn",
             "Có", "Gia đình xem ads → +bonus"],
            ["Family Premium", "VND 599K (~$23.7)",
             "Không giới hạn", "Tất cả tools, không giới hạn",
             "Có", "Gia đình xem ads → +bonus"],
            ["B2B Tổ chức", "$10–20/user/tháng",
             "Tuỳ chỉnh", "Tuỳ chỉnh, SLA",
             "Pool riêng", "Không áp dụng"],
        ]
    )

    h(doc, "11.4 Phân loại Tool Calls theo gói", 2)
    tbl(doc,
        ["Nhóm tool", "Free", "149K Basic", "399K Plus", "599K Premium"],
        [
            ["SOS khẩn cấp", "Luôn miễn phí", "Luôn miễn phí",
             "Luôn miễn phí", "Luôn miễn phí"],
            ["Nhắc thuốc & đánh dấu đã uống", "Luôn miễn phí", "Luôn miễn phí",
             "Luôn miễn phí", "Luôn miễn phí"],
            ["Xem giờ, đọc tin nhắn", "Luôn miễn phí", "Luôn miễn phí",
             "Luôn miễn phí", "Luôn miễn phí"],
            ["Gọi thoại gia đình", "3 lượt/ngày", "Không giới hạn",
             "Không giới hạn", "Không giới hạn"],
            ["Gọi video", "Không", "5 lượt/ngày",
             "Không giới hạn", "Không giới hạn"],
            ["Gửi tin nhắn thoại/ảnh", "5/ngày", "20/ngày",
             "Không giới hạn", "Không giới hạn"],
            ["Phát nhạc/radio", "3 bài/ngày", "15 bài/ngày",
             "Không giới hạn", "Không giới hạn"],
            ["Điều hướng (navigate_to)", "1/ngày", "10/ngày",
             "Không giới hạn", "Không giới hạn"],
            ["Camera AI (activate_camera)", "Không", "3 lượt/ngày",
             "10 lượt/ngày", "Không giới hạn"],
            ["Tóm tắt sức khỏe nâng cao", "Không", "Có",
             "Có + báo cáo xu hướng", "Có + export PDF"],
            ["Ghép tình nguyện viên", "Chỉ SOS", "Có",
             "Có + ưu tiên", "Có + ưu tiên cao nhất"],
        ]
    )

    h(doc, "11.5 Phân tích quảng cáo tại VN & hiệu quả mô hình", 2)
    tbl(doc, ["Chỉ số", "Việt Nam", "Mỹ (so sánh)"], [
        ["Facebook CPM", "~$1.83", "~$12–15"],
        ["Facebook CPC", "~$0.16", "~$0.70–1.00"],
        ["Audience Network CPM", "~$2.23", "~$8–12"],
    ])
    para(doc,
        "Với CPM VN ~$2, mỗi lượt xem quảng cáo tạo ra ~$0.002 (VND 50). "
        "Tuy nhiên trong mô hình AiSee, quảng cáo KHÔNG phải nguồn doanh thu chính — "
        "mà là CƠ CHẾ TẠO ENGAGEMENT và BÙ CHI PHÍ cho Free tier:"
    )
    tbl(doc,
        ["Chỉ số", "Giá trị", "Ghi chú"],
        [
            ["Chi phí Gemini 1 phút AI", "~VND 880 (~$0.035)", "Giá GA, hiện miễn phí preview"],
            ["Doanh thu 1 lượt xem ads", "~VND 50 (~$0.002)", "CPM VN ~$2"],
            ["Số ads cần xem để bù 1 phút AI", "~18 lượt", "880 / 50 = ~18"],
            ["Nếu gia đình xem 10 ads/ngày", "+0.56 phút/ngày cho User", "Bù ~56% gói Free"],
            ["Nếu 100 supporters xem 10 ads/ngày", "+56 phút/ngày vào Pool", "Phục vụ ~56 Free users (1 phút/user)"],
        ]
    )
    para(doc,
        "=> Quảng cáo đơn thuần không đủ bù chi phí AI, nhưng kết hợp với subscription "
        "tạo thành mô hình bền vững. Quảng cáo có vai trò:"
    )
    bullets(doc, [
        "Tạo cảm giác ĐÓNG GÓP TRỰC TIẾP cho gia đình và cộng đồng (engagement cao)",
        "Bù một phần chi phí Free tier (giảm gánh nặng grants)",
        "Tạo động lực nâng cấp gói trả phí (khi thấy ads không đủ bù usage)",
        "Xây dựng cộng đồng người hỗ trợ (viral loop, PR tốt)",
    ])

    h(doc, "11.6 Phân tích tính khả thi mức giá", 2)
    tbl(doc,
        ["Gói", "Giá", "% thu nhập trung vị", "So sánh thị trường", "Đánh giá"],
        [
            ["149K", "~$5.9", "3.7%",
             "Đắt hơn Netflix VN (108K) một chút, đắt hơn Spotify (59K)",
             "Hợp lý — giá trị an tâm sức khỏe người thân > giải trí"],
            ["399K", "~$15.8", "10%",
             "Tương đương gói gia đình SaaS VN, rẻ hơn bảo hiểm y tế",
             "Cần chứng minh giá trị rõ, phù hợp gia đình trung lưu"],
            ["599K", "~$23.7", "15%",
             "Phân khúc premium, bằng bảo hiểm sức khỏe cơ bản",
             "Phù hợp gia đình thu nhập cao, hoặc có nhiều người cần giám sát"],
        ]
    )

    h(doc, "11.7 Ước tính doanh thu theo kịch bản", 2)
    tbl(doc,
        ["Kịch bản", "Free Users", "149K", "399K", "599K",
         "B2B", "Ads revenue", "MRR", "ARR"],
        [
            ["Năm 1 (thận trọng)", "500", "100", "30", "10",
             "2 x 20 users", "~VND 3M", "VND 29.5M (~$1,170)", "~$14,000"],
            ["Năm 2 (trung bình)", "2,000", "400", "150", "50",
             "10 x 50 users", "~VND 15M", "VND 174M (~$6,900)", "~$82,800"],
            ["Năm 3 (lạc quan)", "10,000", "1,500", "500", "200",
             "30 x 100 users", "~VND 60M", "VND 800M (~$31,600)", "~$379,200"],
        ]
    )

    h(doc, "11.8 Điểm mạnh của mô hình", 2)
    bullets(doc, [
        "ĐẠO ĐỨC: Người khiếm thị KHÔNG BAO GIỜ bị ép xem quảng cáo — "
        "phù hợp giá trị accessibility",
        "TÌNH CẢM: Gia đình 'xem ads giúp người thân' tạo kết nối cảm xúc mạnh, "
        "engagement rate cao hơn ads thông thường",
        "CỘNG ĐỒNG: Pool hỗ trợ tạo viral loop — 'Tôi đã giúp 100 phút AI cho "
        "người khiếm thị hôm nay' → chia sẻ mạng xã hội → thu hút supporters mới",
        "CHUYỂN ĐỔI TỰ NHIÊN: Free → thấy ads không đủ bù → nâng cấp 149K → "
        "cần nhiều hơn → 399K → gia đình lớn → 599K",
        "KHÁC BIỆT: Không đối thủ nào có mô hình tương tự — "
        "Be My Eyes miễn phí hoàn toàn (phụ thuộc B2B), "
        "Aira đắt ($89/tháng), Envision bán phần cứng",
        "BỀN VỮNG: 3 nguồn thu song song (Subscription + Ads + B2B) "
        "giảm rủi ro phụ thuộc vào 1 nguồn duy nhất",
    ])

    h(doc, "11.9 Tích hợp thanh toán di động", 2)
    tbl(doc, ["Nhà cung cấp", "Người dùng", "Vị trí thị trường"], [
        ["MoMo", "31 triệu+, thâm nhập 69%",
         "Dẫn đầu thị trường, Series E $300M (01/2025)"],
        ["ZaloPay", "20 triệu hoạt động, thâm nhập 44%",
         "Tận dụng ứng dụng nhắn tin Zalo"],
        ["VNPay", "150,000+ điểm POS",
         "Mạnh nhất về chấp nhận tại điểm bán"],
    ])
    para(doc,
        "=> Ưu tiên tích hợp MoMo (tiếp cận người dùng) và VNPay (B2B/tổ chức)."
    )
    doc.add_page_break()

    # ===== 12. FUNDING =====
    h(doc, "12. Cơ hội tài trợ & funding", 1)

    h(doc, "12.1 Tài trợ không pha loãng (Non-dilutive)", 2)
    tbl(doc, ["Nguồn", "Giá trị", "Trạng thái", "Phù hợp"], [
        ["Dự án USAID INCLUSION", "$65 triệu tổng (đến 2026)", "Đang chạy",
         "CAO — tài trợ rõ ràng cho công nghệ hỗ trợ tại VN"],
        ["USAID Inclusion IIb (H&I)", "$12.5 triệu", "Đang chạy", "CAO"],
        ["Google.org Accelerator: Gen AI", "~$1.5 triệu (tiền + tín dụng + cố vấn)",
         "Theo dõi đợt mở 2026", "CAO"],
        ["Google for Startups", "$2K–$350K tín dụng đám mây", "Mở", "CAO"],
        ["The Accessibility Fund (TAF)", "Linh hoạt",
         "Đợt 2026 mở 11/2025", "CAO — ưu tiên Global South"],
        ["VinUni Seed Grant 2025–2026", "Linh hoạt", "Có sẵn",
         "CAO — đã trong hệ sinh thái ELAB"],
        ["Hợp tác UNICEF Việt Nam", "Hiện vật + tài trợ",
         "Cần giới thiệu dự án", "TRUNG BÌNH"],
    ])

    h(doc, "12.2 Đầu tư mạo hiểm / Đầu tư tác động", 2)
    tbl(doc, ["Quỹ", "Mô tả", "Phù hợp"], [
        ["Quỹ ĐT Mạo hiểm TP.HCM (2026)",
         "VND 500 tỷ (~$19.7 triệu), Vingroup+VinaCapital+VNG+FPT", "CAO"],
        ["Touchstone Partners",
         "Khí hậu, y tế, số hoá bao trùm", "CAO"],
        ["Do Ventures",
         "Giai đoạn sớm tại Việt Nam", "TRUNG BÌNH"],
        ["Mạng lưới Đầu tư Tác động VN (VIIN)",
         "Mạng lưới tình nguyện, kết nối", "TRUNG BÌNH"],
        ["IFC Disability Lens Investing",
         "Khung hướng dẫn cho nhà đầu tư tác động", "THAM KHẢO"],
    ])

    h(doc, "12.3 Đối tác tiềm năng", 2)
    tbl(doc, ["Đối tác", "Loại", "Giá trị"], [
        ["Hội Người mù VN", "NGO quốc gia",
         "Kênh phân phối, 63 chi hội tỉnh thành"],
        ["Trung tâm Sao Mai", "NGO công nghệ",
         "Chuyên môn kỹ thuật, cộng đồng NVDA"],
        ["UNICEF Việt Nam", "Liên Hợp Quốc",
         "Tài trợ + chương trình thí điểm"],
        ["Bệnh viện mắt", "Khách hàng B2B",
         "Doanh thu + thu hút người dùng"],
    ])
    doc.add_page_break()

    # ===== 13. KPIs =====
    h(doc, "13. KPIs & metrics đề xuất", 1)

    h(doc, "13.1 Chỉ số Bắc Cực (North Star Metric)", 2)
    para(doc,
        'Đề xuất: "Số phút tương tác AI thành công mỗi tuần" — '
        "phản ánh giá trị cốt lõi (AI hỗ trợ người khiếm thị thực sự hữu ích)."
    )

    h(doc, "13.2 KPIs theo khung AARRR", 2)
    tbl(doc, ["Nhóm", "KPI", "Mục tiêu 6 tháng", "Cách đo"], [
        ["Thu hút", "Đăng ký mới/tuần", "50 người dùng/tuần", "DB: users.created_at"],
        ["Thu hút", "Tỷ lệ hoàn thành onboarding", ">80%",
         "Theo dõi sự kiện (CẦN THÊM)"],
        ["Kích hoạt", "Dùng AI trong 24h đầu", ">60%",
         "audit_logs lần gọi tool đầu tiên"],
        ["Kích hoạt", "Thêm 1+ thành viên gia đình", ">40%",
         "DB: family_links"],
        ["Tương tác", "Tỷ lệ DAU/MAU", ">30%",
         "Theo dõi phiên (CẦN THÊM)"],
        ["Tương tác", "Số lần gọi tool/người dùng/ngày", ">5",
         "DB: audit_logs count"],
        ["Tương tác", "Phiên voice trung bình", ">3 phút",
         "Phân tích LiveKit"],
        ["Giữ chân", "Retention ngày 1", ">50%",
         "CẦN THÊM tracking"],
        ["Giữ chân", "Retention ngày 7", ">30%",
         "CẦN THÊM tracking"],
        ["Giữ chân", "Retention ngày 30", ">15%",
         "CẦN THÊM tracking"],
        ["Sức khỏe", "Tỷ lệ tuân thủ thuốc", ">70%",
         "medication_logs so với medications"],
        ["Chất lượng", "Tỷ lệ thành công tool AI", ">95%",
         "audit_logs thành công/thất bại"],
        ["Chất lượng", "Phiên không crash", ">99%",
         "Crashlytics (CẦN THÊM)"],
        ["Doanh thu", "MRR", "Tuỳ theo mô hình",
         "Hệ thống thanh toán (CẦN THÊM)"],
    ])
    doc.add_page_break()

    # ===== 14. RỦI RO =====
    h(doc, "14. Phân tích rủi ro", 1)
    tbl(doc,
        ["Rủi ro", "Xác suất", "Tác động", "Mức độ", "Giải pháp"],
        [
            ["Google Gemini thay đổi giá/điều khoản", "TB", "Cao", "CAO",
             "Lớp trừu tượng AI, sẵn sàng chuyển sang OpenAI/Claude/tự triển khai"],
            ["Luật AI VN (y tế = nguy cơ cao)", "Cao", "Cao", "CAO",
             "Xác định phân loại sớm, tuân thủ trước 09/2027"],
            ["Không đạt product-market fit", "TB", "Cao", "CAO",
             "Nghiên cứu người dùng sớm (Hội Người mù), lặp lại nhanh"],
            ["Dữ liệu sức khỏe bị rò rỉ", "Thấp", "Rất cao", "CAO",
             "Mã hoá lưu trữ, kiểm thử xâm nhập, kiểm toán truy cập"],
            ["Không có doanh thu sau 12 tháng", "TB", "Cao", "CAO",
             "Đa dạng hoá: B2B + grants + freemium. Nộp USAID/Google.org NGAY"],
            ["Giới hạn phiên Gemini 15 phút", "Cao", "TB", "TB",
             "Session resumption đã triển khai, cần UX liền mạch"],
            ["Tiêu hao pin (luôn lắng nghe)", "Cao", "TB", "TB",
             "Tối ưu engine wake-word, lịch lắng nghe thông minh"],
            ["Phụ thuộc Google OAuth (không có đăng nhập SĐT)", "Cao", "TB", "TB",
             "Thêm đăng nhập OTP qua SĐT — ưu tiên cao"],
            ["Đội ngũ nhỏ, bus factor cao", "Cao", "TB", "TB",
             "Tài liệu tốt (đã có 12 phần), CI/CD tự động"],
            ["CPM Việt Nam quá thấp cho quảng cáo", "Cao", "Thấp", "THẤP",
             "Không dựa vào quảng cáo, tập trung B2B + grants"],
        ]
    )
    doc.add_page_break()

    # ===== 15. ĐỀ XUẤT =====
    h(doc, "15. Đề xuất & lộ trình phát triển", 1)

    h(doc, "15.1 Ưu tiên ngay (Tuần 1–4)", 2)
    tbl(doc, ["#", "Đề xuất", "Lý do", "Effort"], [
        ["1", "Tích hợp analytics (PostHog/Firebase Analytics)",
         "KHÔNG THỂ ra quyết định data-driven nếu không có dữ liệu", "Trung bình"],
        ["2", "Theo dõi sự kiện toàn bộ hành động người dùng",
         "Phễu, giữ chân, mức sử dụng tính năng", "Trung bình"],
        ["3", "Báo cáo crash (Firebase Crashlytics)",
         "Biết app crash ở đâu, % người dùng bị ảnh hưởng", "Nhỏ"],
        ["4", "Chính sách quyền riêng tư + Điều khoản dịch vụ",
         "BẮT BUỘC cho App Store + Luật AI VN", "Nhỏ"],
        ["5", "Giới hạn tốc độ toàn API",
         "Bảo mật cơ bản, chống lạm dụng", "Nhỏ"],
    ])

    h(doc, "15.2 Ngắn hạn (Tháng 2–3)", 2)
    tbl(doc, ["#", "Đề xuất", "Lý do", "Effort"], [
        ["1", "Đăng nhập bằng số điện thoại (OTP)",
         "Phổ biến hơn Google ở VN, người lớn tuổi", "Lớn"],
        ["2", "Nộp đơn USAID INCLUSION + Google.org",
         "Tài trợ không pha loãng, $65M + $1.5M có sẵn", "Trung bình"],
        ["3", "Hợp tác Hội Người mù VN",
         "Kênh phân phối, 63 tỉnh thành", "Trung bình"],
        ["4", "Nhắc thuốc ngoại tuyến",
         "Tính năng quan trọng không nên phụ thuộc internet", "Trung bình"],
        ["5", "Hướng dẫn onboarding bằng giọng nói",
         "Giúp người dùng mới biết nói gì với AI", "Trung bình"],
    ])

    h(doc, "15.3 Trung hạn (Tháng 4–6)", 2)
    tbl(doc, ["#", "Đề xuất", "Lý do", "Effort"], [
        ["1", "Thí điểm B2B với bệnh viện mắt / trung tâm khiếm thị",
         "Nguồn doanh thu chính theo mô hình Be My Eyes", "Lớn"],
        ["2", "Tích hợp thanh toán (MoMo + VNPay)",
         "Đăng ký Family Premium", "Lớn"],
        ["3", "Bảng quản trị (Admin panel)",
         "Quản lý người dùng, analytics, kiểm duyệt", "Lớn"],
        ["4", "Mã hoá dữ liệu lưu trữ",
         "Tuân thủ Luật AI VN + bảo vệ dữ liệu sức khỏe", "Trung bình"],
        ["5", "Dự phòng mô hình AI (Claude/GPT/tự triển khai)",
         "Giảm phụ thuộc Google Gemini", "Lớn"],
    ])

    h(doc, "15.4 Dài hạn (Tháng 7–12)", 2)
    tbl(doc, ["#", "Đề xuất", "Lý do", "Effort"], [
        ["1", "Đa ngôn ngữ (tiếng Anh)",
         "Mở rộng ASEAN, đối tác quốc tế", "Lớn"],
        ["2", "Chuyển sang Kubernetes",
         "Mở rộng 1,000+ người dùng", "Lớn"],
        ["3", "Gắn nhãn accessibility trên App Store",
         "Apple Accessibility Labels, vị trí nổi bật", "Nhỏ"],
        ["4", "Kiểm toán tuân thủ Luật AI VN",
         "Hạn chót 09/2027, cần chuẩn bị sớm", "Lớn"],
    ])
    doc.add_page_break()

    # ===== 16. KẾT LUẬN =====
    h(doc, "16. Kết luận", 1)

    para(doc,
        "AiSee là sản phẩm MVP ấn tượng với kiến trúc vững chắc, "
        "tập trung đúng đối tượng mục tiêu và có tiềm năng thị trường đáng kể."
    )

    h(doc, "Điểm mạnh cốt lõi", 2)
    bullets(doc, [
        "ĐẦU TIÊN TRÊN THỊ TRƯỜNG tại VN: Không có đối thủ trực tiếp với giải pháp AI voice "
        "cho người khiếm thị Việt Nam",
        "TAM lớn: ~1–2 triệu người khiếm thị + gia đình, thị trường sức khỏe số $2.84 tỷ",
        "Chi phí AI hợp lý: Gemini 3.1 Flash Live hiện miễn phí (preview), khi GA ~$5.25/user/tháng, LiveKit mã nguồn mở",
        "Hệ sinh thái toàn diện: AI + Sức khỏe + Gia đình + Khẩn cấp "
        "(đối thủ chỉ có 1–2 tính năng)",
        "Bối cảnh tài trợ thuận lợi: USAID $65M, Google.org $1.5M, "
        "Quỹ ĐT Mạo hiểm TP.HCM $19.7M",
    ])

    h(doc, "3 vấn đề cốt lõi cần giải quyết NGAY", 2)
    numbered(doc, [
        "LỖ HỔNG ANALYTICS: Không có dữ liệu → không thể data-driven → "
        "không thể cải thiện. Tích hợp PostHog/Firebase Analytics là việc ĐẦU TIÊN.",
        "MONETIZATION: Quảng cáo không khả thi (CPM $1.83). Mức giá Family 149K/399K/599K "
        "hợp lý cho thị trường VN. Cần triển khai sớm kết hợp B2B tổ chức + grants. "
        "Nộp đơn USAID/Google.org ngay.",
        "TUÂN THỦ PHÁP LUẬT: Dữ liệu sức khỏe + vị trí + khuyết tật = nhạy cảm rất cao. "
        "Luật AI VN (có hiệu lực 03/2026) yêu cầu AI y tế tuân thủ trước 09/2027. "
        "Cần chính sách quyền riêng tư, mã hoá lưu trữ, và rà soát pháp lý NGAY.",
    ])

    doc.add_paragraph()
    para(doc,
        "Với nền tảng kỹ thuật vững chắc, thị trường chưa có đối thủ, và nhiều cơ hội "
        "tài trợ không pha loãng, AiSee có thể trở thành ứng dụng hỗ trợ người khiếm thị "
        "hàng đầu khu vực nếu thực hiện đúng lộ trình trong 6–12 tháng tới."
    )

    doc.add_paragraph()
    p = doc.add_paragraph("— Hết báo cáo —")
    p.alignment = WD_ALIGN_PARAGRAPH.CENTER

    # ===== PHỤ LỤC: NGUỒN THAM KHẢO =====
    doc.add_page_break()
    h(doc, "Phụ lục: Nguồn tham khảo", 1)

    h(doc, "Thị trường & Dân số", 2)
    bullets(doc, [
        "UNICEF Việt Nam — Kết quả chính VDS 2016",
        "Tổng cục Thống kê VN — Kết quả VDS 2023",
        "DataReportal — Digital 2025/2026 Vietnam",
        "WHO — Sức khoẻ mắt khu vực Tây Thái Bình Dương",
        "Vietnam Law Magazine — Người khuyết tật & Chuyển đổi số 2024",
    ])

    h(doc, "Đối thủ cạnh tranh", 2)
    bullets(doc, [
        "Be My Eyes — Thông báo gọi vốn $6.1M, Be My Eyes for Business",
        "Seeing AI — Google Play, Đánh giá cập nhật 2025 của Henshaws",
        "Envision AI — letsenvision.com, trang giá",
        "Aira — aira.io, đăng ký và tin tức giá",
        "Trung tâm Sao Mai — saomaicenter.org",
    ])

    h(doc, "Công nghệ & Chi phí", 2)
    bullets(doc, [
        "Google AI — Giá Gemini API, Giới hạn tốc độ, Tài liệu Live API",
        "OpenAI — Giá Realtime API",
        "LiveKit — Bảng giá, Tài liệu Hạn mức & Giới hạn",
        "Picovoice — Hướng dẫn nhận diện từ khoá đánh thức",
        "DaVoice — davoice.io",
    ])

    h(doc, "Monetization & Tài trợ", 2)
    bullets(doc, [
        "USAID — Dự án INCLUSION Việt Nam ($65 triệu)",
        "Google.org — Accelerator: Generative AI",
        "The Accessibility Fund — accessibilityfund.org",
        "VinUni — Seed Grant AY 2025–2026",
        "Mordor Intelligence — Thị trường thanh toán di động Việt Nam",
        "AdCostly — Chi phí quảng cáo Facebook Việt Nam",
        "Mordor Intelligence — Thị trường mHealth châu Á–Thái Bình Dương",
        "IMARC Group — Thị trường sức khỏe số Việt Nam",
    ])

    h(doc, "Pháp lý", 2)
    bullets(doc, [
        "Luật AI Việt Nam (134/2025/QH15) — Phân tích của Duane Morris, Baker McKenzie",
        "Nghị định 13/2023/NĐ-CP — Bảo vệ dữ liệu cá nhân",
        "Luật Người khuyết tật 2010 (51/2010/QH12)",
        "Luật An ninh mạng 2018",
    ])

    h(doc, "Thị trường Công nghệ Hỗ trợ", 2)
    bullets(doc, [
        "Custom Market Insights — Thị trường Assistive Technology 2024",
        "Mordor Intelligence — AT cho người khiếm thị",
        "Transparency Market Research — AT khiếm thị",
        "Market.us — Thị trường Voice AI Agents",
        "360iResearch — Thị trường Assistive Technology",
    ])

    # Lưu file
    path = (
        "/home/eddiesngu/Desktop/Dang/aiseebackend/docs/"
        "2026-04-06-aisee-full-data-analysis-report.docx"
    )
    doc.save(path)
    return path


if __name__ == "__main__":
    print(f"Báo cáo đã lưu tại: {build()}")
