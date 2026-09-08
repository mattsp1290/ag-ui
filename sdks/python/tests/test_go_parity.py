"""Python oracle for the shared Go SDK parity corpus."""

from __future__ import annotations

import enum
import hashlib
import inspect
import json
import os
import unittest
from pathlib import Path
from typing import Any

from pydantic import BaseModel, TypeAdapter, ValidationError

import ag_ui.core as core


SDK_ROOT = Path(__file__).resolve().parents[1]
PARITY_ROOT = SDK_ROOT.parent / "community" / "go" / "testdata" / "parity"
FIXTURE_PATH = PARITY_ROOT / "fixtures.json"
MANIFEST_PATH = PARITY_ROOT / "manifest.json"
OUTPUT_ENV = "AG_UI_PARITY_OUTPUT_DIR"
PHASE_ENV = "AG_UI_PARITY_PHASE"


def _load(path: Path) -> Any:
    return json.loads(path.read_text(encoding="utf-8"))


def _dump(adapter: TypeAdapter, model: Any) -> Any:
    return adapter.dump_python(model, by_alias=True, mode="json")


ADAPTERS = {
    "event": TypeAdapter(core.Event),
    "message": TypeAdapter(core.Message),
    "request": TypeAdapter(core.RunAgentInput),
    "content": TypeAdapter(core.InputContent),
    "capabilities": TypeAdapter(core.AgentCapabilities),
    "usage": TypeAdapter(core.TokenUsage),
}
USAGE = TypeAdapter(core.TokenUsage)
USAGES = TypeAdapter(list[core.TokenUsage])


def _expected(case: dict[str, Any], language: str = "python") -> Any:
    language_values = case.get("expected_by_language", {})
    return language_values[language] if language in language_values else case["expected"]


def _valid(case: dict[str, Any], language: str = "python") -> bool:
    validity = case.get("peer_validity", {})
    return validity[language] if language in validity else case["valid"]


def _exception(manifest: dict[str, Any], case_id: str, route: str) -> dict[str, Any]:
    declarations = manifest.get("route_exceptions", [])
    if isinstance(declarations, dict):
        direct = declarations.get(f"{case_id}/{route}") or declarations.get(f"{case_id}:{route}")
        if isinstance(direct, dict):
            return direct
        declarations = declarations.get(case_id, [])
        if isinstance(declarations, dict) and route in declarations:
            value = declarations[route]
            return value if isinstance(value, dict) else {}
    for declaration in declarations if isinstance(declarations, list) else []:
        if declaration.get("case_id", declaration.get("id")) == case_id and declaration.get("route") == route:
            return declaration
    return {}


def _route_expectation(case: dict[str, Any], manifest: dict[str, Any], route: str) -> tuple[bool, Any]:
    accepted, value = _valid(case), _expected(case)
    declaration = _exception(manifest, case["id"], route)
    if "accepted" in declaration:
        accepted = declaration["accepted"]
    if "value" in declaration:
        value = declaration["value"]
    return accepted, value


def _native(case: dict[str, Any]) -> Any:
    kind, value = case["kind"], case["input"]
    if kind == "aggregate":
        entries = USAGES.validate_python(value["entries"])
        return _dump(USAGES, core.aggregate_token_usage(entries))
    if kind == "mapper":
        result = core.token_usage_from_langchain_metadata(
            value["metadata"], provider=value.get("provider"), model=value.get("model")
        )
        return None if result is None else _dump(USAGE, result)
    adapter = ADAPTERS[kind]
    return _dump(adapter, adapter.validate_python(value))


def _parse_peer_value(case: dict[str, Any], value: Any) -> Any:
    """Parse the paired producer's actual value; helpers are never rerun."""
    kind = case["kind"]
    if kind == "aggregate":
        return _dump(USAGES, USAGES.validate_python(value))
    if kind == "mapper":
        return None if value is None else _dump(USAGE, USAGE.validate_python(value))
    adapter = ADAPTERS[kind]
    return _dump(adapter, adapter.validate_python(value))


def _record(case_id: str, action) -> dict[str, Any]:
    try:
        return {"id": case_id, "accepted": True, "value": action()}
    except (ValidationError, ValueError, TypeError, OverflowError) as error:
        return {"id": case_id, "accepted": False, "error": str(error)}


