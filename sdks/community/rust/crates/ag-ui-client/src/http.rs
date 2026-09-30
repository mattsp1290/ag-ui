use crate::Agent;
use crate::agent::AgentError;
use crate::core::event::Event;
use crate::core::types::RunAgentInput;
use crate::core::{AgentState, FwdProps};
use crate::sse::SseResponseExt;
use crate::stream::EventStream;
use crate::thinking::ThinkingTranslator;
use ag_ui_core::types::AgentId;
use async_trait::async_trait;
use futures::StreamExt;
use log::{debug, trace};
use reqwest::header::{HeaderMap, HeaderName, HeaderValue};
use reqwest::{Client as HttpClient, Url};
use std::str::FromStr;

/// Represents an agent that communicates primarily via HTTP.
pub struct HttpAgent {
    http_client: HttpClient,
    base_url: Url,
    header_map: HeaderMap,
    agent_id: Option<AgentId>,
}

impl HttpAgent {
    pub fn new(base_url: Url, header_map: HeaderMap) -> Self {
        let http_client = HttpClient::new();
        let mut header_map: HeaderMap = header_map;

        header_map.insert("Content-Type", HeaderValue::from_static("application/json"));
        Self {
            http_client,
            base_url,
            header_map,
            agent_id: None,
        }
    }

    pub fn builder() -> HttpAgentBuilder {
        HttpAgentBuilder::new()
    }
}

pub struct HttpAgentBuilder {
    base_url: Option<Url>,
    header_map: HeaderMap,
    http_client: Option<HttpClient>,
    agent_id: Option<AgentId>,
}

impl HttpAgentBuilder {
    pub fn new() -> Self {
        Self {
            base_url: None,
            header_map: HeaderMap::new(),
            http_client: None,
            agent_id: None,
        }
    }

    /// Set the base URL from a Url instance
    pub fn with_url(mut self, base_url: Url) -> Self {
        self.base_url = Some(base_url);
        self
    }

    /// Set the base URL from a string, returning Result for validation
    pub fn with_url_str(mut self, url: &str) -> Result<Self, AgentError> {
        let parsed_url = Url::parse(url).map_err(|e| AgentError::Config {
            message: format!("Invalid URL '{url}': {e}"),
        })?;
        self.base_url = Some(parsed_url);
        Ok(self)
    }

    /// Replace all headers with the provided HeaderMap
    pub fn with_headers(mut self, header_map: HeaderMap) -> Self {
        self.header_map = header_map;
        self
    }

    /// Add a single header by name and value strings
    pub fn with_header(mut self, name: &str, value: &str) -> Result<Self, AgentError> {
        let header_name = HeaderName::from_str(name).map_err(|e| AgentError::Config {
            message: format!("Invalid header name '{value}': {e}"),
        })?;
        let header_value = HeaderValue::from_str(value).map_err(|e| AgentError::Config {
            message: format!("Invalid header value '{value}': {e}"),
        })?;
        self.header_map.insert(header_name, header_value);
        Ok(self)
    }

    /// Add a header using HeaderName and HeaderValue directly
    pub fn with_header_typed(mut self, name: HeaderName, value: HeaderValue) -> Self {
        self.header_map.insert(name, value);
        self
    }

    /// Add an authorization bearer token
    pub fn with_bearer_token(self, token: &str) -> Result<Self, AgentError> {
        let auth_value = format!("Bearer {token}");
        self.with_header("Authorization", &auth_value)
    }

    /// Set a custom HTTP client
    pub fn with_http_client(mut self, client: HttpClient) -> Self {
        self.http_client = Some(client);
        self
    }

    /// Set request timeout in seconds
    pub fn with_timeout(mut self, timeout_secs: u64) -> Self {
        let client = HttpClient::builder()
            .timeout(std::time::Duration::from_secs(timeout_secs))
            .build()
            .unwrap_or_else(|_| HttpClient::new());
        self.http_client = Some(client);
        self
    }

    /// Set Agent ID
    pub fn with_agent_id(mut self, agent_id: AgentId) -> Self {
        self.agent_id = Some(agent_id);
        self
    }

    pub fn build(self) -> Result<HttpAgent, AgentError> {
        let base_url = self.base_url.ok_or(AgentError::Config {
            message: "Base URL is required".to_string(),
        })?;

        // Validate URL scheme
        if !["http", "https"].contains(&base_url.scheme()) {
            return Err(AgentError::Config {
                message: format!("Unsupported URL scheme: {}", base_url.scheme()),
            });
        }

        let http_client = self.http_client.unwrap_or_default();

        Ok(HttpAgent {
            http_client,
            base_url,
            header_map: self.header_map,
            agent_id: self.agent_id,
        })
    }
}

