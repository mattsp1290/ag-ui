use crate::agent::{AgentError, AgentStateMutation};
use crate::core::event::Event;
use crate::core::types::{
    FunctionCall, Message, MessageId, ReasoningEncryptedValueSubtype, Role, RunAgentInput,
    RunFinishedOutcome, TokenUsage, ToolCall,
};
use crate::core::{AgentState, FwdProps, JsonValue};
use crate::subscriber::{AgentSubscriberParams, Subscribers};
use json_patch::PatchOperation;
use log::error;
use std::collections::{HashMap, HashSet};

/// Captures the run state and handles events
#[derive(Clone)]
pub(crate) struct EventHandler<'a, StateT, FwdPropsT>
where
    StateT: AgentState,
    FwdPropsT: FwdProps,
{
    pub messages: Vec<Message>,
    pub state: StateT,
    pub input: &'a RunAgentInput<StateT, FwdPropsT>,
    pub subscribers: Subscribers<StateT, FwdPropsT>,
    pub result: JsonValue,
    pub outcome: Option<RunFinishedOutcome>,
    pub usage: Option<Vec<TokenUsage>>,
    active_run: bool,
    finished_run: bool,
    active_text: Option<MessageId>,
    active_reasoning: Option<MessageId>,
    active_span: Option<MessageId>,
    active_steps: HashSet<String>,
    active_subagents: HashSet<String>,
    text_chunk: Option<(MessageId, Role)>,
    tool_chunk: Option<(crate::core::types::ToolCallId, String)>,
}

