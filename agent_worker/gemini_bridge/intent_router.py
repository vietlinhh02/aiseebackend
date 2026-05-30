"""State-aware intent router for user utterances.

Classifies user speech into intents before any tool is called.
Uses domain routing (not denylists) and state context for ambiguous phrases.

This replaces the old camera_activation_policy.py phrase-list approach.
"""

from __future__ import annotations

import unicodedata
from dataclasses import dataclass
from enum import StrEnum
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from .agent_runtime_state import AgentRuntimeState


# ─────────────────────────────────────────────────────────────────────
# Intent types
# ─────────────────────────────────────────────────────────────────────

class UserIntent(StrEnum):
    CAMERA_ACTIVATE = "camera.activate"
    CAMERA_RETRY = "camera.retry"
    CAMERA_DEACTIVATE = "camera.deactivate"
    VISUAL_ASSIST = "visual.assist"
    CALL_FAMILY = "call.family"
    CALL_SUPPORT = "call.support"
    EMERGENCY_ALERT = "emergency.alert"
    NAVIGATION = "navigation"
    MUSIC = "music"
    MESSAGE = "message"
    CALENDAR = "calendar"
    INFORMATION_QUERY = "information.query"
    CONFIRM_YES = "confirm.yes"
    CANCEL = "cancel"
    SOFT_RESPONSE = "soft_response"
    UNKNOWN = "unknown"


@dataclass(frozen=True, slots=True)
class IntentResult:
    intent: UserIntent
    confidence: float
    requires_context: bool
    reason: str
    is_camera_action: bool

    @property
    def is_definite(self) -> bool:
        return self.confidence >= 0.7 and not self.requires_context


# ─────────────────────────────────────────────────────────────────────
# Text normalization
# ─────────────────────────────────────────────────────────────────────

def _strip_accents(value: str) -> str:
    """Remove Vietnamese/English diacritics."""
    normalized = unicodedata.normalize("NFD", value)
    without_marks = "".join(
        ch for ch in normalized if unicodedata.category(ch) != "Mn"
    )
    # đ (U+0111) doesn't decompose in NFD
    return without_marks.replace("đ", "d").replace("Đ", "d")


def normalize_text(value: str) -> str:
    """Lowercase, strip accents, remove punctuation, collapse whitespace."""
    lowered = _strip_accents(str(value or "").strip().lower())
    cleaned = "".join(ch if ch.isalnum() else " " for ch in lowered)
    return " ".join(cleaned.split())


# Filler words/phrases to remove during normalization
_FILLERS = frozenset({
    "nhe", "nha", "di", "giup toi", "giup", "gium toi", "gium",
    "voi", "nao", "duoc khong", "ban", "minh", "cho", "cua",
    "toi", "tao",
    # English-only fillers (do NOT include "me"/"con"/"em" — they are Vietnamese nouns)
    "please", "help me", "for me", "can you", "could you",
    "let me", "up",
})


def normalize_utterance(text: str) -> str:
    """Full normalization: strip accents, remove fillers, normalize synonyms."""
    normalized = normalize_text(text)

    # Remove filler words (longest match first)
    words = normalized.split()
    filtered: list[str] = []
    i = 0
    while i < len(words):
        matched = False
        # Check 3-word, 2-word, then 1-word fillers
        for length in (3, 2, 1):
            if i + length <= len(words):
                phrase = " ".join(words[i:i + length])
                if phrase in _FILLERS:
                    i += length
                    matched = True
                    break
        if not matched:
            filtered.append(words[i])
            i += 1

    return " ".join(filtered)


# ─────────────────────────────────────────────────────────────────────
# Domain keyword sets (for routing, NOT denylist)
# ─────────────────────────────────────────────────────────────────────

# Message domain — phrases that clearly indicate in-app messaging
_MESSAGE_PHRASES = frozenset({
    "tin nhan", "tin nhan", "message", "messages",
    "doc tin", "gui tin", "nhan tin",
    "send message", "read message",
})

# Music domain — phrases that indicate music playback
_MUSIC_PHRASES = frozenset({
    "nhac", "bai hat", "phat nhac", "bat nhac",
    "mo nhac", "nghe nhac", "music", "song", "play music",
    "chuyen bai", "next song", "previous song",
    "pause music", "dung nhac", "tam dung nhac",
})

