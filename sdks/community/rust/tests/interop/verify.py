#!/usr/bin/env python3
"""Independent pinned-schema and exact-wire oracle; never calls Rust validators."""
import argparse
import copy
import hashlib
import json
from pathlib import Path
import subprocess

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[4]
PINS = json.loads((HERE / "pins.json").read_text())
SCENARIOS = json.loads((HERE / "scenarios.json").read_text())


def check_pins():
    for name, expected in PINS["sourceFiles"].items():
        actual = hashlib.sha256((ROOT / name).read_bytes()).hexdigest()
        if actual != expected:
            raise AssertionError(f"pinned TypeScript build input changed: {name}")
    # New runtime files are changes too, even when no old file was edited.
    files = subprocess.check_output(
        ["git", "ls-files", "--cached", "--others", "--exclude-standard", "--", *PINS["runtimeRoots"]],
        cwd=ROOT, text=True,
    ).splitlines()
    def excluded(name):
        return any(part in name for part in ("/src/conformance/", "/__tests__/", "/tests/")) or name.endswith((".test.ts", ".spec.ts", "README.md", "CHANGELOG.md"))
    for name in files:
        if not excluded(name) and name not in PINS["sourceFiles"]:
            raise AssertionError(f"unpinned TypeScript build input: {name}")
    schema = ROOT / "spec/1.0/schema.json"
    assert hashlib.sha256(schema.read_bytes()).hexdigest() == PINS["schemaSha256"], "schema pin changed"


def validators():
    from jsonschema import Draft202012Validator
    schema = json.loads((ROOT / "spec/1.0/schema.json").read_text())
    return (Draft202012Validator(schema), Draft202012Validator({**schema, "$ref": "#/$defs/RunAgentInput"}))


def expected_events(step):
    events = copy.deepcopy(step["response"])
    for event in events:
        if event["type"] == "RUN_STARTED":
            event["protocolVersion"] = "1.0"
    return events


def check_order(events, request):
    """Independent state machine for the live fixture's current-event subset."""
    active = False
    texts = set()
    tools = set()
    seen_tools = set()
    subagents = {}
    completed = set()
    for index, event in enumerate(events):
        kind = event["type"]
        if kind == "RUN_STARTED":
            assert not active and index == 0
            assert event["threadId"] == request["threadId"] and event["runId"] == request["runId"]
            active = True
        else:
            assert active, f"event outside active run: {kind}"
        if kind == "TEXT_MESSAGE_START":
            assert event["messageId"] not in texts
            texts.add(event["messageId"])
        elif kind == "TEXT_MESSAGE_CONTENT":
            assert event["messageId"] in texts
        elif kind == "TEXT_MESSAGE_END":
            texts.remove(event["messageId"])
        elif kind == "TOOL_CALL_START":
            assert event["toolCallId"] not in seen_tools
            tools.add(event["toolCallId"])
            seen_tools.add(event["toolCallId"])
        elif kind == "TOOL_CALL_ARGS":
            assert event["toolCallId"] in tools
        elif kind == "TOOL_CALL_END":
            tools.remove(event["toolCallId"])
        elif kind == "SUBAGENT_STARTED":
            identity = event["subagentRunId"]
            assert identity not in subagents and identity not in completed
            parent = event.get("parentSubagentRunId")
            assert parent is None or parent in subagents
            subagents[identity] = parent
        elif kind == "SUBAGENT_FINISHED":
            identity = event["subagentRunId"]
            assert identity in subagents and identity not in subagents.values()
            del subagents[identity]
            completed.add(identity)
        elif kind == "RUN_FINISHED":
            assert index == len(events) - 1 and not texts and not tools and not subagents
            assert event["threadId"] == request["threadId"] and event["runId"] == request["runId"]
            assert set(event.get("outcome", {}).get("pendingToolCallIds", [])) <= seen_tools
            active = False
        if "subagentRunId" in event and not kind.startswith("SUBAGENT_"):
            assert event["subagentRunId"] in subagents
    assert not active and events[-1]["type"] == "RUN_FINISHED"


