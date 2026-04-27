"""Video frame sampling skeleton for the Gemini bridge."""

from __future__ import annotations

from io import BytesIO

from google.genai import types
from livekit import rtc
from PIL import Image


def should_sample_frame(elapsed_seconds: float, fps: float) -> bool:
    if fps <= 0:
        return False
    return elapsed_seconds >= 1 / fps


def video_frame_to_jpeg_blob(frame: rtc.VideoFrame, quality: int = 80) -> types.Blob:
    rgb = frame.convert(rtc.VideoBufferType.RGB24)
    image = Image.frombytes("RGB", (rgb.width, rgb.height), bytes(rgb.data))
    out = BytesIO()
    image.save(out, format="JPEG", quality=quality)
    return types.Blob(data=out.getvalue(), mime_type="image/jpeg")
