import logging

from livekit.agents import AgentSession, Agent, RoomInputOptions
from livekit.plugins import google

from backend import BackendClient
from config import GEMINI_MODEL, GO_BACKEND_URL, INTERNAL_API_KEY
from data_channel import DataChannelBridge
from tools import ALL_TOOLS, init_tools

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("aisee-agent")

SYSTEM_PROMPT = """
Bạn là AiSee, trợ lý AI cho người suy giảm thị lực tại Việt Nam.

RESPOND IN VIETNAMESE. YOU MUST RESPOND UNMISTAKABLY IN VIETNAMESE.

Nguyên tắc:
- Luôn nói tiếng Việt, giọng thân thiện, rõ ràng, chậm vừa phải
- Trả lời ngắn gọn, đi thẳng vào vấn đề
- Không lặp lại câu người dùng vừa nói
- Mỗi câu trả lời phải là thông tin mới, không tóm tắt lại

Hỗ trợ khẩn cấp (3 cấp):
1. emergency_alert — SOS gửi thông báo đến tất cả người thân
2. escalate_to_family — gọi 1 người thân cụ thể
3. escalate_to_volunteer — nếu không ai trong gia đình trả lời, kết nối với tình nguyện viên

Sau cuộc gọi với tình nguyện viên, hỏi người dùng đánh giá 1-5 sao rồi dùng rate_volunteer.

Lời chào: Khi bắt đầu cuộc trò chuyện, hãy nói:
"Xin chào, tôi là AiSee. Tôi có thể giúp gì cho bạn?"
"""


class AiSeeAgent(Agent):
    def __init__(self):
        super().__init__(
            instructions=SYSTEM_PROMPT,
        )


async def entrypoint(ctx):
    agent = AiSeeAgent()
    bridge = DataChannelBridge()
    backend = BackendClient(GO_BACKEND_URL, INTERNAL_API_KEY)

    # Get user_id from the first remote participant's identity
    user_id = ""
    for participant in ctx.room.remote_participants.values():
        user_id = participant.identity
        break

    if not user_id:
        logger.warning("No remote participant found, using empty user_id")

    # Wire data channel callbacks
    @ctx.room.on("data_received")
    def on_data(data_packet, *_args, **_kwargs):
        raw = (
            data_packet.data
            if hasattr(data_packet, "data")
            else data_packet
        )
        bridge.on_data_received(raw)

    # Initialize tool dependencies
    init_tools(
        backend=backend,
        bridge=bridge,
        local_participant=ctx.room.local_participant,
        user_id=user_id,
    )

    session = AgentSession(
        llm=google.realtime.RealtimeModel(
            model=GEMINI_MODEL,
            search_tool=google.realtime.GoogleSearch(),
        ),
    )

    for tool in ALL_TOOLS:
        session.register_tool(tool)

    await session.start(
        agent=agent,
        room=ctx.room,
        room_input_options=RoomInputOptions(video_enabled=False),
    )


if __name__ == "__main__":
    from livekit.agents import WorkerOptions, cli

    cli.run_app(
        WorkerOptions(
            entrypoint_fnc=entrypoint,
        )
    )