impl Default for HttpAgentBuilder {
    fn default() -> Self {
        Self::new()
    }
}

#[async_trait]
impl<StateT, FwdPropsT> Agent<StateT, FwdPropsT> for HttpAgent
where
    StateT: AgentState,
    FwdPropsT: FwdProps,
{
    async fn run(
        &self,
        input: &RunAgentInput<StateT, FwdPropsT>,
    ) -> Result<EventStream<'async_trait, StateT>, AgentError> {
        // Send the request and get the response
        let response = self
            .http_client
            .post(self.base_url.clone())
            .json(input)
            .headers(self.header_map.clone())
            .send()
            .await?;

        // Check HTTP status and surface structured error on non-success
        let status = response.status();
        if !status.is_success() {
            let text = response.text().await.unwrap_or_default();
            let snippet: String = text.chars().take(512).collect();
            return Err(AgentError::HttpStatus {
                status,
                context: snippet,
            });
        }

        // Convert the response to an SSE event stream
        let mut thinking = ThinkingTranslator::default();
        let stream = response
            .event_source()
            .await
            .filter_map(move |result| {
                let parsed = match result {
                    Ok(event) => {
                        trace!("Received event: {event:?}");
                        parse_event_data::<StateT>(&event.data, &mut thinking)
                    }
                    Err(err) => Some(Err(err)),
                };
                async move { parsed }
            })
            .boxed();
        Ok(stream)
    }

    fn agent_id(&self) -> Option<&AgentId> {
        self.agent_id.as_ref()
    }
}

fn known_event_type(kind: &str) -> bool {
    matches!(
        kind,
        "ACTIVITY_SNAPSHOT"
            | "ACTIVITY_DELTA"
            | "REASONING_START"
            | "REASONING_MESSAGE_START"
            | "REASONING_MESSAGE_CONTENT"
            | "REASONING_MESSAGE_END"
            | "REASONING_MESSAGE_CHUNK"
            | "REASONING_END"
            | "REASONING_ENCRYPTED_VALUE"
            | "SUBAGENT_STARTED"
            | "SUBAGENT_FINISHED"
            | "SUBAGENT_ERROR"
            | "TEXT_MESSAGE_START"
            | "TEXT_MESSAGE_CONTENT"
            | "TEXT_MESSAGE_END"
            | "TEXT_MESSAGE_CHUNK"
            | "THINKING_TEXT_MESSAGE_START"
            | "THINKING_TEXT_MESSAGE_CONTENT"
            | "THINKING_TEXT_MESSAGE_END"
            | "TOOL_CALL_START"
            | "TOOL_CALL_ARGS"
            | "TOOL_CALL_END"
            | "TOOL_CALL_CHUNK"
            | "TOOL_CALL_RESULT"
            | "THINKING_START"
            | "THINKING_END"
            | "STATE_SNAPSHOT"
            | "STATE_DELTA"
            | "MESSAGES_SNAPSHOT"
            | "RAW"
            | "CUSTOM"
            | "RUN_STARTED"
            | "RUN_FINISHED"
            | "RUN_ERROR"
            | "STEP_STARTED"
            | "STEP_FINISHED"
    )
}

