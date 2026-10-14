use crate::agent::{AgentError, AgentStateMutation, RunTerminalStatus};
use crate::core::event::Event;
use crate::core::types::{
    FunctionCall, Message, MessageId, ReasoningEncryptedValueSubtype, Role, RunAgentInput,
    RunFinishedOutcome, TokenUsage, ToolCall, ToolCallId,
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
    pub output: Option<JsonValue>,
    pub terminal_status: Option<RunTerminalStatus>,
    current_run_id: Option<crate::core::types::RunId>,
    pub outcome: Option<RunFinishedOutcome>,
    pub usage: Option<Vec<TokenUsage>>,
    active_run: bool,
    finished_run: bool,
    last_terminal_was_error: bool,
    active_text: HashMap<MessageId, Option<String>>,
    active_reasoning: HashMap<MessageId, Option<String>>,
    active_span: HashMap<MessageId, Option<String>>,
    active_tools: HashMap<ToolCallId, Option<String>>,
    active_steps: HashSet<(Option<String>, String)>,
    activity_owners: HashMap<MessageId, Option<String>>,
    pub run_error: Option<String>,
    active_subagents: HashSet<String>,
    closed_subagents: HashSet<String>,
    attributed_subagents: HashSet<String>,
    message_owners: HashMap<MessageId, Option<String>>,
    tool_owners: HashMap<ToolCallId, Option<String>>,
    notified_messages: HashSet<MessageId>,
    notified_tools: HashSet<ToolCallId>,
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
        let notified_messages = messages.iter().map(|m| m.id().clone()).collect();
        let notified_tools = messages
            .iter()
            .filter_map(|m| m.tool_calls())
            .flatten()
            .map(|call| call.id.clone())
            .collect();
        let mut handler = Self {
            messages,
            state,
            input,
            subscribers,
            result: JsonValue::Null,
            output: None,
            terminal_status: None,
            current_run_id: None,
            outcome: None,
            usage: None,
            active_run: false,
            finished_run: false,
            last_terminal_was_error: false,
            active_text: HashMap::new(),
            active_reasoning: HashMap::new(),
            active_span: HashMap::new(),
            active_tools: HashMap::new(),
            active_steps: HashSet::new(),
            activity_owners: HashMap::new(),
            run_error: None,
            active_subagents: HashSet::new(),
            closed_subagents: HashSet::new(),
            attributed_subagents: HashSet::new(),
            message_owners: HashMap::new(),
            tool_owners: HashMap::new(),
            notified_messages,
            notified_tools,
            text_chunk: None,
            tool_chunk: None,
        };
        handler.seed_owners();
        handler
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
        if self.finished_run
            && !matches!(event, Event::RunStarted(_))
            && (!matches!(event, Event::RunError(_)) || self.last_terminal_was_error)
        {
            return Err(fail(
                "The run has already finished with 'RUN_FINISHED' or 'RUN_ERROR'".into(),
            ));
        }
        if !self.active_run && !matches!(event, Event::RunStarted(_) | Event::RunError(_)) {
            return Err(fail(
                "First event must be 'RUN_STARTED' or 'RUN_ERROR'".into(),
            ));
        }
        match event {
            Event::RunStarted(_) if self.active_run => {
                return Err(fail(
                    "Cannot send 'RUN_STARTED' while a run is still active".into(),
                ));
            }
            Event::RunStarted(e) => {
                if e.thread_id != self.input.thread_id {
                    return Err(AgentError::ExchangeIdentity(
                        "RUN_STARTED threadId does not match the request".into(),
                    ));
                }
                self.current_run_id = Some(e.run_id.clone());
                self.result = JsonValue::Null;
                self.output = None;
                self.outcome = None;
                self.usage = None;
                self.run_error = None;
                self.terminal_status = None;
                self.active_run = true;
                self.finished_run = false;
                self.last_terminal_was_error = false;
                self.active_text.clear();
                self.active_reasoning.clear();
                self.active_span.clear();
                self.active_tools.clear();
                self.active_steps.clear();
                self.active_subagents.clear();
                self.closed_subagents.clear();
                self.attributed_subagents.clear();
                self.message_owners.clear();
                self.tool_owners.clear();
                self.activity_owners.clear();
                self.seed_owners();
            }
            Event::RunFinished(e) => {
                if e.thread_id != self.input.thread_id
                    || Some(&e.run_id) != self.current_run_id.as_ref()
                {
                    return Err(AgentError::ExchangeIdentity(
                        "RUN_FINISHED identity does not match its RUN_STARTED and request threadId"
                            .into(),
                    ));
                }
                if !self.active_subagents.is_empty() {
                    return Err(fail(format!(
                        "RUN_FINISHED with active subagent {}",
                        self.active_subagents.iter().next().unwrap()
                    )));
                }
                if !self.active_text.is_empty()
                    || !self.active_reasoning.is_empty()
                    || !self.active_span.is_empty()
                    || !self.active_tools.is_empty()
                    || !self.active_steps.is_empty()
                {
                    return Err(fail("Cannot send 'RUN_FINISHED' while text messages are still active or another streamed item is open".into()));
                }
                self.active_run = false;
                self.finished_run = true;
                self.last_terminal_was_error = false;
                self.terminal_status = Some(RunTerminalStatus::Finished {
                    thread_id: e.thread_id.clone(),
                    run_id: e.run_id.clone(),
                });
            }
            Event::RunError(e) => {
                self.active_run = false;
                self.finished_run = true;
                self.last_terminal_was_error = true;
                self.run_error = Some(e.message.clone());
                self.result = JsonValue::Null;
                self.output = None;
                self.outcome = None;
                self.terminal_status = Some(RunTerminalStatus::ProducerError(e.clone()));
                self.active_text.clear();
                self.active_reasoning.clear();
                self.active_span.clear();
                self.active_tools.clear();
                self.active_steps.clear();
                self.active_subagents.clear();
                self.activity_owners.clear();
            }
            Event::TextMessageStart(e) => {
                if !matches!(
                    e.role,
                    None | Some(Role::Assistant | Role::User | Role::System | Role::Developer)
                ) {
                    return Err(fail("TEXT_MESSAGE_START has an invalid text role".into()));
                }
                let owner = self
                    .message_owners
                    .entry(e.message_id.clone())
                    .or_insert_with(|| e.base.subagent_run_id.clone());
                check_owner(owner, &e.base.subagent_run_id, e.message_id.as_ref())?;
                if self
                    .active_text
                    .insert(e.message_id.clone(), owner.clone())
                    .is_some()
                {
                    return Err(fail(format!(
                        "TEXT_MESSAGE_START for open message '{}'",
                        e.message_id
                    )));
                }
            }
            Event::TextMessageContent(e) => {
                let owner = self.active_text.get(&e.message_id).ok_or_else(|| {
                    fail(format!(
                        "No active text message found with ID '{}'",
                        e.message_id
                    ))
                })?;
                check_owner(owner, &e.base.subagent_run_id, e.message_id.as_ref())?;
            }
            Event::TextMessageEnd(e) => {
                let owner = self.active_text.get(&e.message_id).ok_or_else(|| {
                    fail(format!(
                        "No active text message found with ID '{}'",
                        e.message_id
                    ))
                })?;
                check_owner(owner, &e.base.subagent_run_id, e.message_id.as_ref())?;
                self.active_text.remove(&e.message_id);
            }
            Event::ReasoningStart(e) => {
                if self
                    .active_span
                    .insert(e.message_id.clone(), e.base.subagent_run_id.clone())
                    .is_some()
                {
                    return Err(fail("REASONING_START for an already open span".into()));
                }
            }
            Event::ReasoningEnd(e) => {
                let owner = self
                    .active_span
                    .get(&e.message_id)
                    .ok_or_else(|| fail("No active reasoning span".into()))?;
                check_owner(owner, &e.base.subagent_run_id, e.message_id.as_ref())?;
                self.active_span.remove(&e.message_id);
            }
            Event::ReasoningMessageStart(e) => {
                if e.role != Role::Reasoning {
                    return Err(fail(
                        "Reasoning message role: expected \\\"reasoning\\\"".into(),
                    ));
                }
                let owner = self
                    .message_owners
                    .entry(e.message_id.clone())
                    .or_insert_with(|| e.base.subagent_run_id.clone());
                check_owner(owner, &e.base.subagent_run_id, e.message_id.as_ref())?;
                if self
                    .active_reasoning
                    .insert(e.message_id.clone(), owner.clone())
                    .is_some()
                {
                    return Err(fail(format!(
                        "REASONING_MESSAGE_START for open message '{}'",
                        e.message_id
                    )));
                }
            }
            Event::ReasoningMessageContent(e) => {
                let owner = self.active_reasoning.get(&e.message_id).ok_or_else(|| {
                    fail(format!(
                        "No active reasoning message found with ID '{}'",
                        e.message_id
                    ))
                })?;
                check_owner(owner, &e.base.subagent_run_id, e.message_id.as_ref())?;
            }
            Event::ReasoningMessageEnd(e) => {
                let owner = self.active_reasoning.get(&e.message_id).ok_or_else(|| {
                    fail(format!(
                        "No active reasoning message found with ID '{}'",
                        e.message_id
                    ))
                })?;
                check_owner(owner, &e.base.subagent_run_id, e.message_id.as_ref())?;
                self.active_reasoning.remove(&e.message_id);
            }
            Event::ToolCallStart(e) => {
                let inherited = e
                    .parent_message_id
                    .as_ref()
                    .and_then(|id| self.message_owners.get(id))
                    .cloned();
                if let Some(parent_id) = &e.parent_message_id
                    && let Some(parent) = self.messages.iter().find(|m| m.id() == parent_id)
                {
                    if parent.role() != Role::Assistant {
                        return Err(fail(format!(
                            "Tool call '{}' parent message is not assistant",
                            e.tool_call_id
                        )));
                    }
                    check_owner(
                        &message_owner(parent).cloned(),
                        &e.base.subagent_run_id,
                        e.tool_call_id.as_ref(),
                    )?;
                }
                let effective_owner = inherited
                    .clone()
                    .unwrap_or_else(|| e.base.subagent_run_id.clone());
                if let Some(owner) = self.tool_owners.get(&e.tool_call_id) {
                    check_owner(owner, &e.base.subagent_run_id, e.tool_call_id.as_ref())?;
                    if inherited.is_some() && owner != &effective_owner {
                        return Err(fail("Tool call owner does not match parent message".into()));
                    }
                }
                let owner = self
                    .tool_owners
                    .entry(e.tool_call_id.clone())
                    .or_insert(effective_owner)
                    .clone();
                if self
                    .active_tools
                    .insert(e.tool_call_id.clone(), owner)
                    .is_some()
                {
                    return Err(fail(format!(
                        "TOOL_CALL_START for open call '{}'",
                        e.tool_call_id
                    )));
                }
            }
            Event::ToolCallArgs(e) => {
                let owner = self.active_tools.get(&e.tool_call_id).ok_or_else(|| {
                    fail(format!(
                        "No active tool call found with ID '{}'",
                        e.tool_call_id
                    ))
                })?;
                check_owner(owner, &e.base.subagent_run_id, e.tool_call_id.as_ref())?;
            }
            Event::ToolCallEnd(e) => {
                let owner = self.active_tools.get(&e.tool_call_id).ok_or_else(|| {
                    fail(format!(
                        "No active tool call found with ID '{}'",
                        e.tool_call_id
                    ))
                })?;
                check_owner(owner, &e.base.subagent_run_id, e.tool_call_id.as_ref())?;
                self.active_tools.remove(&e.tool_call_id);
            }
            Event::StepStarted(e) => {
                let key = (e.base.subagent_run_id.clone(), e.step_name.clone());
                if !self.active_steps.insert(key) {
                    return Err(fail(format!(
                        "STEP_STARTED for already open step '{}' in this owner",
                        e.step_name
                    )));
                }
            }
            Event::StepFinished(e) => {
                let key = (e.base.subagent_run_id.clone(), e.step_name.clone());
                if !self.active_steps.remove(&key) {
                    return Err(fail(format!(
                        "STEP_FINISHED for step that was not started in this owner: {}",
                        e.step_name
                    )));
                }
            }
            Event::ActivitySnapshot(e) => {
                let existing = self.messages.iter().find(|m| m.id() == &e.message_id);
                if existing.is_none() || e.replace.unwrap_or(true) {
                    self.activity_owners
                        .insert(e.message_id.clone(), e.base.subagent_run_id.clone());
                }
            }
            Event::ActivityDelta(e) => {
                // A reminted non-activity target is the tolerated missing-target
                // case, not a continuation of its former activity ownership.
                if self
                    .messages
                    .iter()
                    .any(|m| m.id() == &e.message_id && m.role() == Role::Activity)
                    && let Some(owner) = self.activity_owners.get(&e.message_id)
                {
                    check_owner(owner, &e.base.subagent_run_id, e.message_id.as_ref())?;
                }
            }
            Event::ToolCallResult(e) => {
                self.activity_owners.remove(&e.message_id);
                self.message_owners
                    .insert(e.message_id.clone(), e.base.subagent_run_id.clone());
            }
            Event::SubagentStarted(e) => {
                if self.active_subagents.contains(&e.subagent_run_id)
                    || self.closed_subagents.contains(&e.subagent_run_id)
                {
                    return Err(fail("Subagent invocation ID is already active or was already closed in this run".into()));
                }
                if self.attributed_subagents.contains(&e.subagent_run_id) {
                    return Err(fail(
                        "SUBAGENT_STARTED must precede attributed events for that invocation"
                            .into(),
                    ));
                }
                if let Some(parent) = &e.parent_subagent_run_id
                    && !self.active_subagents.contains(parent)
                    && !self.closed_subagents.contains(parent)
                {
                    return Err(fail(
                        "parentSubagentRunId has not been started in this run".into(),
                    ));
                }
                self.active_subagents.insert(e.subagent_run_id.clone());
            }
            Event::SubagentFinished(e) => {
                if !self.active_subagents.remove(&e.subagent_run_id) {
                    return Err(fail(format!("Subagent not started: {}", e.subagent_run_id)));
                }
                self.closed_subagents.insert(e.subagent_run_id.clone());
            }
            Event::SubagentError(e) => {
                if !self.active_subagents.remove(&e.subagent_run_id) {
                    return Err(fail(format!("Subagent not started: {}", e.subagent_run_id)));
                }
                self.closed_subagents.insert(e.subagent_run_id.clone());
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
        if let Some(owner) = event_attribution(event) {
            self.attributed_subagents.insert(owner.to_owned());
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
                if let Some(existing) = self.messages.iter_mut().find(|m| m.id() == &e.message_id) {
                    merge_message_metadata(existing, &e.base.metadata);
                } else {
                    self.messages.push(text_message(e)?);
                }
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

                if let Some(existing_call) = self
                    .messages
                    .iter_mut()
                    .filter_map(|message| message.tool_calls_mut())
                    .flatten()
                    .find(|call| call.id == e.tool_call_id)
                {
                    if let Some(metadata) = &e.base.metadata {
                        existing_call
                            .metadata
                            .get_or_insert_with(Default::default)
                            .extend(metadata.clone());
                    }
                } else if let Some(last_message) = self.messages.iter_mut().rev().find(|m| {
                    m.role() == Role::Assistant && Some(m.id()) == e.parent_message_id.as_ref()
                }) {
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
                for message in &self.messages {
                    if message
                        .tool_calls()
                        .is_some_and(|calls| calls.iter().any(|call| call.id == e.tool_call_id))
                    {
                        self.message_owners
                            .insert(message.id().clone(), message_owner(message).cloned());
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
                let call = self
                    .messages
                    .iter_mut()
                    .filter_map(|message| message.tool_calls_mut())
                    .flatten()
                    .find(|call| call.id == e.tool_call_id)
                    .ok_or_else(|| {
                        AgentError::exec(format!("No tool call with ID '{}'", e.tool_call_id))
                    })?;
                call.function.arguments.push_str(&e.delta);
                if let Some(metadata) = &e.base.metadata {
                    call.metadata
                        .get_or_insert_with(Default::default)
                        .extend(metadata.clone());
                }
                let tool_call_buffer = call.function.arguments.clone();
                let tool_call_name = call.function.name.clone();
                let partial_args =
                    serde_json::from_str::<HashMap<String, JsonValue>>(&tool_call_buffer)
                        .unwrap_or_default();
                current_mutation.messages = Some(self.messages.clone());

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
                let call = self
                    .messages
                    .iter_mut()
                    .filter_map(|message| message.tool_calls_mut())
                    .flatten()
                    .find(|call| call.id == e.tool_call_id)
                    .ok_or_else(|| {
                        AgentError::exec(format!("No tool call with ID '{}'", e.tool_call_id))
                    })?;
                if let Some(metadata) = &e.base.metadata {
                    call.metadata
                        .get_or_insert_with(Default::default)
                        .extend(metadata.clone());
                }
                let tool_call_name = call.function.name.clone();
                let tool_call_args =
                    serde_json::from_str::<HashMap<String, JsonValue>>(&call.function.arguments)
                        .unwrap_or_default();
                if e.base.metadata.is_some() {
                    current_mutation.messages = Some(self.messages.clone());
                }

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
                let message = Message::Tool {
                    id: e.message_id.clone(),
                    content: e.content.clone(),
                    tool_call_id: e.tool_call_id.clone(),
                    error: None,
                    metadata: e.base.metadata.clone(),
                    encrypted_value: None,
                    subagent_run_id: e.base.subagent_run_id.clone(),
                };
                upsert_message(&mut self.messages, message);
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

                if json_patch::patch(&mut state_val, &patches).is_err() {
                    self.warn(
                        "/delta",
                        "Failed to apply state patch: patch does not apply to the current state",
                    )
                    .await?;
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
                reconcile_messages(&mut self.messages, &e.messages);
                self.seed_owners();
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
                self.output = e.result.clone();
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
                if let Some(existing) = self.messages.iter_mut().find(|m| m.id() == &e.message_id) {
                    if let Message::Activity {
                        content,
                        activity_type,
                        subagent_run_id,
                        ..
                    } = existing
                    {
                        if e.replace.unwrap_or(true) {
                            *content = e.content.clone();
                            *activity_type = e.activity_type.clone();
                            *subagent_run_id = e.base.subagent_run_id.clone();
                        }
                        merge_message_metadata(existing, &e.base.metadata);
                    } else if e.replace.unwrap_or(true) {
                        *existing = activity_message(e);
                    }
                } else {
                    self.messages.push(activity_message(e));
                }
                current_mutation.messages = Some(self.messages.clone());
                for subscriber in &self.subscribers {
                    let params = self.to_subscriber_params();
                    mutations.push(subscriber.on_activity_snapshot_event(e, params).await?);
                }
            }
            Event::ActivityDelta(e) => {
                let mut warning = None;
                if let Some(message @ Message::Activity { .. }) =
                    self.messages.iter_mut().find(|m| m.id() == &e.message_id)
                {
                    merge_message_metadata(message, &e.base.metadata);
                    let Message::Activity {
                        content,
                        activity_type,
                        ..
                    } = message
                    else {
                        unreachable!()
                    };
                    let mut value = JsonValue::Object(content.clone());
                    let patches: Vec<PatchOperation> =
                        serde_json::from_value(serde_json::to_value(&e.patch)?)?;
                    match json_patch::patch(&mut value, &patches) {
                        Ok(()) => match value {
                            JsonValue::Object(map) => {
                                *content = map;
                                *activity_type = e.activity_type.clone();
                            }
                            _ => {
                                warning = Some(
                                    "Failed to apply activity patch: content must remain an object",
                                )
                            }
                        },
                        Err(_) => {
                            warning = Some(
                                "Failed to apply activity patch: patch does not apply to the current content",
                            )
                        }
                    }
                    current_mutation.messages = Some(self.messages.clone());
                } else {
                    warning =
                        Some("ACTIVITY_DELTA target is missing or is not an activity message");
                }
                if let Some(message) = warning {
                    self.warn("/patch", message).await?;
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
                if let Some(existing) = self.messages.iter_mut().find(|m| m.id() == &e.message_id) {
                    merge_message_metadata(existing, &e.base.metadata);
                } else {
                    self.messages.push(Message::Reasoning {
                        id: e.message_id.clone(),
                        content: String::new(),
                        encrypted_value: None,
                        metadata: e.base.metadata.clone(),
                        subagent_run_id: e.base.subagent_run_id.clone(),
                    });
                }
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
                if let Some(message) = self.messages.iter_mut().find(|m| m.id() == &e.message_id) {
                    merge_message_metadata(message, &e.base.metadata);
                    current_mutation.messages = Some(self.messages.clone());
                }
                for subscriber in &self.subscribers {
                    let params = self.to_subscriber_params();
                    mutations.push(subscriber.on_reasoning_message_end_event(e, params).await?);
                }
            }
            Event::ReasoningMessageChunk(e) => {
                let id = e
                    .message_id
                    .clone()
                    .or_else(|| self.active_reasoning.keys().next().cloned());
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
                            }
                            | Message::User {
                                encrypted_value, ..
                            }
                            | Message::System {
                                encrypted_value, ..
                            }
                            | Message::Developer {
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
            self.messages = messages;
            let new_messages: Vec<_> = self
                .messages
                .iter()
                .filter(|message| !self.notified_messages.contains(message.id()))
                .cloned()
                .collect();
            for message in new_messages {
                self.notified_messages.insert(message.id().clone());
                self.notify_new_message(&message).await?;
            }
            let new_calls: Vec<_> = self
                .messages
                .iter()
                .filter_map(|m| m.tool_calls())
                .flatten()
                .filter(|call| !self.notified_tools.contains(&call.id))
                .cloned()
                .collect();
            for call in new_calls {
                self.notified_tools.insert(call.id.clone());
                self.notify_new_tool_call(&call).await?;
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

    async fn warn(&self, path: &str, message: &str) -> Result<(), AgentError> {
        use crate::core::processing::{Diagnostic, DiagnosticCategory};
        let diagnostic = Diagnostic {
            category: if message.starts_with("ACTIVITY_DELTA") {
                DiagnosticCategory::MissingTarget
            } else {
                DiagnosticCategory::PatchNotApplied
            },
            path: path.to_owned(),
            message: message.to_owned(),
        };
        log::warn!("{}", diagnostic.message);
        for subscriber in &self.subscribers {
            subscriber
                .on_diagnostic(&diagnostic, self.to_subscriber_params())
                .await?;
        }
        Ok(())
    }

    fn seed_owners(&mut self) {
        self.message_owners.clear();
        self.activity_owners.clear();
        self.tool_owners.clear();
        for message in &self.messages {
            let owner = message_owner(message).cloned();
            if message.role() == Role::Activity {
                self.activity_owners
                    .insert(message.id().clone(), owner.clone());
            } else {
                self.message_owners
                    .insert(message.id().clone(), owner.clone());
            }
            if let Some(calls) = message.tool_calls() {
                for call in calls {
                    self.tool_owners.insert(call.id.clone(), owner.clone());
                }
            }
        }
        for (id, owner) in &mut self.active_text {
            if let Some(current) = self.message_owners.get(id) {
                *owner = current.clone();
            }
        }
        for (id, owner) in &mut self.active_reasoning {
            if let Some(current) = self.message_owners.get(id) {
                *owner = current.clone();
            }
        }
        for (id, owner) in &mut self.active_tools {
            if let Some(current) = self.tool_owners.get(id) {
                *owner = current.clone();
            }
        }
    }

    pub async fn on_initialize(&mut self) -> Result<(), AgentError> {
        for subscriber in &self.subscribers.clone() {
            let mutation = subscriber
                .on_run_initialized(self.to_subscriber_params())
                .await?;
            self.apply_mutation(mutation).await?;
        }
        Ok(())
    }

    pub async fn on_error(&self, error: &AgentError) -> Result<(), AgentError> {
        error!("Agent error: {error}");
        let mut first_error = None;
        for subscriber in &self.subscribers {
            if let Err(error) = subscriber
                .on_run_failed(error, self.to_subscriber_params())
                .await
            {
                first_error.get_or_insert(error);
            }
        }
        first_error.map_or(Ok(()), Err)
    }

    pub fn check_completion(&self) -> Result<(), AgentError> {
        match &self.terminal_status {
            None => Err(AgentError::IncompleteRun(
                "AG-UI stream ended before RUN_FINISHED or RUN_ERROR for the latest run".into(),
            )),
            Some(RunTerminalStatus::Finished { run_id, .. }) if run_id != &self.input.run_id => {
                Err(AgentError::ExchangeIdentity(
                    "The final successful runId is not the requested runId".into(),
                ))
            }
            Some(_) => Ok(()),
        }
    }

    pub async fn on_finalize(&mut self) -> Result<(), AgentError> {
        let mut first_error = None;
        for subscriber in &self.subscribers.clone() {
            match subscriber
                .on_run_finalized(self.to_subscriber_params())
                .await
            {
                Ok(mutation) => {
                    self.update_from_mutation(&mutation);
                }
                Err(error) => {
                    first_error.get_or_insert(error);
                }
            }
        }
        first_error.map_or(Ok(()), Err)
    }
}

fn text_message(event: &crate::core::event::TextMessageStartEvent) -> Result<Message, AgentError> {
    let mut value = serde_json::json!({
        "id": event.message_id, "role": event.role.clone().unwrap_or(Role::Assistant), "content": "",
    });
    if let Some(name) = &event.name {
        value["name"] = name.clone().into();
    }
    if let Some(metadata) = &event.base.metadata {
        value["metadata"] = JsonValue::Object(metadata.clone());
    }
    if let Some(owner) = &event.base.subagent_run_id {
        value["subagentRunId"] = owner.clone().into();
    }
    Ok(serde_json::from_value(value)?)
}

fn activity_message(event: &crate::core::event::ActivitySnapshotEvent) -> Message {
    Message::Activity {
        id: event.message_id.clone(),
        activity_type: event.activity_type.clone(),
        content: event.content.clone(),
        metadata: event.base.metadata.clone(),
        subagent_run_id: event.base.subagent_run_id.clone(),
    }
}

fn upsert_message(messages: &mut Vec<Message>, message: Message) {
    if let Some(existing) = messages.iter_mut().find(|m| m.id() == message.id()) {
        *existing = message;
    } else {
        messages.push(message);
    }
}

fn event_attribution<S: AgentState>(event: &Event<S>) -> Option<&str> {
    let base = match event {
        Event::TextMessageStart(e) => &e.base,
        Event::TextMessageContent(e) => &e.base,
        Event::TextMessageEnd(e) => &e.base,
        Event::TextMessageChunk(e) => &e.base,
        Event::ToolCallStart(e) => &e.base,
        Event::ToolCallArgs(e) => &e.base,
        Event::ToolCallEnd(e) => &e.base,
        Event::ToolCallChunk(e) => &e.base,
        Event::ToolCallResult(e) => &e.base,
        Event::ReasoningStart(e) => &e.base,
        Event::ReasoningEnd(e) => &e.base,
        Event::ReasoningMessageStart(e) => &e.base,
        Event::ReasoningMessageContent(e) => &e.base,
        Event::ReasoningMessageEnd(e) => &e.base,
        Event::ReasoningMessageChunk(e) => &e.base,
        Event::ReasoningEncryptedValue(e) => &e.base,
        Event::ActivitySnapshot(e) => &e.base,
        Event::ActivityDelta(e) => &e.base,
        Event::StepStarted(e) => &e.base,
        Event::StepFinished(e) => &e.base,
        Event::StateSnapshot(e) => &e.base,
        Event::StateDelta(e) => &e.base,
        Event::Custom(e) => &e.base,
        Event::Raw(e) => &e.base,
        _ => return None,
    };
    base.subagent_run_id.as_deref()
}

fn message_owner(message: &Message) -> Option<&String> {
    match message {
        Message::Developer {
            subagent_run_id, ..
        }
        | Message::System {
            subagent_run_id, ..
        }
        | Message::Assistant {
            subagent_run_id, ..
        }
        | Message::User {
            subagent_run_id, ..
        }
        | Message::Activity {
            subagent_run_id, ..
        }
        | Message::Reasoning {
            subagent_run_id, ..
        }
        | Message::Tool {
            subagent_run_id, ..
        } => subagent_run_id.as_ref(),
    }
}

fn reconcile_messages(current: &mut Vec<Message>, snapshot: &[Message]) {
    let declares_activity = snapshot
        .iter()
        .any(|message| message.role() == Role::Activity);
    let declares_reasoning = snapshot
        .iter()
        .any(|message| message.role() == Role::Reasoning);
    let by_id: HashMap<_, _> = snapshot
        .iter()
        .map(|message| (message.id().clone(), message))
        .collect();
    let mut retained = HashSet::new();
    current.retain_mut(|message| {
        if !retained.insert(message.id().clone()) {
            return false;
        }
        if let Some(replacement) = by_id.get(message.id()) {
            *message = (*replacement).clone();
            true
        } else {
            matches!(message.role(), Role::Activity if !declares_activity)
                || matches!(message.role(), Role::Reasoning if !declares_reasoning)
        }
    });
    let mut seen: HashSet<_> = current.iter().map(|message| message.id().clone()).collect();
    for message in snapshot {
        if seen.insert(message.id().clone()) {
            current.push((*by_id[message.id()]).clone());
        }
    }
}

fn check_owner(
    expected: &Option<String>,
    incoming: &Option<String>,
    entity: &str,
) -> Result<(), AgentError> {
    if incoming
        .as_ref()
        .is_some_and(|tag| Some(tag) != expected.as_ref())
    {
        return Err(AgentError::Execution {
            message: format!("Subagent attribution for '{entity}' does not match the opener"),
        });
    }
    Ok(())
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
        let run_start: Event =
            serde_json::from_value(json!({"type":"RUN_STARTED","threadId":"t","runId":"r"}))
                .unwrap();
        handler.handle_event(&run_start).await.unwrap();
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

    #[test]
    fn snapshots_preserve_client_roles_and_existing_positions() {
        let message = |value: Value| serde_json::from_value::<Message>(value).unwrap();
        let mut current = vec![
            message(json!({"id":"a","role":"assistant","content":"old"})),
            message(json!({"id":"act","role":"activity","activityType":"search","content":{}})),
            message(json!({"id":"r","role":"reasoning","content":"thought"})),
            message(json!({"id":"drop","role":"assistant","content":"obsolete"})),
        ];
        let snapshot = vec![
            message(json!({"id":"new","role":"assistant","content":"appended"})),
            message(json!({"id":"a","role":"assistant","content":"updated"})),
        ];
        reconcile_messages(&mut current, &snapshot);
        let values: Vec<_> = current
            .iter()
            .map(|m| serde_json::to_value(m).unwrap())
            .collect();
        assert_eq!(
            values
                .iter()
                .map(|m| m["id"].as_str().unwrap())
                .collect::<Vec<_>>(),
            ["a", "act", "r", "new"]
        );
        assert_eq!(values[0]["content"], "updated");
        let snapshot = vec![
            message(json!({"id":"a","role":"assistant","content":"updated"})),
            message(json!({"id":"act2","role":"activity","activityType":"search","content":{}})),
            message(json!({"id":"r2","role":"reasoning","content":"new thought"})),
        ];
        reconcile_messages(&mut current, &snapshot);
        let ids: Vec<_> = current.iter().map(|m| m.id().as_ref()).collect();
        assert_eq!(ids, ["a", "act2", "r2"]);
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
            "subagent-attribution-mismatch-fatal",
            "run-error-first-run-continues",
            "run-error-then-new-run-continues",
            "late-run-error-after-finished-run-continues",
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
            let last_start = fixture["stream"]
                .as_array()
                .unwrap()
                .iter()
                .rev()
                .find(|event| event["type"] == "RUN_STARTED");
            let input = RunAgentInput::new(
                ThreadId::from(
                    last_start
                        .and_then(|event| event["threadId"].as_str())
                        .unwrap_or("t"),
                ),
                RunId::from(
                    last_start
                        .and_then(|event| event["runId"].as_str())
                        .unwrap_or("r"),
                ),
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