def _envelope(route: str, cases: list[dict[str, Any]], digest: str) -> dict[str, Any]:
    return {"version": 1, "route": route, "corpus_sha256": digest, "cases": cases}


def _produce(cases: list[dict[str, Any]], digest: str) -> dict[str, Any]:
    return _envelope("python.produced", [_record(case["id"], lambda c=case: _native(c)) for case in cases], digest)


def _validate_artifact(
    document: Any, route: str, case_ids: list[str], digest: str, kinds: dict[str, str]
) -> dict[str, dict[str, Any]]:
    if not isinstance(document, dict) or set(document) != {"version", "route", "corpus_sha256", "cases"}:
        raise AssertionError(f"{route}: malformed envelope")
    if document["version"] != 1 or document["route"] != route or document["corpus_sha256"] != digest:
        raise AssertionError(f"{route}: version, route, or corpus digest mismatch")
    records = document["cases"]
    if not isinstance(records, list) or len(records) != len(case_ids):
        raise AssertionError(f"{route}: expected {len(case_ids)} records")
    by_id: dict[str, dict[str, Any]] = {}
    for record in records:
        if not isinstance(record, dict) or "id" not in record or "accepted" not in record:
            raise AssertionError(f"{route}: every record requires id and accepted")
        if set(record) - {"id", "accepted", "value", "error", "unsupported"}:
            raise AssertionError(f"{route}/{record['id']}: unknown record property")
        case_id = record["id"]
        if case_id in by_id or type(record["accepted"]) is not bool:
            raise AssertionError(f"{route}: duplicate id or non-boolean accepted for {case_id}")
        if record["accepted"]:
            if "value" not in record or "error" in record or "unsupported" in record:
                raise AssertionError(f"{route}/{case_id}: malformed accepted record")
            if record["value"] is None and kinds.get(case_id) != "mapper":
                raise AssertionError(f"{route}/{case_id}: only mapper results may be null")
        elif "value" in record or not record.get("error"):
            raise AssertionError(f"{route}/{case_id}: malformed rejected record")
        by_id[case_id] = record
    if set(by_id) != set(case_ids):
        raise AssertionError(f"{route}: missing or unexpected case IDs")
    return by_id


def _consume(
    source: dict[str, Any], cases: list[dict[str, Any]], case_ids: list[str], digest: str
) -> dict[str, Any]:
    kinds = {case["id"]: case["kind"] for case in cases}
    paired = _validate_artifact(source, "go.direct", case_ids, digest, kinds)
    records = []
    for case in cases:
        incoming = paired[case["id"]]
        if not incoming["accepted"]:
            record = {
                "id": case["id"],
                "accepted": False,
                "error": "source rejection: " + incoming["error"],
            }
            if incoming.get("unsupported"):
                record["unsupported"] = True
            records.append(record)
            continue
        records.append(
            _record(case["id"], lambda c=case, value=incoming["value"]: _parse_peer_value(c, value))
        )
    return _envelope("python.from-go", records, digest)


def _write(output_dir: Path, name: str, document: dict[str, Any]) -> None:
    path = output_dir / name
    path.write_text(json.dumps(document, indent=2, sort_keys=True) + "\n", encoding="utf-8")


def _json_default(value: Any) -> Any:
    if isinstance(value, enum.Enum):
        return value.value
    raise TypeError(f"not JSON serializable: {type(value).__name__}")


def _inventory() -> dict[str, Any]:
    models = {
        name: value
        for name, value in inspect.getmembers(core, inspect.isclass)
        if issubclass(value, BaseModel) and value is not BaseModel
    }
    result = {}
    for name, model in models.items():
        fields = {}
        for field in model.model_fields.values():
            wire = field.serialization_alias or field.alias or field.validation_alias or field.title
            if not isinstance(wire, str):
                wire = field.alias
            descriptor = {"annotation": str(field.annotation), "required": field.is_required()}
            if not field.is_required():
                if field.default_factory is not None:
                    descriptor["default_factory"] = str(field.default_factory)
                else:
                    descriptor["default"] = json.loads(json.dumps(field.default, default=_json_default))
            fields[wire] = descriptor
        result[name] = fields
    return result


class GoParityOracleTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.fixture_bytes = FIXTURE_PATH.read_bytes()
        cls.corpus = json.loads(cls.fixture_bytes)
        cls.manifest = _load(MANIFEST_PATH)
        cls.cases = cls.corpus["cases"]
        cls.case_ids = [case["id"] for case in cls.cases]
        cls.kinds = {case["id"]: case["kind"] for case in cls.cases}
        cls.digest = hashlib.sha256(cls.fixture_bytes).hexdigest()

    def test_checkout_source_and_dynamic_schema_inventory(self) -> None:
        module_path = Path(inspect.getfile(core)).resolve()
        self.assertTrue(module_path.is_relative_to(SDK_ROOT), module_path)
        self.assertEqual(self.manifest["schema_fields"]["python"], _inventory())

    def test_native_corpus(self) -> None:
        self.assertEqual(1, self.corpus["version"])
        self.assertEqual(88, len(self.cases))
        self.assertEqual(self.manifest["case_ids"], self.case_ids)
        self.assertEqual(len(self.case_ids), len(set(self.case_ids)))
        produced = _produce(self.cases, self.digest)
        by_id = _validate_artifact(
            produced, "python.produced", self.case_ids, self.digest, self.kinds
        )
        for case in self.cases:
            accepted, expected = _route_expectation(case, self.manifest, "python.produced")
            record = by_id[case["id"]]
            self.assertEqual(accepted, record["accepted"], case["id"] + ": " + record.get("error", ""))
            if accepted:
                self.assertEqual(expected, record["value"], case["id"])

    def test_artifact_envelopes_are_strict(self) -> None:
        produced = _produce(self.cases, self.digest)
        mutations = {
            "unknown envelope property": lambda document: document.update(extra=True),
            "unknown record property": lambda document: document["cases"][0].update(extra=True),
            "missing accepted": lambda document: document["cases"][0].pop("accepted"),
            "accepted without value": lambda document: document["cases"][0].pop("value"),
            "accepted with error": lambda document: document["cases"][0].update(error="contradiction"),
            "accepted with unsupported": lambda document: document["cases"][0].update(unsupported=False),
            "rejected with value": lambda document: document["cases"][0].update(
                accepted=False, error="rejected"
            ),
            "rejected without error": lambda document: document["cases"][0].update(
                accepted=False, value=None
            ),
        }
        for name, mutate in mutations.items():
            with self.subTest(name=name):
                changed = json.loads(json.dumps(produced))
                mutate(changed)
                with self.assertRaises(AssertionError):
                    _validate_artifact(
                        changed, "python.produced", self.case_ids, self.digest, self.kinds
                    )

    def test_artifact_mode(self) -> None:
        output_value, phase = os.getenv(OUTPUT_ENV), os.getenv(PHASE_ENV)
        if output_value is None:
            self.assertIsNone(phase, f"{PHASE_ENV} requires {OUTPUT_ENV}")
            return
        self.assertTrue(output_value, f"{OUTPUT_ENV} must name a directory")
        self.assertIn(phase, {"produce", "consume", "verify"})
        output_dir = Path(output_value)
        self.assertTrue(output_dir.is_dir(), "caller must create output directory")
        files = self.manifest["generated_artifacts"]["files"]
        if phase == "produce":
            document = _produce(self.cases, self.digest)
            self.test_native_corpus()
            _write(output_dir, files["python.produced"], document)
        elif phase == "consume":
            source = _load(output_dir / files["go.direct"])
            _write(output_dir, files["python.from-go"], _consume(source, self.cases, self.case_ids, self.digest))
        else:
            documents = {}
            required_routes = self.manifest["generated_artifacts"]["required_routes"]
            self.assertEqual(
                {
                    "go.direct", "go.encoder", "python.produced", "typescript.produced",
                    "python.from-go", "typescript.from-go", "go.from-python", "go.from-typescript",
                },
                set(required_routes),
            )
            for route in required_routes:
                documents[route] = _load(output_dir / files[route])
                _validate_artifact(
                    documents[route], route, self.case_ids, self.digest, self.kinds
                )
            self.assertEqual(_produce(self.cases, self.digest), documents["python.produced"])
            replayed = _consume(documents["go.direct"], self.cases, self.case_ids, self.digest)
            self.assertEqual(replayed, documents["python.from-go"])


if __name__ == "__main__":
    unittest.main()
