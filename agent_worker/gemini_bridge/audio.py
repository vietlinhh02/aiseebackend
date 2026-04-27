"""Audio conversion helpers for the Gemini bridge."""

from __future__ import annotations

from dataclasses import dataclass

from livekit import rtc


@dataclass(frozen=True, slots=True)
class PCMFrame:
    data: bytes
    sample_rate: int
    channels: int


def pcm_to_audio_frame(pcm: bytes, sample_rate: int, channels: int = 1) -> rtc.AudioFrame:
    bytes_per_sample = 2
    frame_size = channels * bytes_per_sample
    usable_len = len(pcm) - (len(pcm) % frame_size)
    if usable_len <= 0:
        return rtc.AudioFrame(b"", sample_rate, channels, 0)
    data = pcm[:usable_len]
    samples_per_channel = usable_len // frame_size
    return rtc.AudioFrame(data, sample_rate, channels, samples_per_channel)
