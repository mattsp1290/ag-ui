use crate::types::ids::ToolCallId;
use crate::types::message::FunctionCall;
use serde::{Deserialize, Serialize};
use serde_json::Value as JsonValue;

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct ToolCall {
    pub id: ToolCallId,
    #[serde(rename = "type")]
    pub call_type: String,
    pub function: FunctionCall,
}

impl ToolCall {
    pub fn new(id: impl Into<ToolCallId>, function: FunctionCall) -> Self {
        Self {
            id: id.into(),
            call_type: "function".to_string(),
            function,
        }
    }
}

/// A tool definition.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Tool {
    /// The tool name
    pub name: String,
    /// The tool description
    pub description: String,
    /// The tool parameters
    #[serde(skip_serializing_if = "Option::is_none")]
    pub parameters: Option<serde_json::Value>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub metadata: Option<serde_json::Map<String, serde_json::Value>>,
}

impl Tool {
    pub fn new(name: String, description: String, parameters: JsonValue) -> Self {
        Self {
            name,
            description,
            parameters: Some(parameters),
            metadata: None,
        }
    }
}
