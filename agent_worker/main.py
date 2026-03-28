import logging

from livekit.agents import AgentSession, Agent, RoomInputOptions
from livekit.plugins import google

from config import GEMINI_MODEL
from tools import ALL_TOOLS

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

    session = AgentSession(
        llm=google.realtime.RealtimeModel(
            model=GEMINI_MODEL,
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