fn parse_event_data<StateT: AgentState>(
    data: &str,
    thinking: &mut ThinkingTranslator,
) -> Option<Result<Event<StateT>, AgentError>> {
    let mut raw: serde_json::Value = match serde_json::from_str(data) {
        Ok(raw) => raw,
        Err(err) => return Some(Err(err.into())),
    };
    // Validate the known legacy shape before replacing or dropping its fields.
    // In particular, THINKING_START.title must be a string when present.
    if matches!(
        raw.get("type").and_then(|value| value.as_str()),
        Some(
            "THINKING_START"
                | "THINKING_END"
                | "THINKING_TEXT_MESSAGE_START"
                | "THINKING_TEXT_MESSAGE_CONTENT"
                | "THINKING_TEXT_MESSAGE_END"
        )
    ) && let Err(error) = serde_json::from_value::<Event<StateT>>(raw.clone())
    {
        return Some(Err(error.into()));
    }
    thinking.translate(&mut raw);
    let kind = raw.get("type").and_then(|v| v.as_str()).map(str::to_owned);
    if let Some(kind) = kind.as_deref()
        && !known_event_type(kind)
    {
        log::warn!("Dropping unknown AG-UI event type: {kind}");
        return None;
    }
    if kind.as_deref() == Some("STATE_DELTA")
        && let Some(delta) = raw
            .get_mut("delta")
            .and_then(serde_json::Value::as_array_mut)
    {
        let mut index = 0;
        delta.retain(|operation| {
            let known = operation
                .get("op")
                .and_then(serde_json::Value::as_str)
                .is_none_or(|op| {
                    matches!(op, "add" | "remove" | "replace" | "move" | "copy" | "test")
                });
            if !known {
                log::warn!("Dropping unknown patch operation at /delta/{index}");
            }
            index += 1;
            known
        });
    }
    if kind.as_deref() == Some("MESSAGES_SNAPSHOT")
        && let Some(messages) = raw
            .get_mut("messages")
            .and_then(serde_json::Value::as_array_mut)
    {
        for (message_index, message) in messages.iter_mut().enumerate() {
            if let Some(parts) = message
                .get_mut("content")
                .and_then(serde_json::Value::as_array_mut)
            {
                let mut part_index = 0;
                parts.retain(|part| {
                    let known = part.get("type").and_then(serde_json::Value::as_str)
                        .is_none_or(|kind| matches!(kind, "text" | "image" | "audio" | "video" | "document"));
                    if !known {
                        log::warn!("Dropping unknown content part at /messages/{message_index}/content/{part_index}");
                    }
                    part_index += 1;
                    known
                });
            }
        }
    }
    if kind.as_deref() == Some("RUN_FINISHED")
        && raw.get("outcome").is_some_and(serde_json::Value::is_array)
    {
        return Some(Err(AgentError::exec(
            "outcome: expected object, received array",
        )));
    }
    if kind.as_deref() == Some("RUN_FINISHED")
        && let Some(outcome) = raw.get("outcome").and_then(|value| value.as_object())
        && let Some(kind) = outcome.get("type").and_then(|value| value.as_str())
        && !matches!(kind, "success" | "interrupt" | "cancelled")
    {
        log::warn!("Dropping unrecognized RUN_FINISHED outcome type: {kind}");
        raw.as_object_mut().unwrap().remove("outcome");
    }
    let malformed_field = match kind.as_deref() {
        Some("TEXT_MESSAGE_CONTENT")
            if raw.get("delta").is_some_and(|value| !value.is_string()) =>
        {
            Some("delta")
        }
        Some("TEXT_MESSAGE_START") if raw.get("role").is_some() => Some("role"),
        _ => None,
    };
    let parsed: Result<Event<StateT>, _> = serde_json::from_value(raw);
    if let Ok(ref event) = parsed {
        debug!("Deserialized event: {event:?}");
    }
    Some(parsed.map_err(|error| match malformed_field {
        Some(field) => AgentError::exec(format!("Invalid {field}: {error}")),
        None => error.into(),
    }))
}

#[cfg(test)]
mod event_parsing_tests {
    use super::*;
    use serde_json::Value;

