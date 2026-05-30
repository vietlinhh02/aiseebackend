"""Tests for intent_router — domain routing, camera consent, cancel semantics."""

import pytest

from gemini_bridge.intent_router import (
    UserIntent,
    classify_intent,
    normalize_utterance,
    validate_camera_consent,
)


# ── Normalization ──

class TestNormalization:
    def test_strips_accents_and_fillers(self):
        assert "camera" in normalize_utterance("Mở camera giúp tôi nhé")

    def test_preserves_domain_words(self):
        result = normalize_utterance("Xem thời tiết hôm nay")
        assert "thoi" in result or "tiet" in result


# ── Layer 3: Information query domain ──

class TestInformationQuery:
    """Phrases that should NOT trigger camera activation."""

    @pytest.mark.parametrize("text", [
        "Xem thời tiết hôm nay",
        "Thời tiết ngày mai thế nào",
        "Trời mưa không",
        "Đọc báo giúp tôi",
        "Tin tức hôm nay",
        "Giá vàng bao nhiêu",
        "Tỉ giá đô la",
        "Mấy giờ rồi",
        "Hôm nay là ngày mấy",
        "Cho tôi xem thời tiết",
        "Check weather forecast",
    ])
    def test_information_queries_not_camera(self, text):
        result = classify_intent(text)
        assert result.is_camera_action is False, (
            f"'{text}' classified as camera action: intent={result.intent}, reason={result.reason}"
        )
        assert result.intent != UserIntent.CAMERA_ACTIVATE
        assert result.intent != UserIntent.VISUAL_ASSIST

    def test_weather_query_is_information_query(self):
        result = classify_intent("Xem thời tiết")
        assert result.intent == UserIntent.INFORMATION_QUERY

    def test_news_query_is_information_query(self):
        result = classify_intent("Đọc báo")
        assert result.intent == UserIntent.INFORMATION_QUERY

    def test_price_query_is_information_query(self):
        result = classify_intent("Giá vàng hôm nay")
        assert result.intent == UserIntent.INFORMATION_QUERY


# ── Layer 4: Camera-specific (should still work) ──

class TestCameraActivation:
    """Explicit camera commands should still activate camera."""

    @pytest.mark.parametrize("text", [
        "Mở camera lên",
        "Bật camera",
        "Camera",
        "Turn on camera",
        "Open camera",
    ])
    def test_explicit_camera_commands(self, text):
        result = classify_intent(text)
        assert result.is_camera_action is True
        assert result.intent == UserIntent.CAMERA_ACTIVATE

    def test_view_verb_with_demonstrative_is_visual_assist(self):
        result = classify_intent("Xem cái này")
        assert result.is_camera_action is True
        assert result.intent == UserIntent.VISUAL_ASSIST

    def test_read_label_is_visual_assist(self):
        result = classify_intent("Đọc nhãn thuốc này")
        assert result.is_camera_action is True
        assert result.intent == UserIntent.VISUAL_ASSIST

    def test_camera_deactivate(self):
        result = classify_intent("Tắt camera")
        assert result.is_camera_action is False
        assert result.intent == UserIntent.CAMERA_DEACTIVATE


# ── Layer 1: Cancel semantics (soft vs hard) ──

class TestCancelSemantics:
    """Hard cancel vs soft response."""

    # Hard cancel
    @pytest.mark.parametrize("text", [
        "Hủy",
        "Không cần",
        "Không muốn",
        "Bỏ qua",
        "Cancel",
        "Dừng lại",
        "Không gọi nữa",
        "Không bật nữa",
    ])
    def test_hard_cancel_phrases(self, text):
        result = classify_intent(text)
        assert result.intent == UserIntent.CANCEL, (
            f"'{text}' should be CANCEL, got {result.intent}"
        )

    def test_solo_khong_is_cancel(self):
        result = classify_intent("Không")
        assert result.intent == UserIntent.CANCEL

    def test_solo_thoi_is_cancel(self):
        result = classify_intent("Thôi")
        assert result.intent == UserIntent.CANCEL

    # Soft responses — should NOT be cancel
    @pytest.mark.parametrize("text", [
        "Không sao",
        "Không sao đâu",
        "Không phải",
        "Không phải đâu",
        "Không biết",
        "Không hiểu",
        "Không chắc",
    ])
    def test_soft_responses_not_cancel(self, text):
        result = classify_intent(text)
        assert result.intent != UserIntent.CANCEL, (
            f"'{text}' should NOT be CANCEL, got {result.intent}"
        )
        assert result.intent == UserIntent.SOFT_RESPONSE

    def test_khong_with_extra_words_not_cancel(self):
        """'Không' + other words = not cancel (e.g., 'Không phải cái đó')."""
        result = classify_intent("Không phải cái đó")
        assert result.intent != UserIntent.CANCEL


# ── Camera consent validation ──

class TestCameraConsent:
    def test_explicit_request_with_camera_phrase(self):
        allowed, reason = validate_camera_consent(
            "explicit_request", "Mở camera lên",
        )
        assert allowed is True

    def test_explicit_request_with_non_camera_phrase_denied(self):
        allowed, reason = validate_camera_consent(
            "explicit_request", "Xem thời tiết",
        )
        assert allowed is False

    def test_explicit_confirmation_with_yes(self):
        allowed, reason = validate_camera_consent(
            "explicit_confirmation", "Có",
        )
        assert allowed is True

    def test_explicit_confirmation_with_soft_response_denied(self):
        allowed, reason = validate_camera_consent(
            "explicit_confirmation", "Không sao",
        )
        assert allowed is False

    def test_explicit_confirmation_with_cancel_denied(self):
        allowed, reason = validate_camera_consent(
            "explicit_confirmation", "Không cần",
        )
        assert allowed is False

    def test_empty_phrase_denied(self):
        allowed, reason = validate_camera_consent(
            "explicit_request", "",
        )
        assert allowed is False


# ── Existing domain routing (regression checks) ──

class TestDomainRoutingRegression:
    def test_message_domain(self):
        result = classify_intent("Đọc tin nhắn")
        assert result.intent == UserIntent.MESSAGE
        assert result.is_camera_action is False

    def test_music_domain(self):
        result = classify_intent("Phát nhạc")
        assert result.intent == UserIntent.MUSIC
        assert result.is_camera_action is False

    def test_call_family_domain(self):
        result = classify_intent("Gọi cho mẹ")
        assert result.intent == UserIntent.CALL_FAMILY
        assert result.is_camera_action is False

    def test_navigation_domain(self):
        result = classify_intent("Chỉ đường đến bệnh viện")
        assert result.intent == UserIntent.NAVIGATION
        assert result.is_camera_action is False

    def test_calendar_domain(self):
        result = classify_intent("Xem lịch khám")
        assert result.intent == UserIntent.CALENDAR
        assert result.is_camera_action is False

    def test_emergency_domain(self):
        result = classify_intent("Cứu tôi")
        assert result.intent == UserIntent.EMERGENCY_ALERT
        assert result.is_camera_action is False
