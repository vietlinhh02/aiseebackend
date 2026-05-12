import os


LIVEKIT_URL = os.environ.get("LIVEKIT_URL", "ws://localhost:7880")
LIVEKIT_API_KEY = os.environ.get("LIVEKIT_API_KEY", "devkey")
LIVEKIT_API_SECRET = os.environ.get("LIVEKIT_API_SECRET", "devsecret")
GO_BACKEND_URL = os.environ.get("GO_BACKEND_URL", "http://localhost:8080/api/internal")
INTERNAL_API_KEY = os.environ.get("INTERNAL_API_KEY", "your-internal-api-key")
GEMINI_API_KEY = os.environ.get("GEMINI_API_KEY", "")
GEMINI_MODEL = os.environ.get("GEMINI_MODEL", "gemini-3.1-flash-live-preview")
GEMINI_VOICE = os.environ.get("GEMINI_VOICE", "Aoede")
GEMINI_THINKING_LEVEL = os.environ.get("GEMINI_THINKING_LEVEL", "low")
GEMINI_AUDIO_SAMPLE_RATE = int(os.environ.get("GEMINI_AUDIO_SAMPLE_RATE", "16000"))
GEMINI_RESPONSE_AUDIO_SAMPLE_RATE = int(os.environ.get("GEMINI_RESPONSE_AUDIO_SAMPLE_RATE", "24000"))
GEMINI_VIDEO_ENABLED = os.environ.get("GEMINI_VIDEO_ENABLED", "true").lower() == "true"
GEMINI_VIDEO_FPS_SPEAKING = float(os.environ.get("GEMINI_VIDEO_FPS_SPEAKING", "1"))
GEMINI_VIDEO_FPS_IDLE = float(os.environ.get("GEMINI_VIDEO_FPS_IDLE", "0.33"))
GEMINI_GOOGLE_SEARCH_ENABLED = os.environ.get("GEMINI_GOOGLE_SEARCH_ENABLED", "false").lower() == "true"
AI_COST_PRICING_VERSION = os.environ.get("AI_COST_PRICING_VERSION", "")
EXA_API_KEY = os.environ.get("EXA_API_KEY", "")
EXA_SEARCH_URL = os.environ.get("EXA_SEARCH_URL", "https://api.exa.ai/search")
EXA_SEARCH_MAX_RESULTS = int(os.environ.get("EXA_SEARCH_MAX_RESULTS", "3"))
AI_TRANSPORT = os.environ.get("AI_TRANSPORT", "websocket").lower()
ASSISTANT_WS_HOST = os.environ.get("ASSISTANT_WS_HOST", "0.0.0.0")
ASSISTANT_WS_PORT = int(os.environ.get("ASSISTANT_WS_PORT", "8765"))
