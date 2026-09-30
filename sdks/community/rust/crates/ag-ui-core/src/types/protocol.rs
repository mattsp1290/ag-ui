use super::{MessageId, ToolCallId};
use serde::{Deserialize, Serialize};
use serde_json::{Map, Value};

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(tag = "op", rename_all = "lowercase")]
pub enum JsonPatchOperation {
    Add {
        path: JsonPointer,
        value: Value,
    },
    Remove {
        path: JsonPointer,
    },
    Replace {
        path: JsonPointer,
        value: Value,
    },
    Move {
        from: JsonPointer,
        path: JsonPointer,
    },
    Copy {
        from: JsonPointer,
        path: JsonPointer,
    },
    Test {
        path: JsonPointer,
        value: Value,
    },
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
#[serde(transparent)]
pub struct JsonPointer(pub String);
impl<'de> Deserialize<'de> for JsonPointer {
    fn deserialize<D: serde::Deserializer<'de>>(deserializer: D) -> Result<Self, D::Error> {
        let value = String::deserialize(deserializer)?;
        if !value.is_empty() && !value.starts_with('/') {
            return Err(serde::de::Error::custom("JSON Pointer must start with /"));
        }
        let bytes = value.as_bytes();
        let mut i = 0;
        while i < bytes.len() {
            if bytes[i] == b'~' {
                if bytes.get(i + 1) != Some(&b'0') && bytes.get(i + 1) != Some(&b'1') {
                    return Err(serde::de::Error::custom("bad JSON Pointer escape"));
                }
                i += 1;
            }
            i += 1;
        }
        Ok(Self(value))
    }
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "lowercase", deny_unknown_fields)]
pub enum PartSource {
    Data {
        value: String,
        #[serde(rename = "mimeType")]
        mime_type: String,
    },
    Url {
        value: String,
        #[serde(rename = "mimeType", skip_serializing_if = "Option::is_none")]
        mime_type: Option<String>,
    },
    File {
        value: String,
        #[serde(skip_serializing_if = "Option::is_none")]
        provider: Option<String>,
        #[serde(rename = "mimeType", skip_serializing_if = "Option::is_none")]
        mime_type: Option<String>,
    },
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "lowercase", deny_unknown_fields)]
pub enum ContentPart {
    Text {
        text: String,
        #[serde(skip_serializing_if = "Option::is_none")]
        id: Option<String>,
        #[serde(skip_serializing_if = "Option::is_none")]
        metadata: Option<Map<String, Value>>,
    },
    Image {
        source: PartSource,
        #[serde(skip_serializing_if = "Option::is_none")]
        id: Option<String>,
        #[serde(skip_serializing_if = "Option::is_none")]
        metadata: Option<Value>,
    },
    Audio {
        source: PartSource,
        #[serde(skip_serializing_if = "Option::is_none")]
        id: Option<String>,
        #[serde(skip_serializing_if = "Option::is_none")]
        metadata: Option<Value>,
    },
    Video {
        source: PartSource,
        #[serde(skip_serializing_if = "Option::is_none")]
        id: Option<String>,
        #[serde(skip_serializing_if = "Option::is_none")]
        metadata: Option<Value>,
    },
    Document {
        source: PartSource,
        #[serde(skip_serializing_if = "Option::is_none")]
        id: Option<String>,
        #[serde(skip_serializing_if = "Option::is_none")]
        metadata: Option<Value>,
    },
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(untagged)]
pub enum MessageContent {
    Text(String),
    Parts(Vec<ContentPart>),
}
impl From<String> for MessageContent {
    fn from(value: String) -> Self {
        Self::Text(value)
    }
}
impl MessageContent {
    pub fn as_text(&self) -> Option<&str> {
        if let Self::Text(s) = self {
            Some(s)
        } else {
            None
        }
    }
}

