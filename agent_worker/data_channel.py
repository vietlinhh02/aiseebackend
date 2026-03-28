"""LiveKit data channel bridge for agent → app communication."""

import asyncio
import json
import logging
import uuid

logger = logging.getLogger("aisee-agent")

TIMEOUT_SECONDS = 5
MAX_RETRIES = 1


class DataChannelTimeoutError(Exception):
    """Raised when app does not ACK within timeout."""


class DataChannelBridge:
    """Send tool actions to app via LiveKit data channel."""

    def __init__(self):
        self._pending: dict[str, asyncio.Future] = {}

    def on_data_received(self, data: bytes):
        """Handle incoming data channel messages from app.

        Call this from the LiveKit room data received callback.
        """
        try:
            msg = json.loads(data)
        except json.JSONDecodeError:
            logger.warning("invalid data channel message: %s", data)
            return

        if msg.get("type") != "tool_action_ack":
            return

        request_id = msg.get("request_id")
        if request_id in self._pending:
            self._pending[request_id].set_result(msg)

    async def send_action(
        self,
        local_participant,
        action: str,
        params: dict,
    ) -> dict:
        """Send a tool action and wait for ACK from the app."""
        request_id = str(uuid.uuid4())
        message = json.dumps({
            "type": "tool_action",
            "request_id": request_id,
            "action": action,
            "params": params,
        })

        for attempt in range(MAX_RETRIES + 1):
            future = asyncio.get_event_loop().create_future()
            self._pending[request_id] = future

            try:
                await local_participant.publish_data(
                    message.encode(),
                    reliable=True,
                )

                result = await asyncio.wait_for(
                    future, timeout=TIMEOUT_SECONDS,
                )

                if result.get("status") == "error":
                    error_msg = result.get(
                        "error_message", "Unknown error",
                    )
                    return {"status": "error", "error": error_msg}

                return result

            except asyncio.TimeoutError:
                if attempt < MAX_RETRIES:
                    logger.warning(
                        "data channel timeout, retrying "
                        "(attempt %d): %s",
                        attempt + 1,
                        action,
                    )
                    continue
                raise DataChannelTimeoutError(
                    f"App did not respond to {action} "
                    f"after {MAX_RETRIES + 1} attempts"
                )
            finally:
                self._pending.pop(request_id, None)
