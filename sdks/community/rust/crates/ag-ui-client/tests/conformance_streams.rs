use ag_ui_client::agent::RunAgentResult;
use ag_ui_client::core::types::Message;
use ag_ui_client::subscriber::{AgentSubscriber, AgentSubscriberParams};
use ag_ui_client::{Agent, HttpAgent, RunAgentParams};
use serde_json::{Value, json};
use std::collections::{HashMap, HashSet};
use std::path::PathBuf;
use std::sync::{Arc, Mutex};
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::net::TcpListener;

#[derive(Clone, Default)]
struct FinalMessages(Arc<Mutex<Vec<Message>>>);

#[async_trait::async_trait]
impl AgentSubscriber for FinalMessages {
    async fn on_run_finalized(
        &self,
        params: AgentSubscriberParams<'async_trait, Value, Value>,
    ) -> Result<ag_ui_client::agent::AgentStateMutation<Value>, ag_ui_client::agent::AgentError>
    {
        *self.0.lock().unwrap() = params.messages.to_vec();
        Ok(Default::default())
    }
}

fn fixture_dir() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../../../../spec/1.0/conformance/streams")
}

fn assert_subset(actual: &Value, expected: &Value, path: &str) {
    match expected {
        Value::Object(fields) => {
            for (key, value) in fields {
                let next = format!("{path}.{key}");
                assert_subset(&actual[key], value, &next);
            }
        }
        Value::Array(items) => {
            let values = actual
                .as_array()
                .unwrap_or_else(|| panic!("{path}: expected array, got {actual}"));
            assert_eq!(values.len(), items.len(), "{path}: array length");
            for (index, item) in items.iter().enumerate() {
                assert_subset(&values[index], item, &format!("{path}[{index}]"));
            }
        }
        _ => assert_eq!(actual, expected, "{path}"),
    }
}

async fn replay(
    fixture: &Value,
) -> (
    Result<RunAgentResult<Value>, ag_ui_client::agent::AgentError>,
    Vec<Message>,
) {
    let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
    let address = listener.local_addr().unwrap();
    let frames: String = fixture["stream"]
        .as_array()
        .unwrap()
        .iter()
        .map(|event| format!("data: {event}\n\n"))
        .collect();
    let server = tokio::spawn(async move {
        let (mut socket, _) = listener.accept().await.unwrap();
        let mut request = Vec::new();
        let mut buffer = [0_u8; 4096];
        loop {
            let read = socket.read(&mut buffer).await.unwrap();
            assert!(read > 0, "request headers ended unexpectedly");
            request.extend_from_slice(&buffer[..read]);
            if request.windows(4).any(|window| window == b"\r\n\r\n") {
                break;
            }
        }
        let header = format!(
            "HTTP/1.1 200 OK\r\nContent-Type: text/event-stream\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
            frames.len()
        );
        socket.write_all(header.as_bytes()).await.unwrap();
        socket.write_all(frames.as_bytes()).await.unwrap();
    });
    let agent = HttpAgent::builder()
        .with_url_str(&format!("http://{address}/"))
        .unwrap()
        .build()
        .unwrap();
    let mut params = RunAgentParams::new();
    params.state = json!({});
    if let Some(initial) = fixture.pointer("/input/messages") {
        params.messages = serde_json::from_value(initial.clone()).unwrap();
    }
    let final_messages = FinalMessages::default();
    let result = agent.run_agent(&params, (final_messages.clone(),)).await;
    server.await.unwrap();
    let messages = final_messages.0.lock().unwrap().clone();
    (result, messages)
}

