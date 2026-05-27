"""Shared consent rules for camera activation."""

from __future__ import annotations

import unicodedata

_EXPLICIT_CAMERA_REQUEST_PHRASES = (
    "bật camera",
    "mở camera",
    "cho tôi xem",
    "cho con xem",
    "xem giúp",
    "nhìn giúp",
    "nhìn hộ",
    "nhìn dùm",
    "nhìn giúp tôi",
    "mở mắt",
    "bật máy ảnh",
    "mở máy ảnh",
    "quay video",
    "chụp ảnh",
    "xem thử",
    "nhìn thử",
    "look for me",
    "show you",
    "turn on camera",
    "open camera",
    "enable camera",
    "start camera",
    "use camera",
    "see this",
    "look at this",
    "take a look",
)

_EXPLICIT_CONFIRMATION_PHRASES = (
    "có",
    "có mở đi",
    "có xem đi",
    "được",
    "được rồi",
    "ok",
    "okay",
    "uh",
    "uh mở đi",
    "uh xem đi",
    "u mở đi",
    "u xem đi",
    "vào đi",
    "mở đi",
    "xem đi",
    "bật đi",
    "đồng ý",
    "được thôi",
    "mở máy đi",
    "bật máy đi",
    "nhìn đi",
    "xem nào",
    "go ahead",
    "yes",
    "sure",
    "yeah",
    "yep",
    "yup",
    "please do",
    "do it",
    "go on",
)


def validate_camera_activation_consent(
    consent_source: str,
    consent_phrase: str,
) -> tuple[bool, str]:
    """Return whether camera activation has explicit user consent."""
    normalized_source = _normalize_text(consent_source).replace(" ", "_")
    normalized_phrase = _normalize_text(consent_phrase)
    if not normalized_phrase:
        return False, "camera_consent_required"
    if normalized_source == "explicit_request":
        if _matches_any_phrase(normalized_phrase, _EXPLICIT_CAMERA_REQUEST_PHRASES):
            return True, normalized_phrase
        return False, "camera_consent_invalid"
    if normalized_source == "explicit_confirmation":
        if _matches_any_phrase(normalized_phrase, _EXPLICIT_CONFIRMATION_PHRASES):
            return True, normalized_phrase
        return False, "camera_consent_invalid"
    return False, "camera_consent_required"


def _matches_any_phrase(text: str, phrases: tuple[str, ...]) -> bool:
    padded = f" {text} "
    return any(f" {phrase} " in padded for phrase in phrases)


def _normalize_text(value: str) -> str:
    lowered = unicodedata.normalize("NFD", str(value or "").strip().lower())
    without_marks = "".join(ch for ch in lowered if unicodedata.category(ch) != "Mn")
    cleaned = "".join(ch if ch.isalnum() else " " for ch in without_marks)
    return " ".join(cleaned.split())
