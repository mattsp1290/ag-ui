"""Bounded Python SSE producer for the shared Go interoperability corpus."""

from __future__ import annotations

import inspect
import json
import os
import re
import sys
import unittest
from pathlib import Path
from typing import Any

from pydantic import TypeAdapter

import ag_ui.core as core
import ag_ui.encoder.encoder as encoder_module
from ag_ui.encoder import EventEncoder


SDK_ROOT = Path(__file__).resolve().parents[1]
FIXTURE_PATH = (
    SDK_ROOT.parent / "community" / "go" / "testdata" / "parity" / "sse-scenarios.json"
)
OUTPUT_ENV = "AG_UI_SSE_PARITY_OUTPUT_DIR"
EXPECTED_VERSION = 1
EXPECTED_IDS = {"interrupt", "resumed", "root-error"}
SAFE_ID = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]*$")

REQUEST_ADAPTER = TypeAdapter(core.RunAgentInput)
EVENT_ADAPTER = TypeAdapter(core.Event)


def _load_fixture() -> dict[str, Any]:
    document = json.loads(FIXTURE_PATH.read_text(encoding="utf-8"))
    if not isinstance(document, dict) or set(document) != {"version", "scenarios"}:
        raise AssertionError("SSE fixture must contain only version and scenarios")
    if type(document["version"]) is not int or document["version"] != EXPECTED_VERSION:
        raise AssertionError(f"unsupported SSE fixture version: {document.get('version')!r}")
    scenarios = document["scenarios"]
    if not isinstance(scenarios, list) or not scenarios:
        raise AssertionError("SSE fixture scenarios must be a non-empty array")

    ids: set[str] = set()
    for scenario in scenarios:
        if not isinstance(scenario, dict) or set(scenario) != {"id", "request", "events"}:
            raise AssertionError("each SSE scenario requires exactly id, request, and events")
        scenario_id = scenario["id"]
        if not isinstance(scenario_id, str) or not SAFE_ID.fullmatch(scenario_id):
            raise AssertionError(f"unsafe SSE scenario id: {scenario_id!r}")
        if scenario_id in ids:
            raise AssertionError(f"duplicate SSE scenario id: {scenario_id!r}")
        ids.add(scenario_id)
        events = scenario["events"]
        if not isinstance(events, list) or not events:
            raise AssertionError(f"{scenario_id}: events must be a non-empty array")
    if ids != EXPECTED_IDS:
        raise AssertionError(f"unexpected SSE scenario ids: {sorted(ids)!r}")
    return document


def _encoded_event(event_data: dict[str, Any]) -> bytes:
    event = EVENT_ADAPTER.validate_python(event_data)
    encoded = EventEncoder().encode(event)
    if not isinstance(encoded, str) or not encoded.startswith("data: ") or not encoded.endswith("\n\n"):
        raise AssertionError("EventEncoder returned malformed SSE")
    payload = json.loads(encoded[len("data: ") : -2])
    if payload != event_data:
        raise AssertionError(f"event was not preserved by EventEncoder: {event_data['type']}")
    return encoded.encode("utf-8")


class GoSSEProducerTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.fixture = _load_fixture()
        print(f"SSE producer interpreter: {sys.executable}")
        print(f"SSE producer core module: {Path(inspect.getfile(core)).resolve()}")
        print(f"SSE producer encoder module: {Path(inspect.getfile(encoder_module)).resolve()}")

    def test_uses_checkout_core_and_encoder(self) -> None:
        core_path = Path(inspect.getfile(core)).resolve()
        encoder_path = Path(inspect.getfile(encoder_module)).resolve()
        self.assertTrue(core_path.is_relative_to(SDK_ROOT), core_path)
        self.assertTrue(encoder_path.is_relative_to(SDK_ROOT), encoder_path)

    def test_validates_requests_and_preserves_encoded_events(self) -> None:
        for scenario in self.fixture["scenarios"]:
            with self.subTest(scenario=scenario["id"]):
                REQUEST_ADAPTER.validate_python(scenario["request"])
                produced = b"".join(_encoded_event(event) for event in scenario["events"])
                self.assertGreater(len(produced), 0)

    def test_optional_output_files(self) -> None:
        output_value = os.getenv(OUTPUT_ENV)
        if output_value is None:
            return
        self.assertTrue(output_value, f"{OUTPUT_ENV} must name an existing directory")
        output_dir = Path(output_value)
        self.assertTrue(output_dir.is_dir(), f"{OUTPUT_ENV} must name an existing directory")
        for scenario in self.fixture["scenarios"]:
            content = b"".join(_encoded_event(event) for event in scenario["events"])
            (output_dir / f"python-{scenario['id']}.sse").write_bytes(content)


if __name__ == "__main__":
    unittest.main()
