from pathlib import Path

import yaml

from backend_models import (
    AICostRecordPayload,
    QuotaCheckPayload,
    ToolCallPayload,
    TrackingEventPayload,
    UsageRecordPayload,
    UsageSessionCommitPayload,
    UsageSessionStartPayload,
)


def _load_openapi() -> dict:
    root = Path(__file__).resolve().parents[2]
    path = root / "api" / "openapi" / "internal.yaml"
    return yaml.safe_load(path.read_text())


def test_worker_payload_fields_match_openapi_contract():
    schemas = _load_openapi()["components"]["schemas"]
    cases = {
        "ToolCallRequest": ToolCallPayload,
        "QuotaCheckInput": QuotaCheckPayload,
        "UsageRecordInput": UsageRecordPayload,
        "UsageSessionInput": UsageSessionStartPayload,
        "CommitUsageSessionInput": UsageSessionCommitPayload,
        "AICostEventInput": AICostRecordPayload,
        "EventInput": TrackingEventPayload,
    }

    for schema_name, model in cases.items():
        schema = schemas[schema_name]
        schema_properties = set(schema["properties"].keys())
        model_properties = set(model.model_fields.keys())
        assert model_properties <= schema_properties, (
            f"{model.__name__} fields drifted from OpenAPI {schema_name}: "
            f"{sorted(model_properties - schema_properties)}"
        )


def test_worker_request_paths_stay_bound_to_expected_schemas():
    doc = _load_openapi()
    expected = {
        "/tool-call": "ToolCallRequest",
        "/usage/check": "QuotaCheckInput",
        "/usage/record": "UsageRecordInput",
        "/usage/session/start": "UsageSessionInput",
        "/usage/session/commit": "CommitUsageSessionInput",
        "/ai-cost/record": "AICostEventInput",
        "/tracking/events": "EventInput",
    }

    for path, schema_name in expected.items():
        request_schema = (
            doc["paths"][path]["post"]["requestBody"]["content"]["application/json"]["schema"]["$ref"]
        )
        assert request_schema == f"#/components/schemas/{schema_name}"