# Call family domain
_CALL_FAMILY_PHRASES = frozenset({
    "goi me", "goi bo", "goi cha", "goi chi", "goi anh",
    "goi em", "goi con", "goi nguoi nha", "goi gia dinh",
    "goi dien", "call mom", "call dad", "call family",
    "call my", "goi dien thoai",
})

_CALL_FAMILY_NOUNS = frozenset({
    "me", "bo", "cha", "me", "chi", "anh", "em", "con",
    "ba", "ong", "no", "chu", "co", "di", "bac",
    "mom", "dad", "mother", "father", "sister", "brother",
    "family", "nguoi nha", "gia dinh",
})

# Call support domain
_CALL_SUPPORT_PHRASES = frozenset({
    "goi ho tro", "goi tinh nguyen vien", "goi nguoi ho tro",
    "goi nguoi giup", "can ai do giup", "can nguoi ho tro",
    "call support", "call volunteer", "call helper",
    "nguoi ho tro", "tinh nguyen vien", "volunteer",
})

# Emergency domain — high urgency signals
_EMERGENCY_PHRASES = frozenset({
    "cuu toi", "cuu", "toi bi nga", "toi bi dau", "toi gap nguy hiem",
    "khong tho duoc", "dau nguc", "toi om", "toi bi ton thuong",
    "goi cuu thuong", "goi cap cuu", "sos", "help",
    "i fell", "i am in danger", "call ambulance", "emergency",
    "toi dang gap nguy hiem", "toi can giup do gap",
    "toi bi nga roi", "toi khong di duoc",
})

# Navigation domain
_NAVIGATION_PHRASES = frozenset({
    "chi duong", "dan toi", "mo ban do", "tim duong",
    "navigate", "directions", "open maps", "maps",
    "dan duong", "tim duong di",
})

# Calendar domain
_CALENDAR_PHRASES = frozenset({
    "lich hen", "lich kham", "appointment", "schedule",
    "xem lich", "check schedule",
})

# Information query domain — things user wants to KNOW, not SEE through camera
_INFORMATION_QUERY_PHRASES = frozenset({
    # Weather
    "thoi tiet", "troi mua", "troi nang", "nhiet do",
    "weather", "forecast", "rain",
    # News
    "doc bao", "tin tuc", "tin moi", "xem tin", "news",
    # Prices / finance
    "gia vang", "gia do la", "gia co phieu", "ty gia",
    "gold price", "stock price", "exchange rate",
    # Schedule / time queries
    "may gio", "hom nay la ngay", "ngay mai la ngay",
    "what time", "what day",
    # General information lookups
    "thong tin ve", "tim hieu ve", "thong tin lien he",
    "dia chi", "so dien thoai", "website",
})

# Camera-specific keywords (for direct camera commands)
_CAMERA_KEYWORDS = frozenset({
    "camera", "may anh", "may camera", "cam",
})

# Activate verbs (normalized, accents stripped)
_ACTIVATE_VERBS = frozenset({
    "bat", "mo", "open", "turn on", "enable", "start", "use",
    "kich hoat", "mo ra",
})

# Deactivate verbs
_DEACTIVATE_VERBS = frozenset({
    "tat", "dong", "dung", "ngung", "close", "turn off", "disable", "stop",
    "ngung lai",
})

# Retry markers
_RETRY_MARKERS = frozenset({
    "lai", "again", "back", "re", "lan nua", "them lan",
})

# View/look verbs (visual assist context)
_VIEW_VERBS = frozenset({
    "xem", "nhin", "look", "see", "show", "watch",
    "doc", "read", "kiem tra", "check", "quay", "scan",
})

# Confirmation words
_CONFIRM_WORDS = frozenset({
    "co", "duoc", "ok", "okay", "uh", "u", "dong y",
    "duoc roi", "duoc thoi", "duoc chu",
    "yes", "sure", "yeah", "yep", "yup",
    "go ahead", "please do", "do it", "go on",
    "bat di", "mo di", "bat lai di", "mo lai di",
    "xem di", "nhin di",
})

