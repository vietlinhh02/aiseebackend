"""LiveKit participant state helpers for the Gemini bridge."""

from __future__ import annotations

from enum import StrEnum
from logging import Logger


class AgentState(StrEnum):
    INITIALIZING = "initializing"
    IDLE = "idle"
    LISTENING = "listening"
    THINKING = "thinking"
    SPEAKING = "speaking"
    DISCONNECTED = "disconnected"
    ERROR = "error"


async def publish_agent_state(local_participant, state: AgentState, logger: Logger) -> None:
    try:
        await local_participant.set_attributes({"lk.agent.state": state.value})
        logger.info("published state: %s", state.value)
    except Exception as exc:
        logger.warning("failed to publish state %s: %s", state.value, exc)
