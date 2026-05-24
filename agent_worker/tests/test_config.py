import importlib
import sys


def load_config_module(monkeypatch):
    monkeypatch.delenv("AI_ENGINE", raising=False)

    sys.modules.pop("config", None)
    return importlib.import_module("config")


def test_config_no_longer_exposes_ai_engine_switch(monkeypatch):
    config = load_config_module(monkeypatch)

    assert not hasattr(config, "AI_ENGINE")
    assert not hasattr(config, "AIEngine")
    assert not hasattr(config, "get_ai_engine")
    assert config.get_settings().ai_transport == "websocket"
    assert config.AI_TRANSPORT == "websocket"
