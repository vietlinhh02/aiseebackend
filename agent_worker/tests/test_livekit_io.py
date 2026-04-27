import pytest

from gemini_bridge.livekit_io import AgentAudioPublisher


class FakeAudioSource:
    def __init__(self):
        self.frames = []
        self.cleared = False

    async def capture_frame(self, frame):
        self.frames.append(frame)

    def clear_queue(self):
        self.cleared = True

    async def aclose(self):
        self.closed = True


class FailingUnpublishParticipant:
    def __init__(self):
        self.unpublished = []

    async def unpublish_track(self, sid):
        self.unpublished.append(sid)
        raise RuntimeError("room failure: track not found")


@pytest.mark.anyio
async def test_agent_audio_publisher_converts_pcm_chunks_to_livekit_frames():
    source = FakeAudioSource()
    publisher = AgentAudioPublisher(local_participant=None, audio_source=source, sample_rate=24000)

    await publisher.publish_pcm(b"\x00\x00" * 480)

    assert len(source.frames) == 1
    frame = source.frames[0]
    assert frame.sample_rate == 24000
    assert frame.num_channels == 1
    assert frame.samples_per_channel == 480


def test_agent_audio_publisher_can_clear_queued_audio():
    source = FakeAudioSource()
    publisher = AgentAudioPublisher(local_participant=None, audio_source=source, sample_rate=24000)

    publisher.clear()

    assert source.cleared is True


@pytest.mark.anyio
async def test_agent_audio_publisher_close_tolerates_already_unpublished_track():
    source = FakeAudioSource()
    participant = FailingUnpublishParticipant()
    publisher = AgentAudioPublisher(
        local_participant=participant,
        audio_source=source,
        sample_rate=24000,
        _published_sid="TR_missing",
    )

    await publisher.close()

    assert participant.unpublished == ["TR_missing"]
    assert publisher._published_sid is None
    assert source.closed is True
