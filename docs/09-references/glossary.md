# Bảng thuật ngữ (Glossary)

Giải thích các thuật ngữ kỹ thuật bằng tiếng Việt đơn giản, dành cho thành viên team và stakeholders không chuyên IT.

## A

**API (Application Programming Interface)**
Giao diện lập trình ứng dụng. Giống như menu nhà hàng -- app gửi "order" (yêu cầu) tới server, server trả lại "món ăn" (dữ liệu). Ví dụ: app gửi yêu cầu "lấy danh sách thuốc", server trả về danh sách.

**APNs (Apple Push Notification service)**
Dịch vụ đẩy thông báo của Apple. Khi cần gửi thông báo tới iPhone (ví dụ: cuộc gọi đến, nhắc thuốc), server gửi qua APNs, Apple chuyển tới điện thoại user.

**Agent Worker**
Chương trình Python chạy trên server, đóng vai "cầu nối" giữa Gemini AI và Go Backend. Khi user nói "gọi cho con gái", Agent Worker nhận lệnh từ Gemini, gọi Go Backend để thực hiện.

**Audio Track**
Luồng âm thanh trong cuộc gọi. Mỗi người tham gia có một audio track (giọng nói của họ). LiveKit truyền audio tracks giữa các thiết bị.

## C

**CallKit (iOS) / ConnectionService (Android)**
Hệ thống cuộc gọi native trên điện thoại. Khi có cuộc gọi đến qua AiSee, điện thoại hiện màn hình gọi giống như cuộc gọi thường (có nút nghe, từ chối). Không cần mở app.

**CRUD**
Viết tắt của Create, Read, Update, Delete. 4 thao tác cơ bản với dữ liệu: tạo mới, đọc, cập nhật, xóa.

## D

**Data Channel**
Kênh truyền dữ liệu trong LiveKit. Dùng để gửi lệnh từ Agent Worker tới app (ví dụ: "bật camera", "phát nhạc") mà không cần đi qua server riêng.

**Docker**
Công nghệ đóng gói phần mềm. Giống như container hàng hóa -- mỗi phần mềm (backend, database, LiveKit) được đóng vào "hộp" riêng, chạy độc lập trên cùng máy chủ. Dễ cài đặt, dễ di chuyển.

**Docker Compose**
Công cụ quản lý nhiều Docker container. Thay vì chạy từng phần mềm riêng, Docker Compose chạy tất cả cùng lúc bằng một lệnh.

**Drift**
Thư viện database SQLite cho Flutter. Lưu dữ liệu offline trên điện thoại (tin nhắn cache, lịch thuốc).

## E

**Endpoint**
Địa chỉ URL cụ thể trên server mà app gọi tới. Ví dụ: `/api/v1/calls/token` là endpoint để lấy token cuộc gọi.

## F

**FCM (Firebase Cloud Messaging)**
Dịch vụ đẩy thông báo của Google cho Android. Tương tự APNs nhưng cho Android. Server gửi thông báo qua FCM, Google chuyển tới điện thoại Android.

**Flutter**
Framework phát triển app đa nền tảng. Viết code 1 lần, chạy trên cả iOS và Android. AiSee dùng Flutter.

**Free Tier**
Gói sử dụng miễn phí của dịch vụ cloud. Ví dụ: Gemini API cho phép gọi miễn phí 2 lần/phút. Vượt quá giới hạn thì phải trả phí.

## G

**Gemini**
AI của Google. AiSee dùng Gemini Live -- phiên bản realtime có thể nghe giọng nói, nhìn qua camera, và trả lời bằng giọng nói.

**Gemini Live API**
Giao diện để kết nối tới Gemini dạng hội thoại realtime. Khác với Gemini thường (gửi text, nhận text), Gemini Live cho phép nói chuyện liên tục như gọi điện thoại.

**Go (Golang)**
Ngôn ngữ lập trình do Google phát triển. AiSee backend viết bằng Go vì nhanh, ổn định, dễ deploy.

## H

**Healthcheck / Health Endpoint**
Endpoint kiểm tra sức khỏe server. Server trả lời "OK" nghĩa là đang hoạt động bình thường. Monitoring tools gọi endpoint này định kỳ để phát hiện sự cố.

