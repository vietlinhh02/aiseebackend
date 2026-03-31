import os

LIVEKIT_URL = os.environ.get("LIVEKIT_URL", "ws://localhost:7880")
LIVEKIT_API_KEY = os.environ.get("LIVEKIT_API_KEY", "devkey")
LIVEKIT_API_SECRET = os.environ.get("LIVEKIT_API_SECRET", "devsecret")
GO_BACKEND_URL = os.environ.get("GO_BACKEND_URL", "http://localhost:8080/api/internal")
INTERNAL_API_KEY = os.environ.get("INTERNAL_API_KEY", "your-internal-api-key")
GEMINI_API_KEY = os.environ.get("GEMINI_API_KEY", "")
GEMINI_MODEL = os.environ.get("GEMINI_MODEL", "gemini-3.1-flash-live-preview")
