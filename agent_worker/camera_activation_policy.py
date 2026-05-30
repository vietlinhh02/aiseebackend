"""Camera activation consent — delegates to IntentRouter.

This file is kept for backward compatibility.  The actual classification
is done by intent_router.classify_intent() + validate_camera_consent().

The old phrase-list approach has been replaced with state-aware intent
classification.  See intent_router.py for the implementation.
"""

from __future__ import annotations

from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from gemini_bridge.agent_runtime_state import AgentRuntimeState


def validate_camera_activation_consent(
    consent_source: str,
    consent_phrase: str,
    runtime_state: AgentRuntimeState | None = None,
) -> tuple[bool, str]:
    """Validate camera activation consent.

    Delegates to intent_router.validate_camera_consent().
    Kept as a public API for backward compatibility with tool_router.py.
    """
    from gemini_bridge.intent_router import validate_camera_consent
    return validate_camera_consent(consent_source, consent_phrase, runtime_state)