# Hard cancel — unambiguous cancellation signals
_CANCEL_WORDS = frozenset({
    "huy", "huy bo",
    "khong can", "khong muon", "khong can nua",
    "bo qua", "bo di",
    "thoi di", "thoi khong can",
    "cancel", "nah", "never mind", "forget it",
    "khong goi nua", "khong bat nua", "dung lai",
    "dung lai di", "ngung lai",
})
# Single words that are ambiguous alone ("không" could be "no" to a question,
# "thôi" could be "enough") — only trigger cancel when spoken in isolation.
_CANCEL_AMBIGUOUS_SOLO = frozenset({
    "khong", "thoi", "no",
})
# Soft responses — social acknowledgment, NOT cancellation.
# Should NOT trigger cancel; let Gemini handle conversationally.
_SOFT_RESPONSE_WORDS = frozenset({
    "khong sao", "khong sao dau", "khong co gi",
    "khong phai", "khong phai dau", "khong chac",
    "khong biet", "khong hieu",
    "de nghi", "thoi kệ", "mac ke",
    "never mind that", "it is fine", "it is ok",
    "does not matter", "not really",
})


# ─────────────────────────────────────────────────────────────────────
# Domain routing — check if text clearly belongs to a specific domain
# ─────────────────────────────────────────────────────────────────────

def _contains_phrase(normalized: str, phrases: frozenset[str]) -> bool:
    """Check if any phrase appears in the normalized text."""
    return any(phrase in normalized for phrase in phrases)


def _has_domain_verb_noun(normalized: str, verbs: frozenset[str], nouns: frozenset[str]) -> bool:
    """Check if text contains both a verb and a noun from given sets."""
    words = set(normalized.split())
    has_verb = bool(words & verbs) or any(v in normalized for v in verbs if " " in v)
    has_noun = bool(words & nouns) or any(n in normalized for n in nouns if " " in n)
    return has_verb and has_noun


# ─────────────────────────────────────────────────────────────────────
# Main classifier
# ─────────────────────────────────────────────────────────────────────

