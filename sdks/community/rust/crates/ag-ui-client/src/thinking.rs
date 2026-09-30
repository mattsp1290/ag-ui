//! Compatibility for the five pre-1.0 THINKING event shapes.
use crate::core::types::MessageId;
use serde_json::{Value, json};

#[derive(Default)]
pub(crate) struct ThinkingTranslator {
    span_id: Option<MessageId>,
    message_id: Option<MessageId>,
    #[cfg(test)]
    pub(crate) warnings: Vec<String>,
}

impl ThinkingTranslator {
    fn warn(&mut self, message: String) {
        log::warn!("{message}");
        #[cfg(test)]
        self.warnings.push(message);
    }

    pub(crate) fn translate(&mut self, raw: &mut Value) {
        let Some(kind) = raw.get("type").and_then(Value::as_str) else {
            return;
        };
        let kind = kind.to_owned();
        let replacement = match kind.as_str() {
            "THINKING_START" => {
                self.span_id = Some(MessageId::random());
                "REASONING_START"
            }
            "THINKING_TEXT_MESSAGE_START" => {
                self.message_id = Some(MessageId::random());
                "REASONING_MESSAGE_START"
            }
            "THINKING_TEXT_MESSAGE_CONTENT" => "REASONING_MESSAGE_CONTENT",
            "THINKING_TEXT_MESSAGE_END" => "REASONING_MESSAGE_END",
            "THINKING_END" => "REASONING_END",
            _ => return,
        };
        self.warn(format!("Converting deprecated {kind} to {replacement}"));
        let Some(fields) = raw.as_object_mut() else {
            return;
        };
        fields.insert("type".into(), json!(replacement));
        if kind == "THINKING_START" && fields.remove("title").is_some() {
            self.warn("Dropping THINKING_START.title during reasoning translation".into());
        }
        let established = match kind.as_str() {
            "THINKING_START" | "THINKING_END" => self.span_id.clone(),
            _ => self.message_id.clone(),
        };
        let id = if let Some(id) = established {
            id
        } else {
            // Verification must still reject a continuation with no opener.
            let id = MessageId::random();
            self.warn(format!(
                "Minting messageId '{id}' for {kind}: no THINKING opener preceded it"
            ));
            id
        };
        fields.insert("messageId".into(), json!(id.as_ref()));
        if kind == "THINKING_TEXT_MESSAGE_START" {
            fields.insert("role".into(), json!("reasoning"));
        }
    }
}
