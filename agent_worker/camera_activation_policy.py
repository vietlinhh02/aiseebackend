"""Shared consent rules for camera activation."""

from __future__ import annotations

import unicodedata

_EXPLICIT_CAMERA_REQUEST_PHRASES = (
    "bat camera",
    "mo camera",
    "cho toi xem",
    "cho con xem",
    "xem giup",
    "nhin giup",
    "nhin ho",
    "nhin dum",
    "mo mat",
    "look for me",
    "show you",
    "turn on camera",
    "open camera",
)

_EXPLICIT_CONFIRMATION_PHRASES = (
    "co",
    "co mo di",
    "co xem di",
    "duoc",
    "duoc roi",
    "ok",
    "okay",
    "uh",
    "uh mo di",
    "uh xem di",
    "u mo di",
    "u xem di",
    "vo di",
    "mo di",
    "xem di",
    "bat di",
    "go ahead",
    "yes",
    "sure",
    "please do",
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
