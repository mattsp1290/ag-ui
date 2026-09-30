use crate::Agent;
use crate::agent::AgentError;
use crate::core::event::Event;
use crate::core::types::RunAgentInput;
use crate::core::{AgentState, FwdProps};
use crate::sse::SseResponseExt;
use crate::stream::EventStream;
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
        let stream = response
            .event_source()
            .await
            .filter_map(|result| async move {
                match result {
                    Ok(event) => {
                        trace!("Received event: {event:?}");
                        parse_event_data::<StateT>(&event.data)
                    }
                    Err(err) => Some(Err(err)),
                }
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

fn parse_event_data<StateT: AgentState>(data: &str) -> Option<Result<Event<StateT>, AgentError>> {
    let mut raw: serde_json::Value = match serde_json::from_str(data) {
        Ok(raw) => raw,
        Err(err) => return Some(Err(err.into())),
    };
    let kind = raw.get("type").and_then(|v| v.as_str());
    if let Some(kind) = kind
        && !known_event_type(kind)
    {
        log::warn!("Dropping unknown AG-UI event type: {kind}");
        return None;
    }
    if kind == Some("RUN_FINISHED")
        && let Some(outcome) = raw.get("outcome").and_then(|value| value.as_object())
        && let Some(kind) = outcome.get("type").and_then(|value| value.as_str())
        && !matches!(kind, "success" | "interrupt" | "cancelled")
    {
        log::warn!("Dropping unrecognized RUN_FINISHED outcome type: {kind}");
        raw.as_object_mut().unwrap().remove("outcome");
    }
    let parsed: Result<Event<StateT>, _> = serde_json::from_value(raw);
    if let Ok(ref event) = parsed {
        debug!("Deserialized event: {event:?}");
    }
    Some(parsed.map_err(Into::into))
}

#[cfg(test)]
mod event_parsing_tests {
    use super::*;
    use serde_json::Value;

    #[test]
    fn unknown_optional_outcome_is_stripped_but_malformed_known_arm_fails() {
        let future = r#"{"type":"RUN_FINISHED","threadId":"t","runId":"r","outcome":{"type":"future","data":1}}"#;
        let event = parse_event_data::<Value>(future).unwrap().unwrap();
        assert!(matches!(event, Event::RunFinished(e) if e.outcome.is_none()));
        let malformed =
            r#"{"type":"RUN_FINISHED","threadId":"t","runId":"r","outcome":{"type":"interrupt"}}"#;
        assert!(parse_event_data::<Value>(malformed).unwrap().is_err());
    }

    #[test]
    fn unknown_event_is_dropped_but_malformed_known_event_fails() {
        assert!(parse_event_data::<Value>(r#"{"type":"FUTURE_EVENT","value":1}"#).is_none());
        assert!(
            parse_event_data::<Value>(r#"{"type":"TEXT_MESSAGE_CONTENT","messageId":"m"}"#)
                .unwrap()
                .is_err()
        );
    }
}
