"""Typed request/response boundaries for Go internal API."""

from __future__ import annotations

from typing import Any

from pydantic import BaseModel, ConfigDict, Field, RootModel


class BackendPayload(BaseModel):
    model_config = ConfigDict(extra="forbid", str_strip_whitespace=True)


class BackendJSONResponse(RootModel[dict[str, Any]]):
    """Internal API responses should always be JSON objects."""


class ToolCallPayload(BackendPayload):
    tool: str = Field(min_length=1)
    user_id: str = Field(min_length=1)
    params: dict[str, Any]


class QuotaCheckPayload(BackendPayload):
    user_id: str = Field(min_length=1)
    usage_type: str = Field(min_length=1)
    quantity: float = Field(gt=0)


class UsageRecordPayload(BackendPayload):
    user_id: str = Field(min_length=1)
    usage_type: str = Field(min_length=1)
    quantity: float = Field(gt=0)
    unit: str = Field(min_length=1)
    source: str = Field(min_length=1)
    source_id: str = Field(min_length=1)
    metadata: dict[str, Any]


class UsageSessionStartPayload(BackendPayload):
    user_id: str = Field(min_length=1)
    usage_type: str = Field(min_length=1)
    unit: str = Field(min_length=1)
    source: str = Field(min_length=1)
    source_id: str = Field(min_length=1)
    started_at: str = Field(min_length=1)
    metadata: dict[str, Any]


class UsageSessionCommitPayload(BackendPayload):
    user_id: str = Field(min_length=1)
    usage_type: str = Field(min_length=1)
    unit: str = Field(min_length=1)
    source: str = Field(min_length=1)
    source_id: str = Field(min_length=1)
    started_at: str = Field(min_length=1)
    ended_at: str = Field(min_length=1)
    metadata: dict[str, Any]


class AICostRecordPayload(BackendPayload):
    user_id: str = Field(min_length=1)
    provider: str = Field(min_length=1)
    model: str = Field(min_length=1)
    modality: str = Field(min_length=1)
    source: str = Field(min_length=1)
    source_id: str = Field(min_length=1)
    input_tokens: int = Field(ge=0)
    output_tokens: int = Field(ge=0)
    audio_input_seconds: float = Field(ge=0)
    audio_output_seconds: float = Field(ge=0)
    image_count: int = Field(ge=0)
    video_seconds: float = Field(ge=0)
    cost_usd: float = Field(ge=0)
    cost_vnd: float = Field(ge=0)
    currency: str = Field(min_length=1)
    estimated: bool
    pricing_version: str = Field(min_length=1)
    occurred_at: str = Field(min_length=1)
    metadata: dict[str, Any]


class TrackingEventPayload(BackendPayload):
    event_type: str = Field(min_length=1)
    category: str = Field(min_length=1)
    user_id: str = Field(min_length=1)
    severity: str = ""
    source: str = Field(default="agent_worker", min_length=1)
    source_id: str = Field(default="unknown", min_length=1)
    metadata: dict[str, Any]