impl<'a, StateT, FwdPropsT> EventHandler<'a, StateT, FwdPropsT>
where
    StateT: AgentState,
    FwdPropsT: FwdProps,
{
    pub fn new(
        messages: Vec<Message>,
        state: StateT,
        input: &'a RunAgentInput<StateT, FwdPropsT>,
        subscribers: Subscribers<StateT, FwdPropsT>,
    ) -> Self {
        Self {
            messages,
            state,
            input,
            subscribers,
            result: JsonValue::Null,
            outcome: None,
            usage: None,
            active_run: false,
            finished_run: false,
            active_text: None,
            active_reasoning: None,
            active_span: None,
            active_steps: HashSet::new(),
            active_subagents: HashSet::new(),
            text_chunk: None,
            tool_chunk: None,
        }
    }

    fn to_subscriber_params(&'a self) -> AgentSubscriberParams<'a, StateT, FwdPropsT> {
        AgentSubscriberParams {
            messages: &self.messages,
            state: &self.state,
            input: self.input,
        }
    }

    // Helper method to directly update state and messages without using apply_mutation
    fn update_from_mutation(&mut self, mutation: &AgentStateMutation<StateT>) {
        if let Some(messages) = &mutation.messages {
            self.messages = messages.clone();
        }
        if let Some(state) = &mutation.state {
            self.state = state.clone();
        }
    }

    // Helper method to process a subscriber's mutation
    fn process_mutation(
        &mut self,
        mutation: AgentStateMutation<StateT>,
        current_mutation: &mut AgentStateMutation<StateT>,
    ) {
        // Apply any mutations
        if mutation.messages.is_some() || mutation.state.is_some() {
            // Update directly without using apply_mutation
            self.update_from_mutation(&mutation);

            // Update current_mutation with the applied changes
            if mutation.messages.is_some() {
                current_mutation.messages = mutation.messages;
            }
            if mutation.state.is_some() {
                current_mutation.state = mutation.state;
            }
        }
    }

    fn verify_event(&mut self, event: &Event<StateT>) -> Result<(), AgentError> {
        // RAW does not interrupt a chunk lane. Every other non-chunk event does.
        if !matches!(
            event,
            Event::Raw(_) | Event::TextMessageChunk(_) | Event::ToolCallChunk(_)
        ) {
            self.text_chunk = None;
            self.tool_chunk = None;
        }
        let fail = |message: String| AgentError::Execution { message };
        if self.finished_run {
            return Err(fail("Event after RUN_FINISHED".into()));
        }
        match event {
            Event::RunStarted(_) if self.active_run => {
                return Err(fail("Nested RUN_STARTED".into()));
            }
            Event::RunStarted(_) => self.active_run = true,
            Event::RunFinished(_) => {
                if !self.active_subagents.is_empty() {
                    return Err(fail("RUN_FINISHED with active subagent".into()));
                }
                self.finished_run = true;
            }
            Event::TextMessageStart(e) => self.active_text = Some(e.message_id.clone()),
            Event::TextMessageContent(e) => {
                if self.active_text.as_ref() != Some(&e.message_id) {
                    return Err(fail(format!(
                        "No active text message found with ID '{}'",
                        e.message_id
                    )));
                }
            }
            Event::TextMessageEnd(e) => {
                if self.active_text.as_ref() != Some(&e.message_id) {
                    return Err(fail(format!(
                        "No active text message found with ID '{}'",
                        e.message_id
                    )));
                }
                self.active_text = None;
            }
            Event::ReasoningStart(e) => self.active_span = Some(e.message_id.clone()),
            Event::ReasoningEnd(e) => {
                if self.active_span.as_ref() != Some(&e.message_id) {
                    return Err(fail("No active reasoning span".into()));
                }
                self.active_span = None;
            }
            Event::ReasoningMessageStart(e) => {
                if e.role != Role::Reasoning {
                    return Err(fail("Reasoning message role must be reasoning".into()));
                }
                self.active_reasoning = Some(e.message_id.clone());
            }
            Event::ReasoningMessageContent(e) => {
                if self.active_reasoning.as_ref() != Some(&e.message_id) {
                    return Err(fail(format!(
                        "No active reasoning message found with ID '{}'",
                        e.message_id
                    )));
                }
            }
            Event::ReasoningMessageEnd(e) => {
                if self.active_reasoning.as_ref() != Some(&e.message_id) {
                    return Err(fail(format!(
                        "No active reasoning message found with ID '{}'",
                        e.message_id
                    )));
                }
                self.active_reasoning = None;
            }
            Event::StepStarted(e) => {
                self.active_steps.insert(e.step_name.clone());
            }
            Event::StepFinished(e) => {
                if !self.active_steps.remove(&e.step_name) {
                    return Err(fail(format!(
                        "STEP_FINISHED without STEP_STARTED: {}",
                        e.step_name
                    )));
                }
            }
            Event::SubagentStarted(e) => {
                self.active_subagents.insert(e.subagent_run_id.clone());
            }
            Event::SubagentFinished(e) => {
                if !self.active_subagents.remove(&e.subagent_run_id) {
                    return Err(fail(format!("Subagent not started: {}", e.subagent_run_id)));
                }
            }
            Event::SubagentError(e) => {
                if !self.active_subagents.remove(&e.subagent_run_id) {
                    return Err(fail(format!("Subagent not started: {}", e.subagent_run_id)));
                }
            }
            Event::TextMessageChunk(e) => {
                if let Some((id, role)) = &self.text_chunk {
                    if e.message_id.as_ref().is_some_and(|v| v != id) {
                        return Err(fail("Conflicting TEXT_MESSAGE_CHUNK messageId".into()));
                    }
                    if e.role.as_ref().is_some_and(|v| v != role) {
                        return Err(fail("Conflicting TEXT_MESSAGE_CHUNK role".into()));
                    }
                } else {
                    let id = e.message_id.clone().ok_or_else(|| {
                        fail("First TEXT_MESSAGE_CHUNK must have a messageId".into())
                    })?;
                    self.text_chunk = Some((id, e.role.clone().unwrap_or(Role::Assistant)));
                }
            }
            Event::ToolCallChunk(e) => {
                if let Some((id, name)) = &self.tool_chunk {
                    if e.tool_call_id.as_ref().is_some_and(|v| v != id)
                        || e.tool_call_name.as_ref().is_some_and(|v| v != name)
                    {
                        return Err(fail("Conflicting TOOL_CALL_CHUNK".into()));
                    }
                } else {
                    let id = e.tool_call_id.clone().ok_or_else(|| {
                        fail("First TOOL_CALL_CHUNK must have a toolCallId".into())
                    })?;
                    let name = e.tool_call_name.clone().ok_or_else(|| {
                        fail("First TOOL_CALL_CHUNK must have a toolCallName".into())
                    })?;
                    self.tool_chunk = Some((id, name));
                }
            }
            _ => {}
        }
        Ok(())
    }

    pub async fn handle_event(
        &mut self,
        event: &Event<StateT>,
    ) -> Result<AgentStateMutation<StateT>, AgentError> {
        let mut current_mutation = AgentStateMutation::default();
        let mut mutations = Vec::new();

        self.verify_event(event)?;
        // Clone subscribers to avoid borrowing issues
        for subscriber in &self.subscribers.clone() {
            let params = self.to_subscriber_params();
            let mutation = subscriber.on_event(event, params).await?;
            mutations.push(mutation);
        }

        // Then handle specific event types
        match event {
            Event::TextMessageStart(e) => {
                // Default behavior
                let new_message = Message::Assistant {
                    id: e.message_id.clone(),
                    content: Some(String::new()),
                    name: e.name.clone(),
                    tool_calls: None,
                    metadata: e.base.metadata.clone(),
                    encrypted_value: None,
                    subagent_run_id: e.base.subagent_run_id.clone(),
                };
                self.messages.push(new_message);
                current_mutation.messages = Some(self.messages.clone());

                for subscriber in &self.subscribers {
                    let params = self.to_subscriber_params();
                    let mutation = subscriber.on_text_message_start_event(e, params).await?;
                    mutations.push(mutation);
                }
            }
            Event::TextMessageContent(e) => {
                // Default behavior
                if let Some(last_message) = self
                    .messages
                    .iter_mut()
                    .rev()
                    .find(|m| m.id() == &e.message_id)
                {
                    merge_message_metadata(last_message, &e.base.metadata);
                    if let Some(s) = last_message.content_mut() {
                        s.push_str(&e.delta)
                    }
                    current_mutation.messages = Some(self.messages.clone());
                }

                // Get the current text message buffer
                let text_message_buffer = self
                    .messages
                    .iter()
                    .rev()
                    .find(|m| m.id() == &e.message_id)
                    .and_then(|m| m.content())
                    .unwrap_or_default()
                    .to_string(); // Clone to avoid borrowing issues

                for subscriber in &self.subscribers {
                    let params = self.to_subscriber_params();
                    let mutation = subscriber
                        .on_text_message_content_event(e, &text_message_buffer, params)
                        .await?;
                    mutations.push(mutation);
                }
            }
            Event::TextMessageEnd(e) => {
                if let Some(message) = self
                    .messages
                    .iter_mut()
                    .rev()
                    .find(|m| m.id() == &e.message_id)
                {
                    merge_message_metadata(message, &e.base.metadata);
                    current_mutation.messages = Some(self.messages.clone());
                }
                // Get the current text message buffer
                let text_message_buffer = self
                    .messages
                    .iter()
                    .rev()
                    .find(|m| m.id() == &e.message_id)
                    .and_then(|m| m.content())
                    .unwrap_or_default()
                    .to_string(); // Clone to avoid borrowing issues

                for subscriber in &self.subscribers {
                    let params = self.to_subscriber_params();
                    let mutation = subscriber
                        .on_text_message_end_event(e, &text_message_buffer, params)
                        .await?;
                    mutations.push(mutation);
                }
            }
            Event::TextMessageChunk(e) => {
                if let Some(id) = self.text_chunk.as_ref().map(|(id, _)| id) {
                    if let Some(Message::Assistant { name, content, .. }) =
                        self.messages.iter_mut().rev().find(|m| m.id() == id)
                    {
                        if let Some(author) = &e.name {
                            *name = Some(author.clone());
                        }
                        if let Some(delta) = &e.delta {
                            content.get_or_insert_with(String::new).push_str(delta);
                        }
                    } else {
                        self.messages.push(Message::Assistant {
                            id: id.clone(),
                            content: Some(e.delta.clone().unwrap_or_default()),
                            name: e.name.clone(),
                            tool_calls: None,
                            metadata: None,
                            encrypted_value: None,
                            subagent_run_id: e.base.subagent_run_id.clone(),
                        });
                    }
                    current_mutation.messages = Some(self.messages.clone());
                }
                for subscriber in &self.subscribers {
                    let params = self.to_subscriber_params();
                    let mutation = subscriber.on_text_message_chunk_event(e, params).await?;
                    mutations.push(mutation);
                }
            }
            Event::ToolCallStart(e) => {
                // Default behavior
                let new_tool_call = ToolCall {
                    id: e.tool_call_id.clone(),
                    call_type: "function".to_string(),
                    function: FunctionCall {
                        name: e.tool_call_name.clone(),
                        arguments: String::new(),
                    },
                    metadata: e.base.metadata.clone(),
                    encrypted_value: None,
                };

                if let Some(last_message) = self
                    .messages
                    .iter_mut()
                    .rev()
                    .find(|m| Some(m.id()) == e.parent_message_id.as_ref())
                {
                    if let Message::Assistant { tool_calls, .. } = last_message {
                        tool_calls.get_or_insert_with(Vec::new).push(new_tool_call);
                    }
                } else {
                    let new_message = Message::Assistant {
                        id: e
                            .parent_message_id
                            .clone()
                            .unwrap_or_else(MessageId::random),
                        content: None,
                        name: None,
                        tool_calls: None,
                        metadata: None,
                        encrypted_value: None,
                        subagent_run_id: e.base.subagent_run_id.clone(),
                    };
                    self.messages.push(new_message);
                    if let Some(Message::Assistant { tool_calls, .. }) = self.messages.last_mut() {
                        tool_calls.get_or_insert_with(Vec::new).push(new_tool_call);
                    }
                }
                current_mutation.messages = Some(self.messages.clone());

                for subscriber in &self.subscribers {
                    let params = self.to_subscriber_params();
                    let mutation = subscriber.on_tool_call_start_event(e, params).await?;
                    mutations.push(mutation);
                }
            }
            Event::ToolCallArgs(e) => {
                // Default behavior
                if let Some(last_message) = self.messages.last_mut()
                    && let Some(tool_calls) = last_message.tool_calls_mut()
                    && let Some(last_tool_call) = tool_calls.last_mut()
                {
                    last_tool_call.function.arguments.push_str(&e.delta);
                    current_mutation.messages = Some(self.messages.clone());
                }

                // Get the current tool call buffer and name
                let (tool_call_buffer, tool_call_name, partial_args) = if let Some(last_message) =
                    self.messages.last()
                {
                    if let Some(tool_calls) = last_message.tool_calls() {
                        if let Some(last_tool_call) = tool_calls.last() {
                            // Try to parse the arguments as JSON to get partial args
                            let partial_args = serde_json::from_str::<HashMap<String, JsonValue>>(
                                &last_tool_call.function.arguments,
                            )
                            .unwrap_or_default();
                            (
                                last_tool_call.function.arguments.clone(),
                                last_tool_call.function.name.clone(),
                                partial_args,
                            )
                        } else {
                            (String::new(), String::new(), HashMap::new())
                        }
                    } else {
                        (String::new(), String::new(), HashMap::new())
                    }
                } else {
                    (String::new(), String::new(), HashMap::new())
                };

                for subscriber in &self.subscribers {
                    let params = self.to_subscriber_params();
                    let mutation = subscriber
                        .on_tool_call_args_event(
                            e,
                            &tool_call_buffer,
                            &tool_call_name,
                            &partial_args,
                            params,
                        )
                        .await?;
                    mutations.push(mutation);
                }
            }
            Event::ToolCallEnd(e) => {
                // Get the current tool call buffer and name
                let (tool_call_name, tool_call_args) =
                    if let Some(last_message) = self.messages.last() {
                        if let Some(tool_calls) = last_message.tool_calls() {
                            if let Some(last_tool_call) = tool_calls.last() {
                                // Try to parse the arguments as JSON
                                let args = serde_json::from_str::<HashMap<String, JsonValue>>(
                                    &last_tool_call.function.arguments,
                                )
                                .unwrap_or_default();
                                (last_tool_call.function.name.clone(), args)
                            } else {
                                (String::new(), HashMap::new())
                            }
                        } else {
                            (String::new(), HashMap::new())
                        }
                    } else {
                        (String::new(), HashMap::new())
                    };

                for subscriber in &self.subscribers {
                    let params = self.to_subscriber_params();
                    let mutation = subscriber
                        .on_tool_call_end_event(e, &tool_call_name, &tool_call_args, params)
                        .await?;
                    mutations.push(mutation);
                }
            }
            Event::ToolCallChunk(e) => {
                if let Some((id, name)) = &self.tool_chunk {
                    let parent = e.parent_message_id.clone();
                    let position = self
                        .messages
                        .iter()
                        .rposition(|m| {
                            Some(m.id()) == parent.as_ref() && m.role() == Role::Assistant
                        })
                        .or_else(|| {
                            self.messages
                                .iter()
                                .rposition(|m| m.role() == Role::Assistant)
                        });
                    if let Some(message) = position.and_then(|i| self.messages.get_mut(i)) {
                        let Message::Assistant { tool_calls, .. } = message else {
                            unreachable!()
                        };
                        let calls = tool_calls.get_or_insert_with(Vec::new);
                        if let Some(call) = calls.iter_mut().find(|c| &c.id == id) {
                            call.function
                                .arguments
                                .push_str(e.delta.as_deref().unwrap_or_default());
                        } else {
                            calls.push(ToolCall {
                                id: id.clone(),
                                call_type: "function".into(),
                                function: FunctionCall {
                                    name: name.clone(),
                                    arguments: e.delta.clone().unwrap_or_default(),
                                },
                                metadata: None,
                                encrypted_value: None,
                            });
                        }
                    } else {
                        self.messages.push(Message::Assistant {
                            id: parent.unwrap_or_else(MessageId::random),
                            content: None,
                            name: None,
                            tool_calls: Some(vec![ToolCall {
                                id: id.clone(),
                                call_type: "function".into(),
                                function: FunctionCall {
                                    name: name.clone(),
                                    arguments: e.delta.clone().unwrap_or_default(),
                                },
                                metadata: None,
                                encrypted_value: None,
                            }]),
                            metadata: None,
                            encrypted_value: None,
                            subagent_run_id: e.base.subagent_run_id.clone(),
                        });
                    }
                    current_mutation.messages = Some(self.messages.clone());
                }
                for subscriber in &self.subscribers {
                    let params = self.to_subscriber_params();
                    let mutation = subscriber.on_tool_call_chunk_event(e, params).await?;
                    mutations.push(mutation);
                }
            }
            Event::ToolCallResult(e) => {
                self.messages.push(Message::Tool {
                    id: e.message_id.clone(),
                    content: e.content.clone(),
                    tool_call_id: e.tool_call_id.clone(),
                    error: None,
                    metadata: e.base.metadata.clone(),
                    encrypted_value: None,
                    subagent_run_id: e.base.subagent_run_id.clone(),
                });
                current_mutation.messages = Some(self.messages.clone());
                for subscriber in &self.subscribers {
                    let params = self.to_subscriber_params();
                    let mutation = subscriber.on_tool_call_result_event(e, params).await?;
                    mutations.push(mutation);
                }
            }
            Event::StateSnapshot(e) => {
                // Default behavior
                self.state = e.snapshot.clone();
                current_mutation.state = Some(self.state.clone());

                for subscriber in &self.subscribers {
                    let params = self.to_subscriber_params();
                    let mutation = subscriber.on_state_snapshot_event(e, params).await?;
                    mutations.push(mutation);
                }
            }
            Event::StateDelta(e) => {
                // Default behavior
                let mut state_val = serde_json::to_value(&self.state)?;

                // TODO: This cast to and from JsonValue seems unnecessary
                let patches: Vec<PatchOperation> =
                    serde_json::from_value(serde_json::to_value(e.delta.clone())?)?;

                if let Err(err) = json_patch::patch(&mut state_val, &patches) {
                    log::warn!("Failed to apply state patch: {err}");
                } else {
                    let new_state: StateT = serde_json::from_value(state_val)?;
                    self.state = new_state;
                    current_mutation.state = Some(self.state.clone());
                }

                for subscriber in &self.subscribers {
                    let params = self.to_subscriber_params();
                    let mutation = subscriber.on_state_delta_event(e, params).await?;
                    mutations.push(mutation);
                }
            }
            Event::MessagesSnapshot(e) => {
                self.messages = e.messages.clone();
                current_mutation.messages = Some(self.messages.clone());
                for subscriber in &self.subscribers {
                    let params = self.to_subscriber_params();
                    let mutation = subscriber.on_messages_snapshot_event(e, params).await?;
                    mutations.push(mutation);
                }
            }
            Event::Raw(e) => {
                for subscriber in &self.subscribers {
                    let params = self.to_subscriber_params();
                    let mutation = subscriber.on_raw_event(e, params).await?;
                    mutations.push(mutation);
                }
            }
            Event::Custom(e) => {
                for subscriber in &self.subscribers {
                    let params = self.to_subscriber_params();
                    let mutation = subscriber.on_custom_event(e, params).await?;
                    mutations.push(mutation);
                }
            }
            Event::RunStarted(e) => {
                for subscriber in &self.subscribers {
                    let params = self.to_subscriber_params();
                    let mutation = subscriber.on_run_started_event(e, params).await?;
                    mutations.push(mutation);
                }
            }
            Event::RunFinished(e) => {
                // Default behavior
                self.result = e.result.clone().unwrap_or(JsonValue::Null);
                self.outcome = e.outcome.clone();
                self.usage = e.usage.clone();

                for subscriber in &self.subscribers {
                    let params = self.to_subscriber_params();
                    let mutation = subscriber.on_run_finished_event(e, params).await?;
                    mutations.push(mutation);
                }
            }
            Event::RunError(e) => {
                self.usage = e.usage.clone();
                for subscriber in &self.subscribers {
                    let params = self.to_subscriber_params();
                    let mutation = subscriber.on_run_error_event(e, params).await?;
                    mutations.push(mutation);
                }
            }
            Event::StepStarted(e) => {
                for subscriber in &self.subscribers {
                    let params = self.to_subscriber_params();
                    let mutation = subscriber.on_step_started_event(e, params).await?;
                    mutations.push(mutation);
                }
            }
            Event::StepFinished(e) => {
                for subscriber in &self.subscribers {
                    let params = self.to_subscriber_params();
                    let mutation = subscriber.on_step_finished_event(e, params).await?;
                    mutations.push(mutation);
                }
            }
            Event::ActivitySnapshot(e) => {
                if let Some(Message::Activity {
                    content,
                    activity_type,
                    ..
                }) = self.messages.iter_mut().find(|m| m.id() == &e.message_id)
                {
                    if e.replace.unwrap_or(true) {
                        *content = e.content.clone();
                        *activity_type = e.activity_type.clone();
                    }
                } else {
                    self.messages.push(Message::Activity {
                        id: e.message_id.clone(),
                        activity_type: e.activity_type.clone(),
                        content: e.content.clone(),
                        metadata: e.base.metadata.clone(),
                        subagent_run_id: e.base.subagent_run_id.clone(),
                    });
                }
                current_mutation.messages = Some(self.messages.clone());
                for subscriber in &self.subscribers {
                    let params = self.to_subscriber_params();
                    mutations.push(subscriber.on_activity_snapshot_event(e, params).await?);
                }
            }
            Event::ActivityDelta(e) => {
                if let Some(Message::Activity { content, .. }) =
                    self.messages.iter_mut().find(|m| m.id() == &e.message_id)
                {
                    let mut value = JsonValue::Object(content.clone());
                    let patches: Vec<PatchOperation> =
                        serde_json::from_value(serde_json::to_value(&e.patch)?)?;
                    if json_patch::patch(&mut value, &patches).is_ok()
                        && let JsonValue::Object(map) = value
                    {
                        *content = map;
                        current_mutation.messages = Some(self.messages.clone());
                    }
                }
                for subscriber in &self.subscribers {
                    let params = self.to_subscriber_params();
                    mutations.push(subscriber.on_activity_delta_event(e, params).await?);
                }
            }
            Event::ReasoningStart(e) => {
                for subscriber in &self.subscribers {
                    let params = self.to_subscriber_params();
                    mutations.push(subscriber.on_reasoning_start_event(e, params).await?);
                }
            }
            Event::ReasoningMessageStart(e) => {
                self.messages.push(Message::Reasoning {
                    id: e.message_id.clone(),
                    content: String::new(),
                    encrypted_value: None,
                    metadata: e.base.metadata.clone(),
                    subagent_run_id: e.base.subagent_run_id.clone(),
                });
                current_mutation.messages = Some(self.messages.clone());
                for subscriber in &self.subscribers {
                    let params = self.to_subscriber_params();
                    mutations.push(
                        subscriber
                            .on_reasoning_message_start_event(e, params)
                            .await?,
                    );
                }
            }
            Event::ReasoningMessageContent(e) => {
                if let Some(message) = self.messages.iter_mut().find(|m| m.id() == &e.message_id) {
                    merge_message_metadata(message, &e.base.metadata);
                    if let Message::Reasoning { content, .. } = message {
                        content.push_str(&e.delta);
                    }
                    current_mutation.messages = Some(self.messages.clone());
                }
                for subscriber in &self.subscribers {
                    let params = self.to_subscriber_params();
                    mutations.push(
                        subscriber
                            .on_reasoning_message_content_event(e, params)
                            .await?,
                    );
                }
            }
            Event::ReasoningMessageEnd(e) => {
                for subscriber in &self.subscribers {
                    let params = self.to_subscriber_params();
                    mutations.push(subscriber.on_reasoning_message_end_event(e, params).await?);
                }
            }
            Event::ReasoningMessageChunk(e) => {
                let id = e
                    .message_id
                    .clone()
                    .or_else(|| self.active_reasoning.clone());
                if let Some(id) = id {
                    if let Some(Message::Reasoning { content, .. }) =
                        self.messages.iter_mut().find(|m| m.id() == &id)
                    {
                        content.push_str(e.delta.as_deref().unwrap_or_default());
                    } else {
                        self.messages.push(Message::Reasoning {
                            id,
                            content: e.delta.clone().unwrap_or_default(),
                            encrypted_value: None,
                            metadata: e.base.metadata.clone(),
                            subagent_run_id: e.base.subagent_run_id.clone(),
                        });
                    }
                    current_mutation.messages = Some(self.messages.clone());
                }
                for subscriber in &self.subscribers {
                    let params = self.to_subscriber_params();
                    mutations.push(
                        subscriber
                            .on_reasoning_message_chunk_event(e, params)
                            .await?,
                    );
                }
            }
            Event::ReasoningEnd(e) => {
                for subscriber in &self.subscribers {
                    let params = self.to_subscriber_params();
                    mutations.push(subscriber.on_reasoning_end_event(e, params).await?);
                }
            }
            Event::ReasoningEncryptedValue(e) => {
                if e.subtype == ReasoningEncryptedValueSubtype::Message {
                    if let Some(message) = self
                        .messages
                        .iter_mut()
                        .find(|m| m.id().as_ref() == e.entity_id)
                    {
                        match message {
                            Message::Assistant {
                                encrypted_value, ..
                            }
                            | Message::Reasoning {
                                encrypted_value, ..
                            }
                            | Message::Tool {
                                encrypted_value, ..
                            } => *encrypted_value = Some(e.encrypted_value.clone()),
                            _ => {}
                        }
                        current_mutation.messages = Some(self.messages.clone());
                    }
                } else {
                    for message in &mut self.messages {
                        if let Some(calls) = message.tool_calls_mut()
                            && let Some(call) =
                                calls.iter_mut().find(|c| c.id.as_ref() == e.entity_id)
                        {
                            call.encrypted_value = Some(e.encrypted_value.clone());
                            current_mutation.messages = Some(self.messages.clone());
                            break;
                        }
                    }
                }
                for subscriber in &self.subscribers {
                    let params = self.to_subscriber_params();
                    mutations.push(
                        subscriber
                            .on_reasoning_encrypted_value_event(e, params)
                            .await?,
                    );
                }
            }
            Event::SubagentStarted(e) => {
                for subscriber in &self.subscribers {
                    let params = self.to_subscriber_params();
                    mutations.push(subscriber.on_subagent_started_event(e, params).await?);
                }
            }
            Event::SubagentFinished(e) => {
                for subscriber in &self.subscribers {
                    let params = self.to_subscriber_params();
                    mutations.push(subscriber.on_subagent_finished_event(e, params).await?);
                }
            }
            Event::SubagentError(e) => {
                for subscriber in &self.subscribers {
                    let params = self.to_subscriber_params();
                    mutations.push(subscriber.on_subagent_error_event(e, params).await?);
                }
            }
            _ => {}
        }

        for mutation in mutations {
            if mutation.stop_propagation {
                self.update_from_mutation(&mutation);
                return Ok(mutation);
            } else {
                self.process_mutation(mutation, &mut current_mutation);
            }
        }

        Ok(current_mutation)
    }

    pub async fn apply_mutation(
        &mut self,
        mutation: AgentStateMutation<StateT>,
    ) -> Result<(), AgentError> {
        if let Some(messages) = mutation.messages {
            // Check for new messages to notify about
            let old_message_ids: HashSet<&MessageId> =
                self.messages.iter().map(|m| m.id()).collect();

            let new_messages: Vec<&Message> = messages
                .iter()
                .filter(|m| !old_message_ids.contains(m.id()))
                .collect();

            // Set the new messages first
            self.messages = messages.clone();

            // Notify about new messages
            for message in new_messages {
                self.notify_new_message(message).await?;

                // If the message is from assistant and has tool calls, notify about those too
                if message.role() == Role::Assistant && message.tool_calls().is_some() {
                    for tool_call in message.tool_calls().unwrap() {
                        self.notify_new_tool_call(tool_call).await?;
                    }
                }
            }

            // Then notify about messages changed
            self.notify_messages_changed().await?;
        }

        if let Some(state) = mutation.state {
            self.state = state;
            self.notify_state_changed().await?;
        }

        Ok(())
    }

    async fn notify_new_message(&self, message: &Message) -> Result<(), AgentError> {
        for subscriber in &self.subscribers {
            subscriber
                .on_new_message(message, self.to_subscriber_params())
                .await?;
        }
        Ok(())
    }

    async fn notify_new_tool_call(&self, tool_call: &ToolCall) -> Result<(), AgentError> {
        for subscriber in &self.subscribers {
            subscriber
                .on_new_tool_call(tool_call, self.to_subscriber_params())
                .await?;
        }
        Ok(())
    }

    async fn notify_messages_changed(&self) -> Result<(), AgentError> {
        for subscriber in &self.subscribers {
            subscriber
                .on_messages_changed(self.to_subscriber_params())
                .await?;
        }
        Ok(())
    }

    async fn notify_state_changed(&self) -> Result<(), AgentError> {
        for subscriber in &self.subscribers {
            subscriber
                .on_state_changed(self.to_subscriber_params())
                .await?;
        }
        Ok(())
    }

    pub async fn on_error(&self, error: &AgentError) -> Result<(), AgentError> {
        error!("Agent error: {error}");
        for subscriber in &self.subscribers {
            let _mutation = subscriber
                .on_run_failed(error, self.to_subscriber_params())
                .await?;
        }
        Ok(())
    }

    pub async fn on_finalize(&self) -> Result<(), AgentError> {
        for subscriber in &self.subscribers {
            let _mutation = subscriber
                .on_run_finalized(self.to_subscriber_params())
                .await?;
        }
        Ok(())
    }
}

fn merge_message_metadata(
    message: &mut Message,
    extra: &Option<serde_json::Map<String, JsonValue>>,
) {
    let Some(extra) = extra else {
        return;
    };
    let metadata = match message {
        Message::Developer { metadata, .. }
        | Message::System { metadata, .. }
        | Message::Assistant { metadata, .. }
        | Message::User { metadata, .. }
        | Message::Activity { metadata, .. }
        | Message::Reasoning { metadata, .. }
        | Message::Tool { metadata, .. } => metadata,
    };
    metadata
        .get_or_insert_with(Default::default)
        .extend(extra.clone());
}

#[cfg(test)]
mod name_tests {
    use super::*;
    use crate::core::types::{RunId, ThreadId};
    use serde_json::json;

    #[tokio::test]
    async fn named_text_events_materialize_author() {
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
        let start: Event = serde_json::from_value(
            json!({"type":"TEXT_MESSAGE_START","messageId":"m","role":"assistant","name":"Ada"}),
        )
        .unwrap();
        handler.handle_event(&start).await.unwrap();
        assert_eq!(
            serde_json::to_value(&handler.messages[0]).unwrap()["name"],
            "Ada"
        );

        let chunk: Event = serde_json::from_value(json!({"type":"TEXT_MESSAGE_CHUNK","messageId":"other","role":"assistant","name":"Grace","delta":"Hi"})).unwrap();
        handler.handle_event(&chunk).await.unwrap();
        let materialized = serde_json::to_value(&handler.messages[1]).unwrap();
        assert_eq!(materialized["name"], "Grace");
        assert_eq!(materialized["content"], "Hi");
    }
}

#[cfg(test)]
mod conformance_tests {
    use super::*;
    use crate::core::types::{RunId, ThreadId};
    use serde_json::{Value, json};

    fn subset(actual: &Value, expected: &Value) {
        match (actual, expected) {
            (Value::Object(a), Value::Object(e)) => {
                for (key, value) in e {
                    assert!(a.contains_key(key), "missing {key} in {actual}");
                    subset(&a[key], value);
                }
            }
            (Value::Array(a), Value::Array(e)) => {
                assert_eq!(a.len(), e.len(), "array mismatch: {actual}");
                for (a, e) in a.iter().zip(e) {
                    subset(a, e);
                }
            }
            _ => assert_eq!(actual, expected),
        }
    }

    struct Hooks(std::sync::Arc<std::sync::Mutex<Vec<&'static str>>>);

    #[async_trait::async_trait]
    impl crate::subscriber::AgentSubscriber for Hooks {
        async fn on_activity_snapshot_event(
            &self,
            _event: &crate::core::event::ActivitySnapshotEvent,
            _params: AgentSubscriberParams<'async_trait, JsonValue, JsonValue>,
        ) -> Result<AgentStateMutation<JsonValue>, AgentError> {
            self.0.lock().unwrap().push("activity");
            Ok(AgentStateMutation::default())
        }
        async fn on_reasoning_message_start_event(
            &self,
            _event: &crate::core::event::ReasoningMessageStartEvent,
            _params: AgentSubscriberParams<'async_trait, JsonValue, JsonValue>,
        ) -> Result<AgentStateMutation<JsonValue>, AgentError> {
            self.0.lock().unwrap().push("reasoning");
            Ok(AgentStateMutation::default())
        }
        async fn on_subagent_started_event(
            &self,
            _event: &crate::core::event::SubagentStartedEvent,
            _params: AgentSubscriberParams<'async_trait, JsonValue, JsonValue>,
        ) -> Result<AgentStateMutation<JsonValue>, AgentError> {
            self.0.lock().unwrap().push("subagent");
            Ok(AgentStateMutation::default())
        }
    }

    #[tokio::test]
    async fn activity_reasoning_and_subagent_hooks_are_delivered() {
        let calls = std::sync::Arc::new(std::sync::Mutex::new(Vec::new()));
        let input = RunAgentInput::new(
            ThreadId::from("t"),
            RunId::from("r"),
            json!({}),
            vec![],
            vec![],
            vec![],
            json!({}),
        );
        let mut handler = EventHandler::new(
            vec![],
            json!({}),
            &input,
            Subscribers::from_subscriber(Hooks(calls.clone())),
        );
        for raw in [
            json!({"type":"RUN_STARTED","threadId":"t","runId":"r"}),
            json!({"type":"ACTIVITY_SNAPSHOT","messageId":"a","activityType":"search","content":{}}),
            json!({"type":"REASONING_MESSAGE_START","messageId":"m","role":"reasoning"}),
            json!({"type":"SUBAGENT_STARTED","subagentRunId":"s","name":"worker"}),
        ] {
            let event: Event = serde_json::from_value(raw).unwrap();
            handler.handle_event(&event).await.unwrap();
        }
        assert_eq!(
            *calls.lock().unwrap(),
            ["activity", "reasoning", "subagent"]
        );
    }

    #[tokio::test]
    async fn named_streams_match_expectations() {
        let names = [
            "activity-snapshot-then-delta",
            "activity-replace-false-preserves",
            "chunk-expansion-assembles",
            "content-without-start-fatal",
            "custom-closes-chunk-stream-fatal",
            "encrypted-value-unknown-entity-tolerated",
            "reasoning-span-and-message",
            "reasoning-discipline-verified",
            "tool-result-parts-mint-tool-message",
            "tool-call-triad-assembles",
            "run-finished-interrupt-outcome-accepted",
            "state-delta-unappliable-warns-and-keeps",
            "metadata-merges-last-write-wins",
            "subagent-unannounced-attribution-accepted",
            "subagent-terminal-closes-open-chunk-stream",
            "subagent-finished-unstarted-fatal",
        ];
        for name in names {
            let path = format!(
                "{}/../../../../../spec/1.0/conformance/streams/{name}.json",
                env!("CARGO_MANIFEST_DIR")
            );
            let fixture: Value =
                serde_json::from_str(&std::fs::read_to_string(path).unwrap()).unwrap();
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
            let mut error = None;
            let mut chunks = crate::chunk::ChunkExpander::default();
            for raw in fixture["stream"].as_array().unwrap() {
                let event: Event = serde_json::from_value(raw.clone())
                    .unwrap_or_else(|e| panic!("{name}: {e}: {raw}"));
                let expanded = match chunks.expand(event) {
                    Ok(events) => events,
                    Err(e) => {
                        error = Some(e.to_string());
                        break;
                    }
                };
                for event in expanded {
                    match handler.handle_event(&event).await {
                        Ok(mutation) => handler.apply_mutation(mutation).await.unwrap(),
                        Err(e) => {
                            error = Some(e.to_string());
                            break;
                        }
                    }
                }
                if error.is_some() {
                    break;
                }
            }
            let expected = &fixture["expect"];
            if expected["outcome"] == "failed" {
                let error = error.unwrap_or_else(|| panic!("{name}: expected failure"));
                assert!(
                    error.contains(expected["errorContains"].as_str().unwrap()),
                    "{name}: {error}"
                );
                continue;
            }
            assert!(error.is_none(), "{name}: {error:?}");
            if let Some(count) = expected["messageCount"].as_u64() {
                assert_eq!(handler.messages.len() as u64, count, "{name}");
            }
            if let Some(messages) = expected["messages"].as_array() {
                for (actual, expected) in handler.messages.iter().zip(messages) {
                    subset(&serde_json::to_value(actual).unwrap(), expected);
                }
            }
            if let Some(state) = expected.get("state") {
                assert_eq!(
                    serde_json::to_value(&handler.state).unwrap(),
                    *state,
                    "{name}"
                );
            }
        }
    }
}
