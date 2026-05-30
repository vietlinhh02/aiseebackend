"""Tests for camera watchdog timer in AgentRuntimeState."""

import asyncio
import pytest

from gemini_bridge.agent_runtime_state import (
    AgentRuntimeState,
    CAMERA_WATCHDOG_TIMEOUT_SECONDS,
)


@pytest.mark.anyio
async def test_watchdog_fires_after_timeout():
    """Watchdog should fire and call deactivate callback after timeout."""
    state = AgentRuntimeState(session_id="s1", user_id="u1")

    deactivated = asyncio.Event()

    async def fake_deactivate():
        deactivated.set()

    state.set_camera_deactivate_callback(fake_deactivate)

    # Monkey-patch timeout to be short for test
    import gemini_bridge.agent_runtime_state as mod
    original_timeout = mod.CAMERA_WATCHDOG_TIMEOUT_SECONDS
    mod.CAMERA_WATCHDOG_TIMEOUT_SECONDS = 0.1

    try:
        state.on_camera_activated()
        assert state.camera.enabled is True

        # Wait for watchdog to fire
        await asyncio.wait_for(deactivated.wait(), timeout=2.0)
        assert deactivated.is_set()
    finally:
        mod.CAMERA_WATCHDOG_TIMEOUT_SECONDS = original_timeout


@pytest.mark.anyio
async def test_watchdog_cancelled_when_camera_deactivated():
    """Watchdog should be cancelled when camera is deactivated normally."""
    state = AgentRuntimeState(session_id="s1", user_id="u1")

    called = asyncio.Event()

    async def fake_deactivate():
        called.set()

    state.set_camera_deactivate_callback(fake_deactivate)

    import gemini_bridge.agent_runtime_state as mod
    original_timeout = mod.CAMERA_WATCHDOG_TIMEOUT_SECONDS
    mod.CAMERA_WATCHDOG_TIMEOUT_SECONDS = 0.2

    try:
        state.on_camera_activated()
        assert state._camera_watchdog_task is not None

        # Deactivate normally before timeout
        state.on_camera_deactivated()
        assert state._camera_watchdog_task is None

        # Wait to confirm watchdog did NOT fire
        await asyncio.sleep(0.5)
        assert not called.is_set()
    finally:
        mod.CAMERA_WATCHDOG_TIMEOUT_SECONDS = original_timeout


@pytest.mark.anyio
async def test_watchdog_restarts_on_reactivation():
    """Watchdog should restart when camera is re-activated."""
    state = AgentRuntimeState(session_id="s1", user_id="u1")

    call_count = 0

    async def fake_deactivate():
        nonlocal call_count
        call_count += 1

    state.set_camera_deactivate_callback(fake_deactivate)

    import gemini_bridge.agent_runtime_state as mod
    original_timeout = mod.CAMERA_WATCHDOG_TIMEOUT_SECONDS
    mod.CAMERA_WATCHDOG_TIMEOUT_SECONDS = 0.1

    try:
        # First activation
        state.on_camera_activated()
        task1 = state._camera_watchdog_task
        await asyncio.sleep(0.2)

        # Re-activate
        state.on_camera_activated()
        task2 = state._camera_watchdog_task
        assert task2 is not task1

        await asyncio.sleep(0.2)
    finally:
        mod.CAMERA_WATCHDOG_TIMEOUT_SECONDS = original_timeout


@pytest.mark.anyio
async def test_watchdog_state_updates_on_camera_blocked():
    """Camera blocked should also cancel watchdog."""
    state = AgentRuntimeState(session_id="s1", user_id="u1")
    state.on_camera_activated()
    assert state._camera_watchdog_task is not None

    state.on_camera_blocked("nsfw_detected")
    assert state._camera_watchdog_task is None
    assert state.camera.enabled is False
