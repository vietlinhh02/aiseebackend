import importlib
import runpy
import sys
import types


def install_main_stubs(monkeypatch):
    class FakeWorkerOptions:
        def __init__(self, **kwargs):
            self.entrypoint_fnc = kwargs["entrypoint_fnc"]
            self.agent_name = kwargs["agent_name"]
            self.prewarm_fnc = kwargs.get("prewarm_fnc")

    run_app_calls = []

    def fake_run_app(options):
        run_app_calls.append(options)

    agents_module = types.ModuleType("livekit.agents")
    agents_module.WorkerOptions = FakeWorkerOptions
    agents_module.cli = types.SimpleNamespace(run_app=fake_run_app)

    class FakeGoogleSDKBridge:
        def __init__(self, ctx, logger):
            self.ctx = ctx
            self.logger = logger

        async def run(self):
            return None

    class FakeGeminiBridgeWebSocketService:
        def __init__(self, **kwargs):
            self.kwargs = kwargs

        async def serve(self, host, port):
            return types.SimpleNamespace(serve_forever=lambda: None)

    class FakeGeminiLiveSettings:
        @classmethod
        def from_env(cls):
            return cls()

    class FakeBackendClient:
        def __init__(self, *args, **kwargs):
            self.args = args
            self.kwargs = kwargs

    class FakeTransportDeviceCommandChannel:
        def __init__(self, transport):
            self.transport = transport

    class FakeToolRouter:
        def __init__(self, **kwargs):
            self.kwargs = kwargs

    gemini_bridge_module = types.ModuleType("gemini_bridge")
    gemini_bridge_module.GoogleSDKBridge = FakeGoogleSDKBridge
    gemini_bridge_module.GeminiBridgeWebSocketService = FakeGeminiBridgeWebSocketService

    backend_module = types.ModuleType("backend")
    backend_module.BackendClient = FakeBackendClient

    config_module = types.ModuleType("config")
    config_module.AI_TRANSPORT = "livekit"
    config_module.ASSISTANT_WS_HOST = "127.0.0.1"
    config_module.ASSISTANT_WS_PORT = 8765
    config_module.GO_BACKEND_URL = "http://backend/api/internal"
    config_module.INTERNAL_API_KEY = "internal"

    prompts_module = types.ModuleType("prompts")
    prompts_module.select_prompt = lambda locale, wake_up, user_context: "prompt"

    core_module = types.ModuleType("gemini_bridge.core")
    core_module.GeminiLiveSettings = FakeGeminiLiveSettings

    tool_router_module = types.ModuleType("gemini_bridge.tool_router")
    tool_router_module.ToolRouter = FakeToolRouter

    transport_module = types.ModuleType("gemini_bridge.transport")
    transport_module.TransportDeviceCommandChannel = FakeTransportDeviceCommandChannel

    for name, module in {
        "main": None,
        "backend": backend_module,
        "config": config_module,
        "livekit.agents": agents_module,
        "prompts": prompts_module,
        "gemini_bridge": gemini_bridge_module,
        "gemini_bridge.core": core_module,
        "gemini_bridge.tool_router": tool_router_module,
        "gemini_bridge.transport": transport_module,
    }.items():
        if module is None:
            sys.modules.pop(name, None)
            continue
        monkeypatch.setitem(sys.modules, name, module)

    imported_main = importlib.import_module("main")
    return imported_main, run_app_calls


def test_entrypoint_is_google_sdk_entrypoint(monkeypatch):
    main_module, _ = install_main_stubs(monkeypatch)

    assert main_module.entrypoint is main_module.google_sdk_entrypoint


def test_worker_bootstrap_keeps_existing_worker_contract(monkeypatch):
    _, run_app_calls = install_main_stubs(monkeypatch)

    globals_dict = runpy.run_module("main", run_name="__main__")

    assert len(run_app_calls) == 1
    options = run_app_calls[0]
    assert options.entrypoint_fnc is globals_dict["entrypoint"]
    assert options.agent_name == "aisee-agent"
    assert options.prewarm_fnc is None
