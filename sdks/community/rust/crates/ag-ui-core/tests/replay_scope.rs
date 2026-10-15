use ag_ui_core::{Replay, event::Event, types::Role};
use serde_json::{Value, json};
fn event(value: Value) -> Event {
    serde_json::from_value(value).unwrap()
}
#[test]
fn replay_preserves_stream_roles_metadata_and_attribution() {
    for (role, expected) in [
        ("user", Role::User),
        ("system", Role::System),
        ("developer", Role::Developer),
        ("assistant", Role::Assistant),
    ] {
        let mut replay = Replay::default();
        replay.apply(&event(json!({"type":"TEXT_MESSAGE_START","messageId":"m","role":role,"name":"Ada","metadata":{"nested":null},"subagentRunId":"s"}))).unwrap();
        replay
            .apply(&event(
                json!({"type":"TEXT_MESSAGE_CONTENT","messageId":"m","delta":"hello"}),
            ))
            .unwrap();
        assert_eq!(replay.messages[0].role(), expected);
        let value = serde_json::to_value(&replay.messages[0]).unwrap();
        assert_eq!(value["metadata"], json!({"nested":null}));
        assert_eq!(value["subagentRunId"], "s");
        assert_eq!(value["name"], "Ada");
    }
}
#[test]
fn replay_state_patch_failure_is_atomic_and_null_snapshot_is_real() {
    let mut replay = Replay {
        state: json!({"keep":1}),
        ..Default::default()
    };
    let before = replay.state.clone();
    let result=replay.apply(&event(json!({"type":"STATE_DELTA","delta":[{"op":"add","path":"/a","value":2},{"op":"remove","path":"/missing"}]})));
    assert!(result.is_err());
    assert_eq!(replay.state, before);
    replay
        .apply(&event(json!({"type":"STATE_SNAPSHOT","snapshot":null})))
        .unwrap();
    assert!(replay.state.is_null());
    replay
        .apply(&event(
            json!({"type":"STATE_DELTA","delta":[{"op":"replace","path":"","value":[null,1]}]}),
        ))
        .unwrap();
    assert_eq!(replay.state, json!([null, 1]));
}
#[test]
fn unsupported_and_contextless_chunks_fail_explicitly() {
    let mut replay = Replay::default();
    for raw in [
        json!({"type":"ACTIVITY_SNAPSHOT","messageId":"a","activityType":"work","content":{}}),
        json!({"type":"TEXT_MESSAGE_CHUNK","delta":"x"}),
        json!({"type":"REASONING_START","messageId":"r"}),
    ] {
        assert!(replay.apply(&event(raw)).is_err());
    }
    assert!(replay.messages.is_empty());
}

#[test]
fn supported_text_events_merge_metadata_keywise_including_end_and_chunks() {
    let mut replay = Replay::default();
    for raw in [
        json!({"type":"TEXT_MESSAGE_START","messageId":"m","role":"user","metadata":{"keep":1,"nested":{"old":1},"last":"start"}}),
        json!({"type":"TEXT_MESSAGE_CONTENT","messageId":"m","delta":"a","metadata":{"nested":{"new":null},"last":"content"}}),
        json!({"type":"TEXT_MESSAGE_CHUNK","messageId":"m","delta":"b","name":"new name","metadata":{"null":null,"last":"chunk"}}),
        json!({"type":"TEXT_MESSAGE_END","messageId":"m","metadata":{"last":"end"}}),
    ] {
        replay.apply(&event(raw)).unwrap();
    }
    let value = serde_json::to_value(&replay.messages[0]).unwrap();
    assert_eq!(
        value["metadata"],
        json!({"keep":1,"nested":{"new":null},"null":null,"last":"end"})
    );
    assert_eq!(value["content"], "ab");
    assert_eq!(value["role"], "user");
    assert_eq!(value["name"], "new name");
}