**High Contrast**
Chế độ hiển thị tương phản cao -- nền tối, chữ sáng. Giúp người suy giảm thị lực đọc dễ hơn.

## J

**JWT (JSON Web Token)**
"Thẻ vào cửa" số. Khi user đăng nhập, server cấp JWT. Mỗi lần app gọi API, app đính kèm JWT để chứng minh "tôi đã đăng nhập, đây là tôi". JWT có thời hạn (24 giờ), hết hạn phải refresh.

## L

**Latency**
Độ trễ. Thời gian từ khi user nói đến khi Gemini trả lời. Latency thấp = phản hồi nhanh = trải nghiệm tốt.

**LiveKit**
Nền tảng WebRTC mã nguồn mở. AiSee dùng LiveKit để truyền giọng nói, video, và dữ liệu realtime giữa app và server. LiveKit giống như "tổng đài" quản lý các cuộc gọi.

**LiveKit Room**
Phòng ảo trong LiveKit. Mỗi user có 1 room riêng. Gemini AI, người thân, tình nguyện viên có thể join vào room để nói chuyện.

**LiveKit SFU (Selective Forwarding Unit)**
Kiến trúc của LiveKit. Thay vì mỗi người gửi audio/video trực tiếp cho nhau (peer-to-peer), tất cả gửi qua server trung gian (SFU). Server forward dữ liệu tới đúng người nhận. Hiệu quả hơn khi có nhiều người trong room.

## M

**Mapbox**
Dịch vụ bản đồ và điều hướng. AiSee dùng Mapbox thay Google Maps (tránh phụ thuộc Google quá nhiều). User nói "chỉ đường tới bệnh viện", Gemini mở ứng dụng bản đồ Mapbox với điểm đến đã điền sẵn.

**Microphone Track**
Xem "Audio Track". Cụ thể là luồng âm thanh từ microphone điện thoại.

## N

**NAT Traversal**
Kỹ thuật kết nối hai thiết bị qua internet khi cả hai đều nằm sau router (NAT). WebRTC cần NAT traversal để thiết lập kết nối audio/video. TURN server hỗ trợ khi NAT traversal trực tiếp thất bại.

## O

**OAuth 2.0**
Chuẩn xác thực. Khi user nhấn "Đăng nhập bằng Google", app chuyển tới Google, user xác nhận, Google trả về token. App dùng token đó để xác minh danh tính. User không cần tạo mật khẩu riêng.

**On-Demand**
Theo yêu cầu. Camera "on-demand" nghĩa là chỉ bật khi cần (user yêu cầu hoặc Gemini quyết định), không bật liên tục.

## P

**PostgreSQL**
Hệ quản trị cơ sở dữ liệu. Nơi lưu trữ tất cả dữ liệu AiSee: user, tin nhắn, thuốc, triệu chứng, lịch sử cuộc gọi. Mã nguồn mở, miễn phí, ổn định.

**Push Notification**
Thông báo đẩy. Server gửi thông báo tới điện thoại user ngay cả khi app đang tắt. Ví dụ: "Cuộc gọi đến từ Lan", "Đã tới giờ uống thuốc". Trên iOS dùng APNs, Android dùng FCM.

## R

**Redis**
Cơ sở dữ liệu tạm thời, cực nhanh. AiSee dùng Redis cho: hàng đợi nhạc, rate limiting, cache session. Nếu Redis chết, app vẫn hoạt động (chỉ mất một số tính năng phụ).

**REST API**
Kiểu thiết kế API phổ biến nhất. Dùng các method HTTP: GET (lấy dữ liệu), POST (tạo mới), PUT (cập nhật), DELETE (xóa). AiSee backend dùng REST API.

**Riverpod**
Thư viện quản lý state trong Flutter. Giúp các phần khác nhau của app chia sẻ và cập nhật dữ liệu. Ví dụ: khi có tin nhắn mới, Riverpod cập nhật badge "tin nhắn chưa đọc" trên UI.

**Room Token**
Token (mã thông hành) để join vào LiveKit room. Mỗi token có thời hạn và quyền hạn (được publish audio, subscribe audio, etc.). Backend tạo token, app dùng token để kết nối.

