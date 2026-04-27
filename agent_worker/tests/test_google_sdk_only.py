from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]


def test_agent_worker_no_longer_depends_on_livekit_google_plugin():
    pyproject = (ROOT / "pyproject.toml").read_text()
    main = (ROOT / "main.py").read_text()

    assert "livekit-plugins-google" not in pyproject
    assert "livekit-plugins-silero" not in pyproject
    assert "livekit-plugins-noise-cancellation" not in pyproject
    assert "livekit.plugins import google" not in main
    assert "google.realtime.RealtimeModel" not in main
    assert "AgentSession" not in main


def test_main_has_single_google_sdk_entrypoint():
    import main

    assert main.entrypoint is main.google_sdk_entrypoint
    assert not hasattr(main, "livekit_plugin_entrypoint")
    assert not hasattr(main, "select_engine_entrypoint")
