use crate::{
    JsonValue,
    event::{BaseEvent, Event},
    types::{Message, MessageContent, MessageId, Role},
};

/// A limited convenience reducer, not a lifecycle verifier or canonical server
/// history/persistence engine. Use the client's explicit lifecycle API for full
/// protocol application and snapshot reconciliation.
///
/// Supported: text start/content/end, explicitly identified text chunks, tool
/// results, complete message replacement, state snapshots (including null), and
/// atomic state patches. Run/step boundaries are observed as no-ops. Activity,
/// reasoning, tool-call assembly, subagents and custom/raw events are explicitly
/// unsupported. `message_texts` is a lossy display convenience, not model history.
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
fn text_message(
    id: MessageId,
    role: Role,
    name: Option<String>,
    base: &BaseEvent,
) -> Result<Message, String> {
    if !matches!(
        role,
        Role::Assistant | Role::User | Role::System | Role::Developer
    ) {
        return Err("invalid streamed text role".into());
    }
    let mut message = Message::new(role, id, "");
    match &mut message {
        Message::Assistant {
            name: n,
            metadata,
            subagent_run_id,
            ..
        }
        | Message::User {
            name: n,
            metadata,
            subagent_run_id,
            ..
        }
        | Message::System {
            name: n,
            metadata,
            subagent_run_id,
            ..
        }
        | Message::Developer {
            name: n,
            metadata,
            subagent_run_id,
            ..
        } => {
            *n = name;
            *metadata = base.metadata.clone();
            *subagent_run_id = base.subagent_run_id.clone();
        }
        _ => unreachable!(),
    }
    Ok(message)
}
fn require_text_role(message: &Message) -> Result<(), String> {
    if matches!(
        message.role(),
        Role::Assistant | Role::User | Role::System | Role::Developer
    ) {
        Ok(())
    } else {
        Err("target is not a streamed text role".into())
    }
}
fn merge_metadata(message: &mut Message, base: &BaseEvent) {
    let Some(incoming) = &base.metadata else {
        return;
    };
    let (Message::Assistant { metadata, .. }
    | Message::User { metadata, .. }
    | Message::System { metadata, .. }
    | Message::Developer { metadata, .. }
    | Message::Tool { metadata, .. }
    | Message::Activity { metadata, .. }
    | Message::Reasoning { metadata, .. }) = message;
    metadata
        .get_or_insert_with(Default::default)
        .extend(incoming.clone());
}
impl Replay {
    pub fn apply(&mut self, event: &Event) -> Result<(), String> {
        match event {
            Event::TextMessageStart(e) => {
                if self.messages.iter().any(|m| m.id() == &e.message_id) {
                    return Err("duplicate text message id in limited replay".into());
                }
                self.messages.push(text_message(
                    e.message_id.clone(),
                    e.role.clone().unwrap_or(Role::Assistant),
                    e.name.clone(),
                    &e.base,
                )?);
            }
            Event::TextMessageContent(e) => {
                let message = self
                    .messages
                    .iter_mut()
                    .find(|m| m.id() == &e.message_id)
                    .ok_or_else(|| "content for unknown message".to_string())?;
                require_text_role(message)?;
                message
                    .content_mut()
                    .ok_or_else(|| "message has no text content".to_string())?
                    .push_str(&e.delta);
                merge_metadata(message, &e.base);
            }
            Event::TextMessageEnd(e) => {
                let message = self
                    .messages
                    .iter_mut()
                    .find(|m| m.id() == &e.message_id)
                    .ok_or_else(|| "end for unknown message".to_string())?;
                require_text_role(message)?;
                merge_metadata(message, &e.base);
            }
            Event::TextMessageChunk(e) => {
                let id = e.message_id.as_ref().ok_or_else(|| {
                    "limited replay requires explicit chunk messageId".to_string()
                })?;
                let index = self.messages.iter().position(|m| m.id() == id);
                // Prepare a replacement independently so even invalid chunks do
                // not leave a partly mutated message collection.
                let mut message = if let Some(index) = index {
                    self.messages[index].clone()
                } else {
                    text_message(
                        id.clone(),
                        e.role.clone().unwrap_or(Role::Assistant),
                        e.name.clone(),
                        &e.base,
                    )?
                };
                require_text_role(&message)?;
                if let Some(role) = e.role.clone()
                    && message.role() != role
                {
                    return Err("chunk role differs from existing text message".into());
                }
                if let Some(delta) = &e.delta {
                    message
                        .content_mut()
                        .ok_or_else(|| "message has no text content".to_string())?
                        .push_str(delta);
                }
                merge_metadata(&mut message, &e.base);
                if let Some(name) = &e.name {
                    match &mut message {
                        Message::Assistant { name: n, .. }
                        | Message::User { name: n, .. }
                        | Message::System { name: n, .. }
                        | Message::Developer { name: n, .. } => *n = Some(name.clone()),
                        _ => return Err("chunk target is not a streamed text role".into()),
                    }
                }
                if let Some(index) = index {
                    self.messages[index] = message;
                } else {
                    self.messages.push(message);
                }
            }
            Event::ToolCallResult(e) => {
                if self.messages.iter().any(|m| m.id() == &e.message_id) {
                    return Err("duplicate tool result message id in limited replay".into());
                }
                self.messages.push(Message::Tool {
                    id: e.message_id.clone(),
                    content: e.content.clone(),
                    tool_call_id: e.tool_call_id.clone(),
                    error: None,
                    metadata: e.base.metadata.clone(),
                    encrypted_value: None,
                    subagent_run_id: e.base.subagent_run_id.clone(),
                });
            }
            // Deliberately full replacement in this convenience reducer. This
            // does not provide the full client's role-aware reconciliation.
            Event::MessagesSnapshot(e) => self.messages = e.messages.clone(),
            Event::StateSnapshot(e) => self.state = e.snapshot.clone(),
            Event::StateDelta(e) => {
                let patch: json_patch::Patch = serde_json::from_value(
                    serde_json::to_value(&e.delta)
                        .map_err(|_| "invalid patch serialization".to_string())?,
                )
                .map_err(|_| "invalid patch".to_string())?;
                let mut next = self.state.clone();
                json_patch::patch(&mut next, &patch).map_err(|_| "patch failed".to_string())?;
                self.state = next;
            }
            Event::RunStarted(_)
            | Event::RunFinished(_)
            | Event::RunError(_)
            | Event::StepStarted(_)
            | Event::StepFinished(_) => {}
            _ => {
                return Err(format!(
                    "event {:?} is unsupported by limited Replay",
                    event.event_type()
                ));
            }
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