def classify_intent(
    text: str,
    state: AgentRuntimeState | None = None,
) -> IntentResult:
    """Classify user utterance into an intent.

    Uses a layered approach:
    1. Domain routing (message, music, call, emergency, navigation, calendar)
    2. Camera-specific detection (camera keyword + verb)
    3. Visual assist (view verbs without domain match)
    4. Ambiguous phrases (require state context)
    5. Fallback to unknown
    """
    normalized = normalize_utterance(text)
    if not normalized:
        return IntentResult(
            intent=UserIntent.UNKNOWN,
            confidence=0.0,
            requires_context=False,
            reason="empty_utterance",
            is_camera_action=False,
        )

    words = set(normalized.split())
    has_camera_word = bool(words & _CAMERA_KEYWORDS)
    has_retry = bool(words & _RETRY_MARKERS)
    has_activate_verb = bool(words & _ACTIVATE_VERBS)
    has_deactivate_verb = bool(words & _DEACTIVATE_VERBS)
    has_view_verb = bool(words & _VIEW_VERBS)

    # ── Layer 1: Cancel (highest priority for safety) ──
    # Multi-word cancel phrases first (unambiguous)
    if _contains_phrase(normalized, _CANCEL_WORDS):
        return IntentResult(
            intent=UserIntent.CANCEL,
            confidence=0.95,
            requires_context=False,
            reason="cancel_phrase",
            is_camera_action=False,
        )
    # Soft responses (social acknowledgment) — NOT cancel
    if _contains_phrase(normalized, _SOFT_RESPONSE_WORDS):
        return IntentResult(
            intent=UserIntent.SOFT_RESPONSE,
            confidence=0.8,
            requires_context=False,
            reason="soft_response",
            is_camera_action=False,
        )
    # Ambiguous single words ("không", "thôi") — only cancel when spoken alone
    if words <= _CANCEL_AMBIGUOUS_SOLO:
        return IntentResult(
            intent=UserIntent.CANCEL,
            confidence=0.8,
            requires_context=False,
            reason="cancel_solo_ambiguous",
            is_camera_action=False,
        )

    # ── Layer 2: Emergency (high priority — safety first) ──
    if _contains_phrase(normalized, _EMERGENCY_PHRASES):
        return IntentResult(
            intent=UserIntent.EMERGENCY_ALERT,
            confidence=0.95,
            requires_context=False,
            reason="emergency_phrase",
            is_camera_action=False,
        )

    # ── Layer 3: Domain routing — clearly non-camera intents ──
    # Message domain
    if _contains_phrase(normalized, _MESSAGE_PHRASES):
        return IntentResult(
            intent=UserIntent.MESSAGE,
            confidence=0.9,
            requires_context=False,
            reason="message_domain",
            is_camera_action=False,
        )

    # Music domain
    if _contains_phrase(normalized, _MUSIC_PHRASES):
        return IntentResult(
            intent=UserIntent.MUSIC,
            confidence=0.9,
            requires_context=False,
            reason="music_domain",
            is_camera_action=False,
        )

    # Call family domain
    if _contains_phrase(normalized, _CALL_FAMILY_PHRASES):
        return IntentResult(
            intent=UserIntent.CALL_FAMILY,
            confidence=0.9,
            requires_context=False,
            reason="call_family_domain",
            is_camera_action=False,
        )
    # "goi" + family noun (e.g., "goi lai me")
    if "goi" in words and (words & _CALL_FAMILY_NOUNS):
        return IntentResult(
            intent=UserIntent.CALL_FAMILY,
            confidence=0.85,
            requires_context=False,
            reason="call_verb_family_noun",
            is_camera_action=False,
        )

    # Call support domain
    if _contains_phrase(normalized, _CALL_SUPPORT_PHRASES):
        return IntentResult(
            intent=UserIntent.CALL_SUPPORT,
            confidence=0.9,
            requires_context=False,
            reason="call_support_domain",
            is_camera_action=False,
        )

    # Navigation domain
    if _contains_phrase(normalized, _NAVIGATION_PHRASES):
        return IntentResult(
            intent=UserIntent.NAVIGATION,
            confidence=0.9,
            requires_context=False,
            reason="navigation_domain",
            is_camera_action=False,
        )

    # Calendar domain
    if _contains_phrase(normalized, _CALENDAR_PHRASES):
        return IntentResult(
            intent=UserIntent.CALENDAR,
            confidence=0.9,
            requires_context=False,
            reason="calendar_domain",
            is_camera_action=False,
        )

    # Information query domain — user wants to KNOW something, not SEE through camera
    if _contains_phrase(normalized, _INFORMATION_QUERY_PHRASES):
        return IntentResult(
            intent=UserIntent.INFORMATION_QUERY,
            confidence=0.85,
            requires_context=False,
            reason="information_query_domain",
            is_camera_action=False,
        )

    # ── Layer 4: Camera-specific (explicit camera word) ──
    if has_camera_word:
        if has_deactivate_verb:
            return IntentResult(
                intent=UserIntent.CAMERA_DEACTIVATE,
                confidence=0.95,
                requires_context=False,
                reason="camera_deactivate_explicit",
                is_camera_action=False,
            )
        if has_retry:
            return IntentResult(
                intent=UserIntent.CAMERA_RETRY,
                confidence=0.95,
                requires_context=False,
                reason="camera_retry_explicit",
                is_camera_action=True,
            )
        return IntentResult(
            intent=UserIntent.CAMERA_ACTIVATE,
            confidence=0.95,
            requires_context=False,
            reason="camera_activate_explicit",
            is_camera_action=True,
        )

    # ── Layer 5: Visual assist (view verbs without domain match) ──
    # "xem thuoc nay", "nhin do an nay", "doc giay nay" — user wants visual help
    if has_view_verb:
        if has_retry:
            return IntentResult(
                intent=UserIntent.CAMERA_RETRY,
                confidence=0.75,
                requires_context=_needs_state_context(state),
                reason="view_verb_retry",
                is_camera_action=True,
            )
        if has_deactivate_verb:
            return IntentResult(
                intent=UserIntent.CAMERA_DEACTIVATE,
                confidence=0.8,
                requires_context=False,
                reason="view_verb_deactivate",
                is_camera_action=False,
            )
        return IntentResult(
            intent=UserIntent.VISUAL_ASSIST,
            confidence=0.7,
            requires_context=_needs_state_context(state),
            reason="view_verb_visual_assist",
            is_camera_action=True,
        )

    # ── Layer 6: Activate verb standalone ("bat di", "mo di") ──
    if has_activate_verb:
        if has_retry:
            return IntentResult(
                intent=UserIntent.CAMERA_RETRY,
                confidence=0.7,
                requires_context=_needs_state_context(state),
                reason="activate_verb_retry",
                is_camera_action=True,
            )
        return IntentResult(
            intent=UserIntent.CAMERA_ACTIVATE,
            confidence=0.6,
            requires_context=_needs_state_context(state),
            reason="activate_verb_standalone",
            is_camera_action=True,
        )

    # ── Layer 7: English patterns ──
    if "turn on" in normalized or "open" in words or "reopen" in words:
        if has_retry:
            return IntentResult(
                intent=UserIntent.CAMERA_RETRY,
                confidence=0.8,
                requires_context=False,
                reason="english_activate_retry",
                is_camera_action=True,
            )
        return IntentResult(
            intent=UserIntent.CAMERA_ACTIVATE,
            confidence=0.7,
            requires_context=_needs_state_context(state),
            reason="english_activate",
            is_camera_action=True,
        )

    # ── Layer 8: Pure confirmation ──
    if words <= _CONFIRM_WORDS:
        return IntentResult(
            intent=UserIntent.CONFIRM_YES,
            confidence=0.5,
            requires_context=True,
            reason="pure_confirmation",
            is_camera_action=False,
        )

    # ── Layer 9: Deactivate verb standalone ──
    if has_deactivate_verb:
        return IntentResult(
            intent=UserIntent.CAMERA_DEACTIVATE,
            confidence=0.5,
            requires_context=_needs_state_context(state),
            reason="deactivate_verb_standalone",
            is_camera_action=False,
        )

    return IntentResult(
        intent=UserIntent.UNKNOWN,
        confidence=0.0,
        requires_context=False,
        reason=f"no_intent:{normalized}",
        is_camera_action=False,
    )


