import importlib
import sys

from config import DEFAULT_AI_COST_PRICING_VERSION, Settings


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


def test_ai_cost_pricing_version_defaults_when_unset():
    settings = Settings()

    assert settings.ai_cost_pricing_version == DEFAULT_AI_COST_PRICING_VERSION


def test_ai_cost_pricing_version_defaults_when_blank():
    settings = Settings(AI_COST_PRICING_VERSION=" ")

    assert settings.ai_cost_pricing_version == DEFAULT_AI_COST_PRICING_VERSION
