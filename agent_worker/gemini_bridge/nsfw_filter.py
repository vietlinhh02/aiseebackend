"""Server-side NSFW filter — last defense before frames reach Gemini.

Uses the same MobileNet model (converted to ONNX from the Flutter TFLite asset).
Decodes JPEG → resizes to 224×224 → normalises → runs ONNX inference.
Blocks frames with NSFW score > 0.7 (sum of Hentai + Porn + Sexy classes).
"""

from __future__ import annotations

import logging
from io import BytesIO
from pathlib import Path

import numpy as np
import onnxruntime as ort
from PIL import Image

logger = logging.getLogger("aisee-agent.nsfw")

_MODEL_PATH = Path(__file__).resolve().parent.parent / "models" / "nsfw_mobilenet.onnx"
_INPUT_SIZE = 224
_NSFW_INDICES = (1, 3, 4)  # Hentai, Porn, Sexy
_THRESHOLD = 0.7


class NsfwFilter:
    """Runs ONNX NSFW inference on JPEG frames."""

    def __init__(self, model_path: str | None = None) -> None:
        path = model_path or str(_MODEL_PATH)
        if not Path(path).exists():
            raise FileNotFoundError(f"NSFW model not found at {path}")

        self._session = ort.InferenceSession(
            path,
            providers=["CPUExecutionProvider"],
        )
        self._input_name = self._session.get_inputs()[0].name

    def classify_jpeg(self, jpeg_bytes: bytes) -> float:
        """Decode JPEG, run inference, return NSFW score 0.0–1.0."""
        try:
            image = Image.open(BytesIO(jpeg_bytes)).convert("RGB")
            resized = image.resize((_INPUT_SIZE, _INPUT_SIZE), Image.LANCZOS)

            input_data = np.asarray(resized, dtype=np.float32) / 255.0
            input_data = np.expand_dims(input_data, axis=0)

            scores = self._session.run(None, {self._input_name: input_data})[0][0]

            nsfw_score = sum(float(scores[i]) for i in _NSFW_INDICES)
            return float(np.clip(nsfw_score, 0.0, 1.0))
        except Exception as exc:
            logger.warning("NSFW classification failed on frame: %s", exc)
            return 0.0

    def is_safe(self, jpeg_bytes: bytes) -> bool:
        return self.classify_jpeg(jpeg_bytes) <= _THRESHOLD
