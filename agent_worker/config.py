from functools import lru_cache

from pydantic import Field, field_validator
from pydantic_settings import BaseSettings, SettingsConfigDict


DEFAULT_AI_COST_PRICING_VERSION = "gemini-live-estimated-v1"

class Settings(BaseSettings):
    model_config = SettingsConfigDict(extra="ignore")

    livekit_url: str = Field("ws://localhost:7880", alias="LIVEKIT_URL")
    livekit_api_key: str = Field("devkey", alias="LIVEKIT_API_KEY")
    livekit_api_secret: str = Field("devsecret", alias="LIVEKIT_API_SECRET")

    go_backend_url: str = Field(
        "http://localhost:8080/api/internal",
        alias="GO_BACKEND_URL",
    )
    internal_api_key: str = Field("your-internal-api-key", alias="INTERNAL_API_KEY")

    gemini_api_key: str = Field("", alias="GEMINI_API_KEY")
    gemini_model: str = Field("gemini-3.1-flash-live-preview", alias="GEMINI_MODEL")
    gemini_voice: str = Field("Aoede", alias="GEMINI_VOICE")
    gemini_thinking_level: str = Field("minimal", alias="GEMINI_THINKING_LEVEL")
    gemini_audio_sample_rate: int = Field(16000, alias="GEMINI_AUDIO_SAMPLE_RATE")
    gemini_response_audio_sample_rate: int = Field(
        24000,
        alias="GEMINI_RESPONSE_AUDIO_SAMPLE_RATE",
    )
    gemini_video_enabled: bool = Field(True, alias="GEMINI_VIDEO_ENABLED")
    gemini_video_fps_speaking: float = Field(1.0, alias="GEMINI_VIDEO_FPS_SPEAKING")
    gemini_video_fps_idle: float = Field(0.33, alias="GEMINI_VIDEO_FPS_IDLE")
    gemini_google_search_enabled: bool = Field(
        False,
        alias="GEMINI_GOOGLE_SEARCH_ENABLED",
    )

    ai_cost_pricing_version: str = Field(
        DEFAULT_AI_COST_PRICING_VERSION,
        alias="AI_COST_PRICING_VERSION",
    )

    exa_api_key: str = Field("", alias="EXA_API_KEY")
    exa_search_url: str = Field("https://api.exa.ai/search", alias="EXA_SEARCH_URL")
    exa_search_max_results: int = Field(3, alias="EXA_SEARCH_MAX_RESULTS")

    ai_transport: str = Field("websocket", alias="AI_TRANSPORT")
    assistant_ws_host: str = Field("0.0.0.0", alias="ASSISTANT_WS_HOST")
    assistant_ws_port: int = Field(8765, alias="ASSISTANT_WS_PORT")

    @field_validator("ai_cost_pricing_version", mode="before")
    @classmethod
    def default_ai_cost_pricing_version(cls, value):
        if value is None or str(value).strip() == "":
            return DEFAULT_AI_COST_PRICING_VERSION
        return value


@lru_cache(maxsize=1)
def get_settings() -> Settings:
    return Settings()


settings = get_settings()

LIVEKIT_URL = settings.livekit_url
LIVEKIT_API_KEY = settings.livekit_api_key
LIVEKIT_API_SECRET = settings.livekit_api_secret
GO_BACKEND_URL = settings.go_backend_url
INTERNAL_API_KEY = settings.internal_api_key
GEMINI_API_KEY = settings.gemini_api_key
GEMINI_MODEL = settings.gemini_model
GEMINI_VOICE = settings.gemini_voice
GEMINI_THINKING_LEVEL = settings.gemini_thinking_level
GEMINI_AUDIO_SAMPLE_RATE = settings.gemini_audio_sample_rate
GEMINI_RESPONSE_AUDIO_SAMPLE_RATE = settings.gemini_response_audio_sample_rate
GEMINI_VIDEO_ENABLED = settings.gemini_video_enabled
GEMINI_VIDEO_FPS_SPEAKING = settings.gemini_video_fps_speaking
GEMINI_VIDEO_FPS_IDLE = settings.gemini_video_fps_idle
GEMINI_GOOGLE_SEARCH_ENABLED = settings.gemini_google_search_enabled
AI_COST_PRICING_VERSION = settings.ai_cost_pricing_version
EXA_API_KEY = settings.exa_api_key
EXA_SEARCH_URL = settings.exa_search_url
EXA_SEARCH_MAX_RESULTS = settings.exa_search_max_results
AI_TRANSPORT = settings.ai_transport.lower()
ASSISTANT_WS_HOST = settings.assistant_ws_host
ASSISTANT_WS_PORT = settings.assistant_ws_port