#[derive(Debug, Clone, PartialEq, Serialize)]
#[serde(tag = "type", rename_all = "lowercase", deny_unknown_fields)]
pub enum RunFinishedOutcome {
    Success {
        #[serde(rename = "pendingToolCallIds", skip_serializing_if = "Option::is_none")]
        pending_tool_call_ids: Option<Vec<ToolCallId>>,
    },
    Interrupt {
        interrupts: Vec<Interrupt>,
    },
    Cancelled,
}
#[derive(Debug, Clone, PartialEq, Serialize)]
#[serde(tag = "type", rename_all = "lowercase", deny_unknown_fields)]
pub enum SubagentFinishedOutcome {
    Success,
    Suspended {
        #[serde(rename = "interruptIds", skip_serializing_if = "Option::is_none")]
        interrupt_ids: Option<Vec<String>>,
    },
}
impl<'de> Deserialize<'de> for RunFinishedOutcome {
    fn deserialize<D: serde::Deserializer<'de>>(d: D) -> Result<Self, D::Error> {
        let v = Value::deserialize(d)?;
        let obj = v
            .as_object()
            .ok_or_else(|| serde::de::Error::custom("outcome must be an object"))?;
        let kind = obj
            .get("type")
            .and_then(Value::as_str)
            .ok_or_else(|| serde::de::Error::custom("outcome type missing"))?;
        match kind {
            "success" => {
                if obj.keys().any(|k| k != "type" && k != "pendingToolCallIds") {
                    return Err(serde::de::Error::custom("unknown success outcome property"));
                }
                let ids = obj
                    .get("pendingToolCallIds")
                    .map(|v| serde_json::from_value(v.clone()).map_err(serde::de::Error::custom))
                    .transpose()?;
                Ok(Self::Success {
                    pending_tool_call_ids: ids,
                })
            }
            "interrupt" => {
                if obj.keys().any(|k| k != "type" && k != "interrupts") {
                    return Err(serde::de::Error::custom(
                        "unknown interrupt outcome property",
                    ));
                }
                let interrupts: Vec<Interrupt> = serde_json::from_value(
                    obj.get("interrupts")
                        .cloned()
                        .ok_or_else(|| serde::de::Error::custom("interrupts missing"))?,
                )
                .map_err(serde::de::Error::custom)?;
                if interrupts.is_empty() {
                    return Err(serde::de::Error::custom(
                        "interrupt outcome requires interrupts",
                    ));
                }
                Ok(Self::Interrupt { interrupts })
            }
            "cancelled" => {
                if obj.len() != 1 {
                    return Err(serde::de::Error::custom(
                        "unknown cancelled outcome property",
                    ));
                }
                Ok(Self::Cancelled)
            }
            _ => Err(serde::de::Error::custom("unknown outcome type")),
        }
    }
}
impl<'de> Deserialize<'de> for SubagentFinishedOutcome {
    fn deserialize<D: serde::Deserializer<'de>>(d: D) -> Result<Self, D::Error> {
        let v = Value::deserialize(d)?;
        let obj = v
            .as_object()
            .ok_or_else(|| serde::de::Error::custom("outcome must be an object"))?;
        let kind = obj
            .get("type")
            .and_then(Value::as_str)
            .ok_or_else(|| serde::de::Error::custom("outcome type missing"))?;
        match kind {
            "success" if obj.len() == 1 => Ok(Self::Success),
            "suspended" if obj.keys().all(|k| k == "type" || k == "interruptIds") => {
                let ids = obj
                    .get("interruptIds")
                    .map(|v| serde_json::from_value(v.clone()).map_err(serde::de::Error::custom))
                    .transpose()?;
                Ok(Self::Suspended { interrupt_ids: ids })
            }
            _ => Err(serde::de::Error::custom("invalid subagent outcome")),
        }
    }
}
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Interrupt {
    pub id: String,
    pub reason: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub message: Option<String>,
    #[serde(rename = "toolCallId", skip_serializing_if = "Option::is_none")]
    pub tool_call_id: Option<ToolCallId>,
    #[serde(rename = "responseSchema", skip_serializing_if = "Option::is_none")]
    pub response_schema: Option<Map<String, Value>>,
    #[serde(rename = "expiresAt", skip_serializing_if = "Option::is_none")]
    pub expires_at: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub metadata: Option<Map<String, Value>>,
    #[serde(rename = "subagentRunId", skip_serializing_if = "Option::is_none")]
    pub subagent_run_id: Option<String>,
}
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct ResumeEntry {
    #[serde(rename = "interruptId")]
    pub interrupt_id: String,
    pub status: ResumeStatus,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub payload: Option<Value>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub metadata: Option<Map<String, Value>>,
}
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum ResumeStatus {
    Resolved,
    Cancelled,
}
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "kebab-case")]
pub enum ReasoningEncryptedValueSubtype {
    Message,
    ToolCall,
}
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct TokenUsage {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub provider: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub model: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub input_tokens: Option<u64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub output_tokens: Option<u64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub total_tokens: Option<u64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub reasoning_tokens: Option<u64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cached_input_tokens: Option<u64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cache_write_input_tokens: Option<u64>,
}
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct SubagentInfo {
    pub name: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,
}
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct AgentCapabilities {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub identity: Option<IdentityCapabilities>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub transport: Option<TransportCapabilities>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub tools: Option<ToolsCapabilities>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub output: Option<OutputCapabilities>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub state: Option<StateCapabilities>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub multi_agent: Option<MultiAgentCapabilities>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub reasoning: Option<ReasoningCapabilities>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub multimodal: Option<MultimodalCapabilities>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub execution: Option<ExecutionCapabilities>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub human_in_the_loop: Option<HumanInTheLoopCapabilities>,
    #[serde(
        skip_serializing_if = "Option::is_none",
        deserialize_with = "non_null_option",
        default
    )]
    pub custom: Option<Map<String, Value>>,
}
macro_rules! capability {
    ($name:ident {$($field:ident : $ty:ty),* $(,)?}) => {
        #[derive(Debug, Clone, PartialEq, Serialize, Deserialize, Default)]
        #[serde(rename_all="camelCase", deny_unknown_fields)]
        pub struct $name { $(#[serde(skip_serializing_if="Option::is_none", deserialize_with="non_null_option", default)] pub $field: Option<$ty>,)* }
    }
}
capability!(IdentityCapabilities {name:String, r#type:String, description:String, version:String, provider:String, documentation_url:String, metadata:Map<String, Value>});
capability!(TransportCapabilities {
    streaming: bool,
    websocket: bool,
    http_binary: bool,
    push_notifications: bool,
    resumable: bool
});
capability!(ToolsCapabilities {supported:bool, items:Vec<super::Tool>, parallel_calls:bool, client_provided:bool});
capability!(OutputCapabilities {structured_output:bool, supported_mime_types:Vec<String>});
capability!(StateCapabilities {
    snapshots: bool,
    deltas: bool,
    memory: bool,
    persistent_state: bool
});
capability!(MultiAgentCapabilities {supported:bool, delegation:bool, handoffs:bool, subagents:Vec<SubagentInfo>});
capability!(ReasoningCapabilities {
    supported: bool,
    streaming: bool,
    encrypted: bool
});
capability!(MultimodalInputCapabilities {
    image: bool,
    audio: bool,
    video: bool,
    pdf: bool,
    file: bool
});
capability!(MultimodalOutputCapabilities {
    image: bool,
    audio: bool
});
capability!(MultimodalCapabilities {
    input: MultimodalInputCapabilities,
    output: MultimodalOutputCapabilities
});
capability!(HumanInTheLoopCapabilities {
    supported: bool,
    approvals: bool,
    interventions: bool,
    feedback: bool,
    interrupts: bool,
    approve_with_edits: bool
});
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ExecutionCapabilities {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub code_execution: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub sandboxed: Option<bool>,
    #[serde(
        skip_serializing_if = "Option::is_none",
        deserialize_with = "safe_unsigned",
        default
    )]
    pub max_iterations: Option<u64>,
    #[serde(
        skip_serializing_if = "Option::is_none",
        deserialize_with = "safe_unsigned",
        default
    )]
    pub max_execution_time: Option<u64>,
}
fn safe_unsigned<'de, D: serde::Deserializer<'de>>(d: D) -> Result<Option<u64>, D::Error> {
    let n = u64::deserialize(d)?;
    if n > 9007199254740991 {
        return Err(serde::de::Error::custom("outside JSON safe integer range"));
    }
    Ok(Some(n))
}
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct ActivityMessage {
    pub id: MessageId,
    #[serde(rename = "activityType")]
    pub activity_type: String,
    pub content: Map<String, Value>,
}
pub fn non_null_option<'de, D, T>(deserializer: D) -> Result<Option<T>, D::Error>
where
    D: serde::Deserializer<'de>,
    T: Deserialize<'de>,
{
    T::deserialize(deserializer).map(Some)
}
