from google.genai import types

from gemini_bridge.session import GeminiLiveSettings


def test_live_settings_build_audio_config_for_gemini_31():
    settings = GeminiLiveSettings(
        api_key="test-key",
        model="gemini-3.1-flash-live-preview",
        voice="Aoede",
        thinking_level="low",
        input_sample_rate=16000,
    )

    config = settings.build_live_config()
    audio_blob = settings.audio_blob(b"pcm")

    assert config.response_modalities == [types.Modality.AUDIO]
    assert config.speech_config.voice_config.prebuilt_voice_config.voice_name == "Aoede"
    assert config.thinking_config.thinking_level is types.ThinkingLevel.LOW
    assert (
        config.realtime_input_config.activity_handling
        is types.ActivityHandling.NO_INTERRUPTION
    )
    assert audio_blob.data == b"pcm"
    assert audio_blob.mime_type == "audio/pcm;rate=16000"