def check_fixtures():
    event_validator, input_validator = validators()
    for steps in SCENARIOS.values():
        for step in steps:
            input_validator.validate(step["expectedRequest"])
            for event in expected_events(step):
                event_validator.validate(event)
            check_order(expected_events(step), step["expectedRequest"])


def check_row(row):
    step = SCENARIOS[row["scenario"]][row["step"]]
    trace = row["trace"]
    expected = expected_events(step)
    assert trace["request"] == step["expectedRequest"], "actual wire request mismatch"
    assert json.loads(trace["rawRequestBody"]) == step["expectedRequest"], "raw request bytes mismatch"
    assert trace["effectiveState"] == step["expectedEffectiveState"]
    assert trace["admissionError"] is None
    assert trace["headers"]["accept"] == "text/event-stream"
    assert trace["headers"]["content-type"] == "application/json"
    assert trace["headers"]["authorization"] == "Bearer interop-only-not-a-secret"
    assert trace["diagnostics"] == []
    body = trace["responseBody"]
    assert not body.startswith("\ufeff") and "\r" not in body and body.endswith("\n\n")
    parsed = []
    comments = 0
    for frame in body[:-2].split("\n\n"):
        if frame.startswith(":"):
            assert all(line.startswith(":") for line in frame.split("\n"))
            comments += 1
        else:
            assert frame.startswith("data:") and "\n" not in frame
            parsed.append(json.loads(frame.removeprefix("data:").lstrip(" ")))
    assert comments == len(expected) + 1, "keepalives are missing or became data events"
    assert parsed == expected, "actual public encoder bytes lost or changed payload"
    assert trace["responseEvents"] == expected
    assert row["events"] == expected, "application-visible event payload changed"
    assert row["messages"] == step["expectedMessages"], "full public history changed"
    assert row["state"] == step["expectedState"]
    assert row["callbacks"] == {"initialized": 1, "finalized": 1, "failed": 0}
    event_validator, input_validator = validators()
    input_validator.validate(trace["request"])
    for event in parsed:
        event_validator.validate(event)
    check_order(parsed, trace["request"])


def check_evidence(path):
    report = json.loads(path.read_text())
    rows = report["evidence"]
    expected_keys = {(name, index) for name, steps in SCENARIOS.items() for index in range(len(steps))}
    keys = [(row["scenario"], row["step"]) for row in rows]
    assert len(keys) == len(set(keys)) and set(keys) == expected_keys, "missing/duplicate/unknown exchanges"
    assert report["exchanges"] == len(expected_keys)
    for row in rows:
        check_row(row)
    # Deliberate mutations prove that exact-wire and application assertions bite.
    for change in ("event", "request", "null", "warning", "body"):
        original = next(row for row in rows if row["scenario"] == "null_restart" and row["step"] == 0) if change == "null" else rows[0]
        row = copy.deepcopy(original)
        if change == "event": row["events"].pop()
        elif change == "request": row["trace"]["request"]["runId"] = "wrong"
        elif change == "null": row.pop("state")
        elif change == "warning": row["trace"]["diagnostics"].append({"unexpected": True})
        else: row["trace"]["responseBody"] += "data: {}\n\n"
        try:
            check_row(row)
        except (AssertionError, KeyError):
            pass
        else:
            raise AssertionError(f"negative control failed to detect {change}")
    return len(rows)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--pins-only", action="store_true")
    parser.add_argument("evidence", nargs="*", type=Path)
    args = parser.parse_args()
    check_pins()
    if args.pins_only:
        print(f"Pinned TypeScript {PINS['clientVersion']} source and schema verified")
        return
    check_fixtures()
    for evidence in args.evidence:
        count = check_evidence(evidence)
        print(f"Independent schema/framing/history oracle: {count} exchanges + 5 negative controls ({evidence.name})")
    if not args.evidence:
        print("Authored fixture requests, events, and ordering pass the pinned independent schema")


if __name__ == "__main__":
    main()