def _needs_state_context(state: AgentRuntimeState | None) -> bool:
    """Check if intent resolution requires state context."""
    if state is None:
        return True
    # If camera was recently active or blocked, ambiguous phrases are camera-related
    if state.camera.enabled or state.camera.last_block_reason is not None:
        return False  # context available, intent is clear
    if state.conversation_mode == "camera_assist":
        return False
    return True


# ─────────────────────────────────────────────────────────────────────
# Camera consent validation (backward-compatible API)
# ─────────────────────────────────────────────────────────────────────

def validate_camera_consent(
    consent_source: str,
    consent_phrase: str,
    state: AgentRuntimeState | None = None,
) -> tuple[bool, str]:
    """Validate camera activation consent using intent classification.

    This replaces the old camera_activation_policy.validate_camera_activation_consent().
    Kept as a separate function for backward compatibility.
    """
    result = classify_intent(consent_phrase, state)
    normalized_source = normalize_text(consent_source).replace(" ", "_")

    if not consent_phrase or not consent_phrase.strip():
        return False, "camera_consent_required"

    if normalized_source == "explicit_request":
        if result.is_camera_action and result.confidence >= 0.5:
            return True, result.reason
        return False, f"camera_consent_invalid:{result.reason}"

    if normalized_source == "explicit_confirmation":
        # Soft responses ("không sao", "không phải") are NOT camera consent
        if result.intent in {UserIntent.SOFT_RESPONSE, UserIntent.CANCEL}:
            return False, f"camera_consent_invalid:{result.reason}"
        if result.intent in {UserIntent.CONFIRM_YES, UserIntent.CAMERA_ACTIVATE,
                             UserIntent.CAMERA_RETRY, UserIntent.VISUAL_ASSIST}:
            return True, result.reason
        # If state shows we're awaiting camera confirmation, accept confirmation words
        if state and state.camera.awaiting_reenable_confirmation:
            if result.intent == UserIntent.CONFIRM_YES:
                return True, "confirmed_with_state_context"
        return False, f"camera_consent_invalid:{result.reason}"

    return False, "camera_consent_required"
