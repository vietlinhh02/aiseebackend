"""LiveKit room media IO for the Gemini bridge."""

from __future__ import annotations

import asyncio
import json
import logging
from dataclasses import dataclass
from logging import Logger
from typing import Any, TYPE_CHECKING

from livekit import rtc

from .audio import pcm_to_audio_frame
from .video import video_frame_to_jpeg_blob

if TYPE_CHECKING:
    from .nsfw_filter import NsfwFilter

logger = logging.getLogger("aisee-agent")


@dataclass(slots=True)
class LiveKitRoomBridge:
    room: object

    async def start(self) -> None:
        raise NotImplementedError("LiveKit audio/video IO is implemented in the audio MVP phase")


@dataclass(slots=True)
class LiveKitDeviceCommandChannel:
    local_participant: object
    destination_identity: str

    async def request(
        self,
        command: str,
        payload: dict[str, Any],
        timeout: float | None = None,
    ) -> dict:
        kwargs = {
            "destination_identity": self.destination_identity,
            "method": command,
            "payload": json.dumps(payload),
        }
        if timeout is not None:
            kwargs["response_timeout"] = timeout
        logger.info(
            "livekit rpc start method=%s destination=%s payload_keys=%s",
            command,
            self.destination_identity,
            sorted(payload.keys()),
        )
        response = await self.local_participant.perform_rpc(**kwargs)
        result = json.loads(response) if response else {"status": "ok"}
        logger.info(
            "livekit rpc ok method=%s destination=%s status=%s",
            command,
            self.destination_identity,
            result.get("status"),
        )
        return result


@dataclass(slots=True)
class AgentAudioPublisher:
    local_participant: object
    sample_rate: int
    audio_source: object | None = None
    track_name: str = "aisee-agent-audio"
    _published_sid: str | None = None

    async def start(self) -> None:
        if self.audio_source is None:
            self.audio_source = rtc.AudioSource(self.sample_rate, 1, queue_size_ms=1000)
        if self.local_participant is None:
            return
        track = rtc.LocalAudioTrack.create_audio_track(self.track_name, self.audio_source)
        options = rtc.TrackPublishOptions()
        options.source = rtc.TrackSource.SOURCE_MICROPHONE
        publication = await self.local_participant.publish_track(track, options)
        self._published_sid = publication.sid

    async def publish_pcm(self, pcm: bytes) -> None:
        if not pcm or self.audio_source is None:
            return
        frame = pcm_to_audio_frame(pcm, self.sample_rate, 1)
        await self.audio_source.capture_frame(frame)

    def clear(self) -> None:
        if self.audio_source and hasattr(self.audio_source, "clear_queue"):
            self.audio_source.clear_queue()

    async def close(self) -> None:
        if self.local_participant is not None and self._published_sid:
            sid = self._published_sid
            try:
                await self.local_participant.unpublish_track(sid)
            except Exception as exc:
                logger.warning("agent audio unpublish skipped sid=%s error=%s", sid, exc)
            finally:
                self._published_sid = None
        if self.audio_source and hasattr(self.audio_source, "aclose"):
            await self.audio_source.aclose()


async def forward_audio_track(track, gemini_session, settings, logger: Logger) -> None:
    stream = rtc.AudioStream(
        track,
        sample_rate=settings.input_sample_rate,
        num_channels=1,
    )
    try:
        async for event in stream:
            frame = event.frame
            pcm = bytes(frame.data)
            if pcm:
                await gemini_session.send_realtime_input(audio=settings.audio_blob(pcm))
    except asyncio.CancelledError:
        raise
    except Exception as exc:
        logger.warning("audio track forwarding stopped: %s", exc)
    finally:
        await stream.aclose()


async def forward_video_track(
    track,
    gemini_session,
    fps: float,
    logger: Logger,
    *,
    nsfw_filter: NsfwFilter | None = None,
) -> None:
    if fps <= 0:
        return
    stream = rtc.VideoStream(track)
    min_interval = 1 / fps
    last_sent = 0.0
    try:
        async for event in stream:
            now = asyncio.get_running_loop().time()
            if now - last_sent < min_interval:
                continue
            last_sent = now
            blob = video_frame_to_jpeg_blob(event.frame)
            if nsfw_filter is not None and blob.data and not nsfw_filter.is_safe(blob.data):
                logger.warning("nsfw frame blocked at agent layer (livekit)")
                continue
            await gemini_session.send_realtime_input(video=blob)
    except asyncio.CancelledError:
        raise
    except Exception as exc:
        logger.warning("video track forwarding stopped: %s", exc)
    finally:
        await stream.aclose()