#[tokio::test]
async fn manifest_streams_replay_through_http_agent() {
    let revision = std::process::Command::new("git")
        .args(["rev-parse", "HEAD"])
        .current_dir(env!("CARGO_MANIFEST_DIR"))
        .output()
        .unwrap();
    assert!(revision.status.success());
    let revision = String::from_utf8(revision.stdout).unwrap();
    // The two SDK crates share a version; read the core package field to verify it.
    let core_manifest = std::fs::read_to_string(
        PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../ag-ui-core/Cargo.toml"),
    )
    .unwrap();
    let core_package = core_manifest
        .split("[package]")
        .nth(1)
        .unwrap()
        .split('[')
        .next()
        .unwrap();
    let core_version = core_package
        .lines()
        .find_map(|line| {
            line.trim()
                .strip_prefix("version = \"")
                .and_then(|value| value.strip_suffix('"'))
        })
        .unwrap();
    assert_eq!(core_version, env!("CARGO_PKG_VERSION"));
    println!("ag-ui-core {core_version} @ {}", revision.trim());

    let directory = fixture_dir();
    let manifest = std::fs::read_to_string(directory.join("MANIFEST.txt")).unwrap();
    let names: Vec<_> = manifest
        .lines()
        .map(str::trim)
        .filter(|line| !line.is_empty() && !line.starts_with('#'))
        .collect();
    let skips = HashMap::from([
        (
            "era-0-0-39-flattens-content.json",
            "0.0.39 request content flattening shim is outside this 1.0 client slice",
        ),
        (
            "era-0-0-47-upgrades-binary-content.json",
            "0.0.47 binary request upgrade shim is outside this 1.0 client slice",
        ),
        (
            "era-current-peer-keeps-modern-content.json",
            "request contains retired binary content requiring the out-of-scope binary-to-image compatibility shim",
        ),
        (
            "era-0-0-57-subagent-dropped-with-warning.json",
            "0.0.57 subagent compatibility shim is outside this 1.0 client slice",
        ),
    ]);
    let listed: HashSet<_> = names.iter().copied().collect();
    assert_eq!(listed.len(), names.len(), "duplicate manifest entries");
    for skipped in skips.keys() {
        assert!(listed.contains(skipped), "stale skip: {skipped}");
    }
    let on_disk: HashSet<_> = std::fs::read_dir(&directory)
        .unwrap()
        .map(|entry| entry.unwrap().file_name().into_string().unwrap())
        .filter(|name| name.ends_with(".json"))
        .collect();
    assert_eq!(
        listed
            .iter()
            .map(|name| name.to_string())
            .collect::<HashSet<_>>(),
        on_disk,
        "manifest does not cover corpus"
    );

    let mut failures = Vec::new();
    let mut passed = 0_usize;
    for name in names {
        if let Some(reason) = skips.get(name) {
            println!("SKIP {name}: {reason}");
            continue;
        }
        let fixture: Value =
            serde_json::from_str(&std::fs::read_to_string(directory.join(name)).unwrap()).unwrap();
        assert_eq!(
            fixture["name"],
            name.trim_end_matches(".json"),
            "{name}: fixture name"
        );
        let (result, messages) = replay(&fixture).await;
        let check = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
            let expected = &fixture["expect"];
            match expected["outcome"].as_str().unwrap() {
                "completed" => {
                    assert!(result.is_ok(), "{name}: expected completed, got {result:?}");
                }
                "failed" => {
                    assert!(result.is_err(), "{name}: expected failed");
                }
                other => panic!("{name}: unexpected expected outcome {other}"),
            }
            if let Some(fragment) = expected["errorContains"].as_str() {
                let error = result.as_ref().unwrap_err().to_string();
                assert!(
                    error.contains(fragment),
                    "{name}: expected error containing {fragment:?}, got {error:?}"
                );
            }
            if let Some(count) = expected["messageCount"].as_u64() {
                assert_eq!(messages.len() as u64, count, "{name}: message count");
            }
            if let Some(expected_messages) = expected["messages"].as_array() {
                assert_eq!(
                    messages.len(),
                    expected_messages.len(),
                    "{name}: expected messages length"
                );
                for (index, (actual, expected_message)) in
                    messages.iter().zip(expected_messages).enumerate()
                {
                    assert_subset(
                        &serde_json::to_value(actual).unwrap(),
                        expected_message,
                        &format!("{name}.messages[{index}]"),
                    );
                }
            }
            if let Some(expected_state) = expected.get("state") {
                assert_eq!(
                    &result.as_ref().unwrap().new_state,
                    expected_state,
                    "{name}: state"
                );
            }
        }));
        if let Err(problem) = check {
            let detail = problem
                .downcast_ref::<String>()
                .map(String::as_str)
                .or_else(|| problem.downcast_ref::<&str>().copied())
                .unwrap_or("unknown assertion failure");
            failures.push(format!("{name}: {detail}"));
            println!("FAIL {name}: {detail}");
        } else {
            passed += 1;
            println!("PASS {name}");
        }
    }
    println!(
        "{passed} passed, {} skipped, {} failed",
        skips.len(),
        failures.len()
    );
    assert!(failures.is_empty(), "{}", failures.join("\n"));
}
