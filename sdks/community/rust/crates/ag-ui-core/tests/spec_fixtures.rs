use ag_ui_core::{event::Event, types::*};
use serde_json::Value;
use std::{fs, path::PathBuf};

fn fixtures() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../../../../spec/1.0/fixtures")
}
fn parse(kind: &str, bytes: &[u8]) -> Result<(), String> {
    macro_rules! typed {
        ($type:ty) => {
            serde_json::from_slice::<$type>(bytes)
                .map(|_| ())
                .map_err(|e| e.to_string())
        };
    }
    match kind {
        "RunAgentInput" => typed!(RunAgentInput),
        "UserMessage" => typed!(Message),
        "ToolMessage" => typed!(Message),
        "Tool" => typed!(Tool),
        "FileSource" => typed!(PartSource),
        "Interrupt" => typed!(Interrupt),
        "ResumeEntry" => typed!(ResumeEntry),
        "SubagentInfo" => typed!(SubagentInfo),
        "AgentCapabilities" => typed!(AgentCapabilities),
        "ExecutionCapabilities" => typed!(ExecutionCapabilities),
        "MultiAgentCapabilities" => typed!(MultiAgentCapabilities),
        _ => typed!(Event),
    }
}
#[test]
fn valid_spec_fixtures_parse() {
    let mut failures = Vec::new();
    let mut count = 0;
    for dir in fs::read_dir(fixtures()).unwrap() {
        let dir = dir.unwrap();
        let kind = dir.file_name().into_string().unwrap();
        let valid = dir.path().join("valid");
        if !valid.exists() {
            continue;
        }
        for entry in fs::read_dir(valid).unwrap() {
            let path = entry.unwrap().path();
            if path.extension().is_none_or(|e| e != "json") {
                continue;
            }
            count += 1;
            let bytes = fs::read(&path).unwrap();
            if let Err(error) = parse(&kind, &bytes) {
                failures.push(format!("{}: {}", path.display(), error));
            }
        }
    }
    assert!(
        failures.is_empty(),
        "{} of {} valid fixtures failed:\n{}",
        failures.len(),
        count,
        failures.join("\n")
    );
}
#[test]
fn invalid_spec_fixtures_rejected_or_classified() {
    let mut accepted = Vec::new();
    let mut count = 0;
    for dir in fs::read_dir(fixtures()).unwrap() {
        let dir = dir.unwrap();
        let kind = dir.file_name().into_string().unwrap();
        let invalid = dir.path().join("invalid");
        if !invalid.exists() {
            continue;
        }
        for entry in fs::read_dir(invalid).unwrap() {
            let path = entry.unwrap().path();
            if !path.to_string_lossy().ends_with(".json")
                || path.to_string_lossy().ends_with(".expect.json")
            {
                continue;
            }
            count += 1;
            let bytes = fs::read(&path).unwrap();
            if parse(&kind, &bytes).is_ok() {
                accepted.push(format!(
                    "{}/{}",
                    kind,
                    path.file_name().unwrap().to_string_lossy()
                ));
            }
        }
    }
    accepted.sort();
    assert_eq!(
        accepted,
        vec![
            "MessagesSnapshotEvent/message-metadata-null.json",
            "TextMessageEndEvent/unknown-property.json",
        ],
        "new invalid-fixture tolerance among {count} fixtures"
    );
}
#[test]
fn conformant_run_replays_to_message_and_state() {
    let bytes =
        fs::read(fixtures().join("../conformance/streams/conformant-run-is-quiet.json")).unwrap();
    let fixture: Value = serde_json::from_slice(&bytes).unwrap();
    let events: Vec<Event> = fixture["stream"]
        .as_array()
        .unwrap()
        .iter()
        .map(|v| serde_json::from_value(v.clone()).unwrap())
        .collect();
    let mut replay = ag_ui_core::Replay::default();
    replay.replay(&events).unwrap();
    assert_eq!(replay.messages.len(), 1);
    assert_eq!(replay.messages[0].content(), Some("Hello, world."));
    assert_eq!(replay.state, serde_json::json!({"answered": true}));
}
#[test]
fn opaque_ids_and_optional_fields_roundtrip() {
    let wire =
        serde_json::json!({"type":"RUN_STARTED","threadId":"thread/with spaces:☃","runId":"run-α"});
    let event: Event = serde_json::from_value(wire.clone()).unwrap();
    assert_eq!(serde_json::to_value(event).unwrap(), wire);
    let input: RunAgentInput = serde_json::from_value(
        serde_json::json!({"threadId":"any string","runId":"r-1","messages":[]}),
    )
    .unwrap();
    let encoded = serde_json::to_value(&input).unwrap();
    assert_eq!(encoded["threadId"], "any string");
    assert!(encoded.get("protocolVersion").is_none());
    assert!(encoded.get("parentRunId").is_none());
    assert!(encoded.get("resume").is_none());
    let message: Message = serde_json::from_value(serde_json::json!({"id":"m-with / punctuation","role":"user","content":[{"type":"text","text":"hello"}]})).unwrap();
    assert_eq!(
        serde_json::to_value(message).unwrap()["id"],
        "m-with / punctuation"
    );
}