    #[test]
    fn malformed_legacy_title_fails_before_translation() {
        for title in [
            serde_json::json!(42),
            serde_json::json!({"unexpected":"object"}),
        ] {
            let wire = serde_json::json!({"type":"THINKING_START","title":title});
            let mut thinking = ThinkingTranslator::default();
            assert!(
                parse_event_data::<Value>(&wire.to_string(), &mut thinking)
                    .unwrap()
                    .is_err()
            );
            assert!(thinking.warnings.is_empty());
        }
        let mut thinking = ThinkingTranslator::default();
        let valid = parse_event_data::<Value>(
            r#"{"type":"THINKING_START","title":"Working"}"#,
            &mut thinking,
        )
        .unwrap()
        .unwrap();
        assert!(matches!(valid, Event::ReasoningStart(_)));
        assert!(
            thinking
                .warnings
                .iter()
                .any(|warning| warning.contains("Dropping THINKING_START.title"))
        );
        let mut thinking = ThinkingTranslator::default();
        assert!(matches!(
            parse_event_data::<Value>(r#"{"type":"THINKING_START"}"#, &mut thinking)
                .unwrap()
                .unwrap(),
            Event::ReasoningStart(_)
        ));
        assert!(
            !thinking
                .warnings
                .iter()
                .any(|warning| warning.contains("Dropping THINKING_START.title"))
        );
    }

    #[tokio::test]
    async fn legacy_thinking_replays_as_reasoning_and_modern_stream_stays_quiet() {
        use crate::core::types::{RunAgentInput, RunId, ThreadId};
        use crate::event_handler::EventHandler;
        use crate::subscriber::Subscribers;
        use serde_json::json;

        for name in ["era-0-0-45-thinking-translated", "conformant-run-is-quiet"] {
            let path = format!(
                "{}/../../../../../spec/1.0/conformance/streams/{name}.json",
                env!("CARGO_MANIFEST_DIR")
            );
            let fixture: Value =
                serde_json::from_str(&std::fs::read_to_string(path).unwrap()).unwrap();
            let mut thinking = ThinkingTranslator::default();
            let input = RunAgentInput::new(
                ThreadId::from("t"),
                RunId::from("r"),
                json!({}),
                vec![],
                vec![],
                vec![],
                json!({}),
            );
            let mut handler =
                EventHandler::new(vec![], json!({}), &input, Subscribers::new(vec![]));
            let mut event_types = Vec::new();
            for raw in fixture["stream"].as_array().unwrap() {
                let event = parse_event_data::<Value>(&raw.to_string(), &mut thinking)
                    .unwrap()
                    .unwrap();
                event_types.push(
                    serde_json::to_value(&event).unwrap()["type"]
                        .as_str()
                        .unwrap()
                        .to_owned(),
                );
                let mutation = handler.handle_event(&event).await.unwrap();
                handler.apply_mutation(mutation).await.unwrap();
            }
            let expected = &fixture["expect"];
            assert_eq!(
                handler.messages.len(),
                expected["messages"].as_array().unwrap().len(),
                "{name}"
            );
            for (message, expected) in handler
                .messages
                .iter()
                .zip(expected["messages"].as_array().unwrap())
            {
                let message = serde_json::to_value(message).unwrap();
                assert_eq!(message["role"], expected["role"], "{name}");
                assert_eq!(message["content"], expected["content"], "{name}");
            }
            if name == "era-0-0-45-thinking-translated" {
                assert!(
                    thinking
                        .warnings
                        .iter()
                        .any(|warning| warning.contains("THINKING_START"))
                );
                assert!(
                    event_types
                        .iter()
                        .any(|kind| kind == "REASONING_MESSAGE_START")
                );
                assert!(!event_types.iter().any(|kind| kind.starts_with("THINKING_")));
            } else {
                assert!(thinking.warnings.is_empty());
                assert_eq!(handler.state, expected["state"]);
            }
        }
    }

    #[tokio::test]
    async fn orphan_legacy_continuation_remains_invalid_after_translation() {
        use crate::core::types::{RunAgentInput, RunId, ThreadId};
        use crate::event_handler::EventHandler;
        use crate::subscriber::Subscribers;
        use serde_json::json;

        let input = RunAgentInput::new(
            ThreadId::from("t"),
            RunId::from("r"),
            json!({}),
            vec![],
            vec![],
            vec![],
            json!({}),
        );
        let mut handler = EventHandler::new(vec![], json!({}), &input, Subscribers::new(vec![]));
        let mut thinking = ThinkingTranslator::default();
        let started = parse_event_data::<Value>(
            r#"{"type":"RUN_STARTED","threadId":"t","runId":"r"}"#,
            &mut thinking,
        )
        .unwrap()
        .unwrap();
        handler.handle_event(&started).await.unwrap();
        let orphan = parse_event_data::<Value>(
            r#"{"type":"THINKING_TEXT_MESSAGE_CONTENT","delta":"orphan"}"#,
            &mut thinking,
        )
        .unwrap()
        .unwrap();
        assert!(handler.handle_event(&orphan).await.is_err());
        assert!(
            thinking
                .warnings
                .iter()
                .any(|warning| warning.contains("Minting messageId"))
        );
    }

    #[test]
    fn unknown_optional_outcome_is_stripped_but_malformed_known_arm_fails() {
        let future = r#"{"type":"RUN_FINISHED","threadId":"t","runId":"r","outcome":{"type":"future","data":1}}"#;
        let event = parse_event_data::<Value>(future, &mut ThinkingTranslator::default())
            .unwrap()
            .unwrap();
        assert!(matches!(event, Event::RunFinished(e) if e.outcome.is_none()));
        let malformed =
            r#"{"type":"RUN_FINISHED","threadId":"t","runId":"r","outcome":{"type":"interrupt"}}"#;
        assert!(
            parse_event_data::<Value>(malformed, &mut ThinkingTranslator::default())
                .unwrap()
                .is_err()
        );
    }

    #[test]
    fn unknown_event_is_dropped_but_malformed_known_event_fails() {
        assert!(
            parse_event_data::<Value>(
                r#"{"type":"FUTURE_EVENT","value":1}"#,
                &mut ThinkingTranslator::default()
            )
            .is_none()
        );
        assert!(
            parse_event_data::<Value>(
                r#"{"type":"TEXT_MESSAGE_CONTENT","messageId":"m"}"#,
                &mut ThinkingTranslator::default()
            )
            .unwrap()
            .is_err()
        );
    }
}