## S

**Screen Reader**
Trình đọc màn hình. Phần mềm đọc to nội dung màn hình cho người suy giảm thị lực. iOS có VoiceOver, Android có TalkBack. AiSee phải tương thích với cả hai.

**SFU**
Xem "LiveKit SFU".

**Soft Delete**
Xóa mềm. Thay vì xóa dữ liệu thật, đánh dấu `deleted_at` với timestamp. Dữ liệu vẫn còn trong database nhưng không hiển thị. Có thể khôi phục nếu cần.

**SSL/TLS**
Mã hóa kết nối internet. URL bắt đầu bằng `https://` thay vì `http://` nghĩa là dùng SSL. Dữ liệu truyền giữa app và server được mã hóa, không ai đọc trộm được.

**STT (Speech-to-Text)**
Chuyển giọng nói thành chữ. User nói "gọi cho con gái", STT chuyển thành text "gọi cho con gái" để AI xử lý. Gemini Live có STT tích hợp.

## T

**TalkBack**
Screen reader trên Android. Xem "Screen Reader".

**Token**
Mã xác thực. Có nhiều loại: JWT token (đăng nhập), LiveKit room token (vào phòng gọi), FCM token (nhận push notification). Mỗi loại có mục đích và thời hạn khác nhau.

**Tool Call**
Khi Gemini AI quyết định thực hiện một hành động. Ví dụ: user nói "gọi cho con gái", Gemini tạo tool call `make_voice_call(contact_name="con gái")`. Agent Worker nhận tool call và thực hiện.

**TTS (Text-to-Speech)**
Chuyển chữ thành giọng nói. Ngược lại với STT. Gemini Live có TTS tích hợp -- AI trả lời bằng giọng nói tự nhiên.

**TURN Server**
Server trung gian hỗ trợ WebRTC khi NAT traversal trực tiếp thất bại. Tất cả audio/video đi qua TURN server. LiveKit tích hợp TURN server sẵn.

## U

**UUID (Universally Unique Identifier)**
Mã định danh duy nhất. Mỗi user, tin nhắn, thuốc đều có UUID riêng. Ví dụ: `550e8400-e29b-41d4-a716-446655440000`. Không bao giờ trùng.

## V

**Video Track**
Luồng video trong LiveKit room. Khi user bật camera, video track được publish lên room. Gemini nhận video track để phân tích hình ảnh.

**VoiceOver**
Screen reader trên iOS. Xem "Screen Reader".

**VoIP (Voice over Internet Protocol)**
Gọi điện qua internet. AiSee dùng VoIP (qua LiveKit) thay vì gọi qua mạng điện thoại truyền thống. Miễn phí, chất lượng tốt khi có internet.

**VoIP Push**
Push notification đặc biệt cho cuộc gọi trên iOS. Khác push thường: VoIP Push đánh thức app ngay lập tức và hiện màn hình cuộc gọi native (CallKit). Bắt buộc cho cuộc gọi trên iOS.

**VPS (Virtual Private Server)**
Máy chủ ảo thuê từ nhà cung cấp (Hetzner, Vultr, DigitalOcean). AiSee chạy backend, database, LiveKit trên VPS. Giống thuê một máy tính chạy 24/7 trên internet.

## W

**WebRTC (Web Real-Time Communication)**
Công nghệ truyền audio/video realtime qua internet. Chuẩn mở, dùng trong video call (Google Meet, Zoom). LiveKit xây dựng trên WebRTC. AiSee dùng WebRTC cho tất cả giao tiếp realtime.

**WebSocket**
Kênh giao tiếp hai chiều liên tục giữa app và server. Khác HTTP (gửi request, nhận response rồi đóng), WebSocket giữ kết nối mở. AiSee dùng WebSocket cho chat realtime: khi có tin nhắn mới, server push ngay tới app.

## Y

**yt-dlp**
Công cụ download/stream audio từ YouTube. AiSee backend dùng yt-dlp để tìm và lấy link audio cho chức năng phát nhạc. Chỉ dùng cho demo/internal (YouTube ToS không cho phép sử dụng thương mại).
