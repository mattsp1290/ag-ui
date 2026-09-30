use crate::{
    JsonValue,
    event::Event,
    types::{Message, MessageContent},
};

/// Materialized conversation and state after consuming AG-UI events.
#[derive(Debug, Clone, PartialEq)]
pub struct Replay {
    pub messages: Vec<Message>,
    pub state: JsonValue,
}
impl Default for Replay {
    fn default() -> Self {
        Self {
            messages: Vec::new(),
            state: JsonValue::Object(Default::default()),
        }
    }
}
impl Replay {
    pub fn apply(&mut self, event: &Event) -> Result<(), String> {
        match event {
            Event::TextMessageStart(e) => {
                self.messages.push(Message::Assistant {
                    id: e.message_id.clone(),
                    content: Some(String::new()),
                    name: None,
                    tool_calls: None,
                });
            }
            Event::TextMessageContent(e) => {
                let message = self
                    .messages
                    .iter_mut()
                    .rev()
                    .find(|m| m.id() == &e.message_id)
                    .ok_or_else(|| format!("content for unknown message {}", e.message_id))?;
                message
                    .content_mut()
                    .ok_or_else(|| "message has no text content".to_owned())?
                    .push_str(&e.delta);
            }
            Event::TextMessageChunk(e) => {
                if let Some(id) = &e.message_id {
                    if !self.messages.iter().any(|m| m.id() == id) {
                        self.messages.push(Message::Assistant {
                            id: id.clone(),
                            content: Some(String::new()),
                            name: None,
                            tool_calls: None,
                        });
                    }
                    if let Some(delta) = &e.delta {
                        let message = self
                            .messages
                            .iter_mut()
                            .rev()
                            .find(|m| m.id() == id)
                            .unwrap();
                        message.content_mut().unwrap().push_str(delta);
                    }
                }
            }
            Event::ToolCallResult(e) => {
                self.messages.push(Message::Tool {
                    id: e.message_id.clone(),
                    content: e.content.clone(),
                    tool_call_id: e.tool_call_id.clone(),
                    error: None,
                });
            }
            Event::MessagesSnapshot(e) => self.messages = e.messages.clone(),
            Event::StateSnapshot(e) => self.state = e.snapshot.clone(),
            Event::StateDelta(e) => {
                let patch: json_patch::Patch = serde_json::from_value(
                    serde_json::to_value(&e.delta).map_err(|e| e.to_string())?,
                )
                .map_err(|e| format!("invalid patch: {e}"))?;
                json_patch::patch(&mut self.state, &patch)
                    .map_err(|e| format!("patch failed: {e}"))?;
            }
            _ => {}
        }
        Ok(())
    }
    pub fn replay<'a>(
        &mut self,
        events: impl IntoIterator<Item = &'a Event>,
    ) -> Result<(), String> {
        for event in events {
            self.apply(event)?;
        }
        Ok(())
    }
    pub fn message_texts(&self) -> Vec<&str> {
        self.messages
            .iter()
            .filter_map(|m| match m {
                Message::User {
                    content: MessageContent::Text(s),
                    ..
                } => Some(s.as_str()),
                _ => m.content(),
            })
            .collect()
    }
}
