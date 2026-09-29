import Foundation

/// Strongly named AG-UI 1.0 definitions with typed accessors for declared fields.
public protocol AGUI1Definition {
    static var definitionName: String { get }
    var document: AGUISchemaDocument { get }
    init(document: AGUISchemaDocument)
    init(_ data: Data) throws
}

public extension AGUI1Definition {
    init(_ data: Data) throws { self.init(document: try AGUISchemaDocument(data, definition: Self.definitionName)) }
    init(forwardCompatible data: Data) throws {
        self.init(document: try AGUISchemaDocument(data, definition: Self.definitionName, allowUnknownFields: true))
    }
    func encoded() throws -> Data { try document.encoded() }
    var fields: [String: Any] { document.fields }
}

private enum AGUI1Nested {
    static func decode<T: AGUI1Definition>(_ value: Any?, as type: T.Type) -> T? {
        guard let value, let data = try? JSONSerialization.data(withJSONObject: value, options: [.fragmentsAllowed]) else { return nil }
        return try? T(data)
    }
}

public enum AGUI1EventTypeValue: String, Sendable, CaseIterable {
    case text_message_start = "TEXT_MESSAGE_START"
    case text_message_content = "TEXT_MESSAGE_CONTENT"
    case text_message_end = "TEXT_MESSAGE_END"
    case text_message_chunk = "TEXT_MESSAGE_CHUNK"
    case tool_call_start = "TOOL_CALL_START"
    case tool_call_args = "TOOL_CALL_ARGS"
    case tool_call_end = "TOOL_CALL_END"
    case tool_call_chunk = "TOOL_CALL_CHUNK"
    case tool_call_result = "TOOL_CALL_RESULT"
    case state_snapshot = "STATE_SNAPSHOT"
    case state_delta = "STATE_DELTA"
    case messages_snapshot = "MESSAGES_SNAPSHOT"
    case activity_snapshot = "ACTIVITY_SNAPSHOT"
    case activity_delta = "ACTIVITY_DELTA"
    case raw = "RAW"
    case custom = "CUSTOM"
    case run_started = "RUN_STARTED"
    case run_finished = "RUN_FINISHED"
    case run_error = "RUN_ERROR"
    case step_started = "STEP_STARTED"
    case step_finished = "STEP_FINISHED"
    case reasoning_start = "REASONING_START"
    case reasoning_message_start = "REASONING_MESSAGE_START"
    case reasoning_message_content = "REASONING_MESSAGE_CONTENT"
    case reasoning_message_end = "REASONING_MESSAGE_END"
    case reasoning_message_chunk = "REASONING_MESSAGE_CHUNK"
    case reasoning_end = "REASONING_END"
    case reasoning_encrypted_value = "REASONING_ENCRYPTED_VALUE"
    case subagent_started = "SUBAGENT_STARTED"
    case subagent_finished = "SUBAGENT_FINISHED"
    case subagent_error = "SUBAGENT_ERROR"
}

public enum AGUI1TextMessageRoleValue: String, Sendable, CaseIterable {
    case developer = "developer"
    case system = "system"
    case assistant = "assistant"
    case user = "user"
}

public enum AGUI1RoleValue: String, Sendable, CaseIterable {
    case developer = "developer"
    case system = "system"
    case assistant = "assistant"
    case user = "user"
    case tool = "tool"
    case activity = "activity"
    case reasoning = "reasoning"
}

public enum AGUI1ReasoningEncryptedValueSubtypeValue: String, Sendable, CaseIterable {
    case tool_call = "tool-call"
    case message = "message"
}

public struct AGUI1Event: AGUI1Definition {
    public static let definitionName = "Event"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var asTextMessageStartEvent: AGUI1TextMessageStartEvent? { AGUI1Nested.decode(document.value, as: AGUI1TextMessageStartEvent.self) }
    public var asTextMessageContentEvent: AGUI1TextMessageContentEvent? { AGUI1Nested.decode(document.value, as: AGUI1TextMessageContentEvent.self) }
    public var asTextMessageEndEvent: AGUI1TextMessageEndEvent? { AGUI1Nested.decode(document.value, as: AGUI1TextMessageEndEvent.self) }
    public var asTextMessageChunkEvent: AGUI1TextMessageChunkEvent? { AGUI1Nested.decode(document.value, as: AGUI1TextMessageChunkEvent.self) }
    public var asToolCallStartEvent: AGUI1ToolCallStartEvent? { AGUI1Nested.decode(document.value, as: AGUI1ToolCallStartEvent.self) }
    public var asToolCallArgsEvent: AGUI1ToolCallArgsEvent? { AGUI1Nested.decode(document.value, as: AGUI1ToolCallArgsEvent.self) }
    public var asToolCallEndEvent: AGUI1ToolCallEndEvent? { AGUI1Nested.decode(document.value, as: AGUI1ToolCallEndEvent.self) }
    public var asToolCallChunkEvent: AGUI1ToolCallChunkEvent? { AGUI1Nested.decode(document.value, as: AGUI1ToolCallChunkEvent.self) }
    public var asToolCallResultEvent: AGUI1ToolCallResultEvent? { AGUI1Nested.decode(document.value, as: AGUI1ToolCallResultEvent.self) }
    public var asStateSnapshotEvent: AGUI1StateSnapshotEvent? { AGUI1Nested.decode(document.value, as: AGUI1StateSnapshotEvent.self) }
    public var asStateDeltaEvent: AGUI1StateDeltaEvent? { AGUI1Nested.decode(document.value, as: AGUI1StateDeltaEvent.self) }
    public var asMessagesSnapshotEvent: AGUI1MessagesSnapshotEvent? { AGUI1Nested.decode(document.value, as: AGUI1MessagesSnapshotEvent.self) }
    public var asActivitySnapshotEvent: AGUI1ActivitySnapshotEvent? { AGUI1Nested.decode(document.value, as: AGUI1ActivitySnapshotEvent.self) }
    public var asActivityDeltaEvent: AGUI1ActivityDeltaEvent? { AGUI1Nested.decode(document.value, as: AGUI1ActivityDeltaEvent.self) }
    public var asRawEvent: AGUI1RawEvent? { AGUI1Nested.decode(document.value, as: AGUI1RawEvent.self) }
    public var asCustomEvent: AGUI1CustomEvent? { AGUI1Nested.decode(document.value, as: AGUI1CustomEvent.self) }
    public var asRunStartedEvent: AGUI1RunStartedEvent? { AGUI1Nested.decode(document.value, as: AGUI1RunStartedEvent.self) }
    public var asRunFinishedEvent: AGUI1RunFinishedEvent? { AGUI1Nested.decode(document.value, as: AGUI1RunFinishedEvent.self) }
    public var asRunErrorEvent: AGUI1RunErrorEvent? { AGUI1Nested.decode(document.value, as: AGUI1RunErrorEvent.self) }
    public var asStepStartedEvent: AGUI1StepStartedEvent? { AGUI1Nested.decode(document.value, as: AGUI1StepStartedEvent.self) }
    public var asStepFinishedEvent: AGUI1StepFinishedEvent? { AGUI1Nested.decode(document.value, as: AGUI1StepFinishedEvent.self) }
    public var asReasoningStartEvent: AGUI1ReasoningStartEvent? { AGUI1Nested.decode(document.value, as: AGUI1ReasoningStartEvent.self) }
    public var asReasoningMessageStartEvent: AGUI1ReasoningMessageStartEvent? { AGUI1Nested.decode(document.value, as: AGUI1ReasoningMessageStartEvent.self) }
    public var asReasoningMessageContentEvent: AGUI1ReasoningMessageContentEvent? { AGUI1Nested.decode(document.value, as: AGUI1ReasoningMessageContentEvent.self) }
    public var asReasoningMessageEndEvent: AGUI1ReasoningMessageEndEvent? { AGUI1Nested.decode(document.value, as: AGUI1ReasoningMessageEndEvent.self) }
    public var asReasoningMessageChunkEvent: AGUI1ReasoningMessageChunkEvent? { AGUI1Nested.decode(document.value, as: AGUI1ReasoningMessageChunkEvent.self) }
    public var asReasoningEndEvent: AGUI1ReasoningEndEvent? { AGUI1Nested.decode(document.value, as: AGUI1ReasoningEndEvent.self) }
    public var asReasoningEncryptedValueEvent: AGUI1ReasoningEncryptedValueEvent? { AGUI1Nested.decode(document.value, as: AGUI1ReasoningEncryptedValueEvent.self) }
    public var asSubagentStartedEvent: AGUI1SubagentStartedEvent? { AGUI1Nested.decode(document.value, as: AGUI1SubagentStartedEvent.self) }
    public var asSubagentFinishedEvent: AGUI1SubagentFinishedEvent? { AGUI1Nested.decode(document.value, as: AGUI1SubagentFinishedEvent.self) }
    public var asSubagentErrorEvent: AGUI1SubagentErrorEvent? { AGUI1Nested.decode(document.value, as: AGUI1SubagentErrorEvent.self) }
}

public struct AGUI1EventType: AGUI1Definition {
    public static let definitionName = "EventType"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var value: AGUI1EventTypeValue? { AGUI1EventTypeValue(rawValue: document.value as? String ?? "") }
}

public struct AGUI1BaseEvent: AGUI1Definition {
    public static let definitionName = "BaseEvent"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: AGUI1EventTypeValue? { AGUI1EventTypeValue(rawValue: fields["type"] as? String ?? "") }
    public var timestamp: Int64? { (fields["timestamp"] as? NSNumber)?.int64Value }
    public var rawEvent: Any? { fields["rawEvent"] }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public func settingType(_ value: AGUI1EventTypeValue?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value.rawValue } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingTimestamp(_ value: Int64?) throws -> Self {
        var updated = fields
        if let value { updated["timestamp"] = value } else { updated.removeValue(forKey: "timestamp") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRawEvent(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["rawEvent"] = value } else { updated.removeValue(forKey: "rawEvent") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(type: AGUI1EventTypeValue) throws {
        let values: [String: Any] = [
            "type": type.rawValue,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1Attributable: AGUI1Definition {
    public static let definitionName = "Attributable"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var subagentRunId: AGUI1SubagentRunId? { AGUI1Nested.decode(fields["subagentRunId"], as: AGUI1SubagentRunId.self) }
    public func settingSubagentRunId(_ value: AGUI1SubagentRunId?) throws -> Self {
        var updated = fields
        if let value { updated["subagentRunId"] = value.document.value } else { updated.removeValue(forKey: "subagentRunId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1SubagentRunId: AGUI1Definition {
    public static let definitionName = "SubagentRunId"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var value: String? { document.value as? String }
}

public struct AGUI1Metadata: AGUI1Definition {
    public static let definitionName = "Metadata"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var values: [String: Any]? { document.value as? [String: Any] }
}

public struct AGUI1State: AGUI1Definition {
    public static let definitionName = "State"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
}

public struct AGUI1TextMessageRole: AGUI1Definition {
    public static let definitionName = "TextMessageRole"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var value: AGUI1TextMessageRoleValue? { AGUI1TextMessageRoleValue(rawValue: document.value as? String ?? "") }
}

public struct AGUI1Role: AGUI1Definition {
    public static let definitionName = "Role"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var value: AGUI1RoleValue? { AGUI1RoleValue(rawValue: document.value as? String ?? "") }
}

public struct AGUI1TextMessageStartEvent: AGUI1Definition {
    public static let definitionName = "TextMessageStartEvent"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var timestamp: Int64? { (fields["timestamp"] as? NSNumber)?.int64Value }
    public var rawEvent: Any? { fields["rawEvent"] }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public var subagentRunId: AGUI1SubagentRunId? { AGUI1Nested.decode(fields["subagentRunId"], as: AGUI1SubagentRunId.self) }
    public var messageId: String? { fields["messageId"] as? String }
    public var role: AGUI1TextMessageRoleValue? { AGUI1TextMessageRoleValue(rawValue: fields["role"] as? String ?? "") }
    public var name: String? { fields["name"] as? String }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingTimestamp(_ value: Int64?) throws -> Self {
        var updated = fields
        if let value { updated["timestamp"] = value } else { updated.removeValue(forKey: "timestamp") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRawEvent(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["rawEvent"] = value } else { updated.removeValue(forKey: "rawEvent") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingSubagentRunId(_ value: AGUI1SubagentRunId?) throws -> Self {
        var updated = fields
        if let value { updated["subagentRunId"] = value.document.value } else { updated.removeValue(forKey: "subagentRunId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMessageId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["messageId"] = value } else { updated.removeValue(forKey: "messageId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRole(_ value: AGUI1TextMessageRoleValue?) throws -> Self {
        var updated = fields
        if let value { updated["role"] = value.rawValue } else { updated.removeValue(forKey: "role") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingName(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["name"] = value } else { updated.removeValue(forKey: "name") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(messageId: String) throws {
        let values: [String: Any] = [
            "type": "TEXT_MESSAGE_START",
            "messageId": messageId,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1TextMessageContentEvent: AGUI1Definition {
    public static let definitionName = "TextMessageContentEvent"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var timestamp: Int64? { (fields["timestamp"] as? NSNumber)?.int64Value }
    public var rawEvent: Any? { fields["rawEvent"] }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public var subagentRunId: AGUI1SubagentRunId? { AGUI1Nested.decode(fields["subagentRunId"], as: AGUI1SubagentRunId.self) }
    public var messageId: String? { fields["messageId"] as? String }
    public var delta: String? { fields["delta"] as? String }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingTimestamp(_ value: Int64?) throws -> Self {
        var updated = fields
        if let value { updated["timestamp"] = value } else { updated.removeValue(forKey: "timestamp") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRawEvent(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["rawEvent"] = value } else { updated.removeValue(forKey: "rawEvent") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingSubagentRunId(_ value: AGUI1SubagentRunId?) throws -> Self {
        var updated = fields
        if let value { updated["subagentRunId"] = value.document.value } else { updated.removeValue(forKey: "subagentRunId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMessageId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["messageId"] = value } else { updated.removeValue(forKey: "messageId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingDelta(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["delta"] = value } else { updated.removeValue(forKey: "delta") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(messageId: String, delta: String) throws {
        let values: [String: Any] = [
            "type": "TEXT_MESSAGE_CONTENT",
            "messageId": messageId,
            "delta": delta,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1TextMessageEndEvent: AGUI1Definition {
    public static let definitionName = "TextMessageEndEvent"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var timestamp: Int64? { (fields["timestamp"] as? NSNumber)?.int64Value }
    public var rawEvent: Any? { fields["rawEvent"] }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public var subagentRunId: AGUI1SubagentRunId? { AGUI1Nested.decode(fields["subagentRunId"], as: AGUI1SubagentRunId.self) }
    public var messageId: String? { fields["messageId"] as? String }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingTimestamp(_ value: Int64?) throws -> Self {
        var updated = fields
        if let value { updated["timestamp"] = value } else { updated.removeValue(forKey: "timestamp") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRawEvent(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["rawEvent"] = value } else { updated.removeValue(forKey: "rawEvent") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingSubagentRunId(_ value: AGUI1SubagentRunId?) throws -> Self {
        var updated = fields
        if let value { updated["subagentRunId"] = value.document.value } else { updated.removeValue(forKey: "subagentRunId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMessageId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["messageId"] = value } else { updated.removeValue(forKey: "messageId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(messageId: String) throws {
        let values: [String: Any] = [
            "type": "TEXT_MESSAGE_END",
            "messageId": messageId,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1TextMessageChunkEvent: AGUI1Definition {
    public static let definitionName = "TextMessageChunkEvent"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var timestamp: Int64? { (fields["timestamp"] as? NSNumber)?.int64Value }
    public var rawEvent: Any? { fields["rawEvent"] }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public var subagentRunId: AGUI1SubagentRunId? { AGUI1Nested.decode(fields["subagentRunId"], as: AGUI1SubagentRunId.self) }
    public var messageId: String? { fields["messageId"] as? String }
    public var role: AGUI1TextMessageRoleValue? { AGUI1TextMessageRoleValue(rawValue: fields["role"] as? String ?? "") }
    public var delta: String? { fields["delta"] as? String }
    public var name: String? { fields["name"] as? String }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingTimestamp(_ value: Int64?) throws -> Self {
        var updated = fields
        if let value { updated["timestamp"] = value } else { updated.removeValue(forKey: "timestamp") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRawEvent(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["rawEvent"] = value } else { updated.removeValue(forKey: "rawEvent") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingSubagentRunId(_ value: AGUI1SubagentRunId?) throws -> Self {
        var updated = fields
        if let value { updated["subagentRunId"] = value.document.value } else { updated.removeValue(forKey: "subagentRunId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMessageId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["messageId"] = value } else { updated.removeValue(forKey: "messageId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRole(_ value: AGUI1TextMessageRoleValue?) throws -> Self {
        var updated = fields
        if let value { updated["role"] = value.rawValue } else { updated.removeValue(forKey: "role") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingDelta(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["delta"] = value } else { updated.removeValue(forKey: "delta") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingName(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["name"] = value } else { updated.removeValue(forKey: "name") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init() throws {
        let values: [String: Any] = [
            "type": "TEXT_MESSAGE_CHUNK",
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1ToolCallStartEvent: AGUI1Definition {
    public static let definitionName = "ToolCallStartEvent"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var timestamp: Int64? { (fields["timestamp"] as? NSNumber)?.int64Value }
    public var rawEvent: Any? { fields["rawEvent"] }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public var subagentRunId: AGUI1SubagentRunId? { AGUI1Nested.decode(fields["subagentRunId"], as: AGUI1SubagentRunId.self) }
    public var toolCallId: String? { fields["toolCallId"] as? String }
    public var toolCallName: String? { fields["toolCallName"] as? String }
    public var parentMessageId: String? { fields["parentMessageId"] as? String }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingTimestamp(_ value: Int64?) throws -> Self {
        var updated = fields
        if let value { updated["timestamp"] = value } else { updated.removeValue(forKey: "timestamp") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRawEvent(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["rawEvent"] = value } else { updated.removeValue(forKey: "rawEvent") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingSubagentRunId(_ value: AGUI1SubagentRunId?) throws -> Self {
        var updated = fields
        if let value { updated["subagentRunId"] = value.document.value } else { updated.removeValue(forKey: "subagentRunId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingToolCallId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["toolCallId"] = value } else { updated.removeValue(forKey: "toolCallId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingToolCallName(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["toolCallName"] = value } else { updated.removeValue(forKey: "toolCallName") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingParentMessageId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["parentMessageId"] = value } else { updated.removeValue(forKey: "parentMessageId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(toolCallId: String, toolCallName: String) throws {
        let values: [String: Any] = [
            "type": "TOOL_CALL_START",
            "toolCallId": toolCallId,
            "toolCallName": toolCallName,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1ToolCallArgsEvent: AGUI1Definition {
    public static let definitionName = "ToolCallArgsEvent"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var timestamp: Int64? { (fields["timestamp"] as? NSNumber)?.int64Value }
    public var rawEvent: Any? { fields["rawEvent"] }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public var subagentRunId: AGUI1SubagentRunId? { AGUI1Nested.decode(fields["subagentRunId"], as: AGUI1SubagentRunId.self) }
    public var toolCallId: String? { fields["toolCallId"] as? String }
    public var delta: String? { fields["delta"] as? String }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingTimestamp(_ value: Int64?) throws -> Self {
        var updated = fields
        if let value { updated["timestamp"] = value } else { updated.removeValue(forKey: "timestamp") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRawEvent(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["rawEvent"] = value } else { updated.removeValue(forKey: "rawEvent") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingSubagentRunId(_ value: AGUI1SubagentRunId?) throws -> Self {
        var updated = fields
        if let value { updated["subagentRunId"] = value.document.value } else { updated.removeValue(forKey: "subagentRunId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingToolCallId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["toolCallId"] = value } else { updated.removeValue(forKey: "toolCallId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingDelta(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["delta"] = value } else { updated.removeValue(forKey: "delta") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(toolCallId: String, delta: String) throws {
        let values: [String: Any] = [
            "type": "TOOL_CALL_ARGS",
            "toolCallId": toolCallId,
            "delta": delta,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1ToolCallEndEvent: AGUI1Definition {
    public static let definitionName = "ToolCallEndEvent"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var timestamp: Int64? { (fields["timestamp"] as? NSNumber)?.int64Value }
    public var rawEvent: Any? { fields["rawEvent"] }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public var subagentRunId: AGUI1SubagentRunId? { AGUI1Nested.decode(fields["subagentRunId"], as: AGUI1SubagentRunId.self) }
    public var toolCallId: String? { fields["toolCallId"] as? String }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingTimestamp(_ value: Int64?) throws -> Self {
        var updated = fields
        if let value { updated["timestamp"] = value } else { updated.removeValue(forKey: "timestamp") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRawEvent(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["rawEvent"] = value } else { updated.removeValue(forKey: "rawEvent") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingSubagentRunId(_ value: AGUI1SubagentRunId?) throws -> Self {
        var updated = fields
        if let value { updated["subagentRunId"] = value.document.value } else { updated.removeValue(forKey: "subagentRunId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingToolCallId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["toolCallId"] = value } else { updated.removeValue(forKey: "toolCallId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(toolCallId: String) throws {
        let values: [String: Any] = [
            "type": "TOOL_CALL_END",
            "toolCallId": toolCallId,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1ToolCallChunkEvent: AGUI1Definition {
    public static let definitionName = "ToolCallChunkEvent"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var timestamp: Int64? { (fields["timestamp"] as? NSNumber)?.int64Value }
    public var rawEvent: Any? { fields["rawEvent"] }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public var subagentRunId: AGUI1SubagentRunId? { AGUI1Nested.decode(fields["subagentRunId"], as: AGUI1SubagentRunId.self) }
    public var toolCallId: String? { fields["toolCallId"] as? String }
    public var toolCallName: String? { fields["toolCallName"] as? String }
    public var parentMessageId: String? { fields["parentMessageId"] as? String }
    public var delta: String? { fields["delta"] as? String }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingTimestamp(_ value: Int64?) throws -> Self {
        var updated = fields
        if let value { updated["timestamp"] = value } else { updated.removeValue(forKey: "timestamp") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRawEvent(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["rawEvent"] = value } else { updated.removeValue(forKey: "rawEvent") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingSubagentRunId(_ value: AGUI1SubagentRunId?) throws -> Self {
        var updated = fields
        if let value { updated["subagentRunId"] = value.document.value } else { updated.removeValue(forKey: "subagentRunId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingToolCallId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["toolCallId"] = value } else { updated.removeValue(forKey: "toolCallId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingToolCallName(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["toolCallName"] = value } else { updated.removeValue(forKey: "toolCallName") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingParentMessageId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["parentMessageId"] = value } else { updated.removeValue(forKey: "parentMessageId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingDelta(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["delta"] = value } else { updated.removeValue(forKey: "delta") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init() throws {
        let values: [String: Any] = [
            "type": "TOOL_CALL_CHUNK",
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1ToolCallResultEvent: AGUI1Definition {
    public static let definitionName = "ToolCallResultEvent"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var timestamp: Int64? { (fields["timestamp"] as? NSNumber)?.int64Value }
    public var rawEvent: Any? { fields["rawEvent"] }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public var subagentRunId: AGUI1SubagentRunId? { AGUI1Nested.decode(fields["subagentRunId"], as: AGUI1SubagentRunId.self) }
    public var messageId: String? { fields["messageId"] as? String }
    public var toolCallId: String? { fields["toolCallId"] as? String }
    public var content: Any? { fields["content"] }
    public var role: String? { fields["role"] as? String }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingTimestamp(_ value: Int64?) throws -> Self {
        var updated = fields
        if let value { updated["timestamp"] = value } else { updated.removeValue(forKey: "timestamp") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRawEvent(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["rawEvent"] = value } else { updated.removeValue(forKey: "rawEvent") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingSubagentRunId(_ value: AGUI1SubagentRunId?) throws -> Self {
        var updated = fields
        if let value { updated["subagentRunId"] = value.document.value } else { updated.removeValue(forKey: "subagentRunId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMessageId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["messageId"] = value } else { updated.removeValue(forKey: "messageId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingToolCallId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["toolCallId"] = value } else { updated.removeValue(forKey: "toolCallId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingContent(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["content"] = value } else { updated.removeValue(forKey: "content") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRole(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["role"] = value } else { updated.removeValue(forKey: "role") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(messageId: String, toolCallId: String, content: Any) throws {
        let values: [String: Any] = [
            "type": "TOOL_CALL_RESULT",
            "role": "tool",
            "messageId": messageId,
            "toolCallId": toolCallId,
            "content": content,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1StateSnapshotEvent: AGUI1Definition {
    public static let definitionName = "StateSnapshotEvent"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var timestamp: Int64? { (fields["timestamp"] as? NSNumber)?.int64Value }
    public var rawEvent: Any? { fields["rawEvent"] }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public var subagentRunId: AGUI1SubagentRunId? { AGUI1Nested.decode(fields["subagentRunId"], as: AGUI1SubagentRunId.self) }
    public var snapshot: AGUI1State? { AGUI1Nested.decode(fields["snapshot"], as: AGUI1State.self) }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingTimestamp(_ value: Int64?) throws -> Self {
        var updated = fields
        if let value { updated["timestamp"] = value } else { updated.removeValue(forKey: "timestamp") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRawEvent(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["rawEvent"] = value } else { updated.removeValue(forKey: "rawEvent") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingSubagentRunId(_ value: AGUI1SubagentRunId?) throws -> Self {
        var updated = fields
        if let value { updated["subagentRunId"] = value.document.value } else { updated.removeValue(forKey: "subagentRunId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingSnapshot(_ value: AGUI1State?) throws -> Self {
        var updated = fields
        if let value { updated["snapshot"] = value.document.value } else { updated.removeValue(forKey: "snapshot") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(snapshot: AGUI1State) throws {
        let values: [String: Any] = [
            "type": "STATE_SNAPSHOT",
            "snapshot": snapshot.document.value,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1StateDeltaEvent: AGUI1Definition {
    public static let definitionName = "StateDeltaEvent"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var timestamp: Int64? { (fields["timestamp"] as? NSNumber)?.int64Value }
    public var rawEvent: Any? { fields["rawEvent"] }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public var subagentRunId: AGUI1SubagentRunId? { AGUI1Nested.decode(fields["subagentRunId"], as: AGUI1SubagentRunId.self) }
    public var delta: AGUI1JsonPatch? { AGUI1Nested.decode(fields["delta"], as: AGUI1JsonPatch.self) }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingTimestamp(_ value: Int64?) throws -> Self {
        var updated = fields
        if let value { updated["timestamp"] = value } else { updated.removeValue(forKey: "timestamp") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRawEvent(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["rawEvent"] = value } else { updated.removeValue(forKey: "rawEvent") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingSubagentRunId(_ value: AGUI1SubagentRunId?) throws -> Self {
        var updated = fields
        if let value { updated["subagentRunId"] = value.document.value } else { updated.removeValue(forKey: "subagentRunId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingDelta(_ value: AGUI1JsonPatch?) throws -> Self {
        var updated = fields
        if let value { updated["delta"] = value.document.value } else { updated.removeValue(forKey: "delta") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(delta: AGUI1JsonPatch) throws {
        let values: [String: Any] = [
            "type": "STATE_DELTA",
            "delta": delta.document.value,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1MessagesSnapshotEvent: AGUI1Definition {
    public static let definitionName = "MessagesSnapshotEvent"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var timestamp: Int64? { (fields["timestamp"] as? NSNumber)?.int64Value }
    public var rawEvent: Any? { fields["rawEvent"] }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public var messages: [AGUI1Message]? { (fields["messages"] as? [Any])?.compactMap { AGUI1Nested.decode($0, as: AGUI1Message.self) } }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingTimestamp(_ value: Int64?) throws -> Self {
        var updated = fields
        if let value { updated["timestamp"] = value } else { updated.removeValue(forKey: "timestamp") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRawEvent(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["rawEvent"] = value } else { updated.removeValue(forKey: "rawEvent") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMessages(_ value: [AGUI1Message]?) throws -> Self {
        var updated = fields
        if let value { updated["messages"] = value.map { $0.document.value } } else { updated.removeValue(forKey: "messages") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(messages: [AGUI1Message]) throws {
        let values: [String: Any] = [
            "type": "MESSAGES_SNAPSHOT",
            "messages": messages.map { $0.document.value },
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1ActivitySnapshotEvent: AGUI1Definition {
    public static let definitionName = "ActivitySnapshotEvent"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var timestamp: Int64? { (fields["timestamp"] as? NSNumber)?.int64Value }
    public var rawEvent: Any? { fields["rawEvent"] }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public var subagentRunId: AGUI1SubagentRunId? { AGUI1Nested.decode(fields["subagentRunId"], as: AGUI1SubagentRunId.self) }
    public var messageId: String? { fields["messageId"] as? String }
    public var activityType: String? { fields["activityType"] as? String }
    public var content: [String: Any]? { fields["content"] as? [String: Any] }
    public var replace: Bool? { fields["replace"] as? Bool }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingTimestamp(_ value: Int64?) throws -> Self {
        var updated = fields
        if let value { updated["timestamp"] = value } else { updated.removeValue(forKey: "timestamp") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRawEvent(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["rawEvent"] = value } else { updated.removeValue(forKey: "rawEvent") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingSubagentRunId(_ value: AGUI1SubagentRunId?) throws -> Self {
        var updated = fields
        if let value { updated["subagentRunId"] = value.document.value } else { updated.removeValue(forKey: "subagentRunId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMessageId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["messageId"] = value } else { updated.removeValue(forKey: "messageId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingActivityType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["activityType"] = value } else { updated.removeValue(forKey: "activityType") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingContent(_ value: [String: Any]?) throws -> Self {
        var updated = fields
        if let value { updated["content"] = value } else { updated.removeValue(forKey: "content") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingReplace(_ value: Bool?) throws -> Self {
        var updated = fields
        if let value { updated["replace"] = value } else { updated.removeValue(forKey: "replace") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(messageId: String, activityType: String, content: [String: Any]) throws {
        let values: [String: Any] = [
            "type": "ACTIVITY_SNAPSHOT",
            "messageId": messageId,
            "activityType": activityType,
            "content": content,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1ActivityDeltaEvent: AGUI1Definition {
    public static let definitionName = "ActivityDeltaEvent"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var timestamp: Int64? { (fields["timestamp"] as? NSNumber)?.int64Value }
    public var rawEvent: Any? { fields["rawEvent"] }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public var subagentRunId: AGUI1SubagentRunId? { AGUI1Nested.decode(fields["subagentRunId"], as: AGUI1SubagentRunId.self) }
    public var messageId: String? { fields["messageId"] as? String }
    public var activityType: String? { fields["activityType"] as? String }
    public var patch: AGUI1JsonPatch? { AGUI1Nested.decode(fields["patch"], as: AGUI1JsonPatch.self) }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingTimestamp(_ value: Int64?) throws -> Self {
        var updated = fields
        if let value { updated["timestamp"] = value } else { updated.removeValue(forKey: "timestamp") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRawEvent(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["rawEvent"] = value } else { updated.removeValue(forKey: "rawEvent") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingSubagentRunId(_ value: AGUI1SubagentRunId?) throws -> Self {
        var updated = fields
        if let value { updated["subagentRunId"] = value.document.value } else { updated.removeValue(forKey: "subagentRunId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMessageId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["messageId"] = value } else { updated.removeValue(forKey: "messageId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingActivityType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["activityType"] = value } else { updated.removeValue(forKey: "activityType") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingPatch(_ value: AGUI1JsonPatch?) throws -> Self {
        var updated = fields
        if let value { updated["patch"] = value.document.value } else { updated.removeValue(forKey: "patch") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(messageId: String, activityType: String, patch: AGUI1JsonPatch) throws {
        let values: [String: Any] = [
            "type": "ACTIVITY_DELTA",
            "messageId": messageId,
            "activityType": activityType,
            "patch": patch.document.value,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1RawEvent: AGUI1Definition {
    public static let definitionName = "RawEvent"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var timestamp: Int64? { (fields["timestamp"] as? NSNumber)?.int64Value }
    public var rawEvent: Any? { fields["rawEvent"] }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public var subagentRunId: AGUI1SubagentRunId? { AGUI1Nested.decode(fields["subagentRunId"], as: AGUI1SubagentRunId.self) }
    public var event: Any? { fields["event"] }
    public var source: String? { fields["source"] as? String }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingTimestamp(_ value: Int64?) throws -> Self {
        var updated = fields
        if let value { updated["timestamp"] = value } else { updated.removeValue(forKey: "timestamp") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRawEvent(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["rawEvent"] = value } else { updated.removeValue(forKey: "rawEvent") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingSubagentRunId(_ value: AGUI1SubagentRunId?) throws -> Self {
        var updated = fields
        if let value { updated["subagentRunId"] = value.document.value } else { updated.removeValue(forKey: "subagentRunId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingEvent(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["event"] = value } else { updated.removeValue(forKey: "event") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingSource(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["source"] = value } else { updated.removeValue(forKey: "source") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(event: Any) throws {
        let values: [String: Any] = [
            "type": "RAW",
            "event": event,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1CustomEvent: AGUI1Definition {
    public static let definitionName = "CustomEvent"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var timestamp: Int64? { (fields["timestamp"] as? NSNumber)?.int64Value }
    public var rawEvent: Any? { fields["rawEvent"] }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public var subagentRunId: AGUI1SubagentRunId? { AGUI1Nested.decode(fields["subagentRunId"], as: AGUI1SubagentRunId.self) }
    public var name: String? { fields["name"] as? String }
    public var value: Any? { fields["value"] }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingTimestamp(_ value: Int64?) throws -> Self {
        var updated = fields
        if let value { updated["timestamp"] = value } else { updated.removeValue(forKey: "timestamp") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRawEvent(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["rawEvent"] = value } else { updated.removeValue(forKey: "rawEvent") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingSubagentRunId(_ value: AGUI1SubagentRunId?) throws -> Self {
        var updated = fields
        if let value { updated["subagentRunId"] = value.document.value } else { updated.removeValue(forKey: "subagentRunId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingName(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["name"] = value } else { updated.removeValue(forKey: "name") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingValue(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["value"] = value } else { updated.removeValue(forKey: "value") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(name: String, value: Any) throws {
        let values: [String: Any] = [
            "type": "CUSTOM",
            "name": name,
            "value": value,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1RunStartedEvent: AGUI1Definition {
    public static let definitionName = "RunStartedEvent"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var timestamp: Int64? { (fields["timestamp"] as? NSNumber)?.int64Value }
    public var rawEvent: Any? { fields["rawEvent"] }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public var threadId: String? { fields["threadId"] as? String }
    public var runId: String? { fields["runId"] as? String }
    public var protocolVersion: String? { fields["protocolVersion"] as? String }
    public var parentRunId: String? { fields["parentRunId"] as? String }
    public var input: AGUI1RunAgentInput? { AGUI1Nested.decode(fields["input"], as: AGUI1RunAgentInput.self) }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingTimestamp(_ value: Int64?) throws -> Self {
        var updated = fields
        if let value { updated["timestamp"] = value } else { updated.removeValue(forKey: "timestamp") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRawEvent(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["rawEvent"] = value } else { updated.removeValue(forKey: "rawEvent") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingThreadId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["threadId"] = value } else { updated.removeValue(forKey: "threadId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRunId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["runId"] = value } else { updated.removeValue(forKey: "runId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingProtocolVersion(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["protocolVersion"] = value } else { updated.removeValue(forKey: "protocolVersion") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingParentRunId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["parentRunId"] = value } else { updated.removeValue(forKey: "parentRunId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingInput(_ value: AGUI1RunAgentInput?) throws -> Self {
        var updated = fields
        if let value { updated["input"] = value.document.value } else { updated.removeValue(forKey: "input") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(threadId: String, runId: String) throws {
        let values: [String: Any] = [
            "type": "RUN_STARTED",
            "threadId": threadId,
            "runId": runId,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1RunFinishedEvent: AGUI1Definition {
    public static let definitionName = "RunFinishedEvent"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var timestamp: Int64? { (fields["timestamp"] as? NSNumber)?.int64Value }
    public var rawEvent: Any? { fields["rawEvent"] }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public var threadId: String? { fields["threadId"] as? String }
    public var runId: String? { fields["runId"] as? String }
    public var result: Any? { fields["result"] }
    public var outcome: AGUI1RunFinishedOutcome? { AGUI1Nested.decode(fields["outcome"], as: AGUI1RunFinishedOutcome.self) }
    public var usage: [AGUI1TokenUsage]? { (fields["usage"] as? [Any])?.compactMap { AGUI1Nested.decode($0, as: AGUI1TokenUsage.self) } }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingTimestamp(_ value: Int64?) throws -> Self {
        var updated = fields
        if let value { updated["timestamp"] = value } else { updated.removeValue(forKey: "timestamp") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRawEvent(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["rawEvent"] = value } else { updated.removeValue(forKey: "rawEvent") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingThreadId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["threadId"] = value } else { updated.removeValue(forKey: "threadId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRunId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["runId"] = value } else { updated.removeValue(forKey: "runId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingResult(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["result"] = value } else { updated.removeValue(forKey: "result") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingOutcome(_ value: AGUI1RunFinishedOutcome?) throws -> Self {
        var updated = fields
        if let value { updated["outcome"] = value.document.value } else { updated.removeValue(forKey: "outcome") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingUsage(_ value: [AGUI1TokenUsage]?) throws -> Self {
        var updated = fields
        if let value { updated["usage"] = value.map { $0.document.value } } else { updated.removeValue(forKey: "usage") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(threadId: String, runId: String) throws {
        let values: [String: Any] = [
            "type": "RUN_FINISHED",
            "threadId": threadId,
            "runId": runId,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1RunErrorEvent: AGUI1Definition {
    public static let definitionName = "RunErrorEvent"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var timestamp: Int64? { (fields["timestamp"] as? NSNumber)?.int64Value }
    public var rawEvent: Any? { fields["rawEvent"] }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public var message: String? { fields["message"] as? String }
    public var code: String? { fields["code"] as? String }
    public var usage: [AGUI1TokenUsage]? { (fields["usage"] as? [Any])?.compactMap { AGUI1Nested.decode($0, as: AGUI1TokenUsage.self) } }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingTimestamp(_ value: Int64?) throws -> Self {
        var updated = fields
        if let value { updated["timestamp"] = value } else { updated.removeValue(forKey: "timestamp") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRawEvent(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["rawEvent"] = value } else { updated.removeValue(forKey: "rawEvent") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMessage(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["message"] = value } else { updated.removeValue(forKey: "message") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingCode(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["code"] = value } else { updated.removeValue(forKey: "code") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingUsage(_ value: [AGUI1TokenUsage]?) throws -> Self {
        var updated = fields
        if let value { updated["usage"] = value.map { $0.document.value } } else { updated.removeValue(forKey: "usage") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(message: String) throws {
        let values: [String: Any] = [
            "type": "RUN_ERROR",
            "message": message,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1StepStartedEvent: AGUI1Definition {
    public static let definitionName = "StepStartedEvent"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var timestamp: Int64? { (fields["timestamp"] as? NSNumber)?.int64Value }
    public var rawEvent: Any? { fields["rawEvent"] }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public var subagentRunId: AGUI1SubagentRunId? { AGUI1Nested.decode(fields["subagentRunId"], as: AGUI1SubagentRunId.self) }
    public var stepName: String? { fields["stepName"] as? String }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingTimestamp(_ value: Int64?) throws -> Self {
        var updated = fields
        if let value { updated["timestamp"] = value } else { updated.removeValue(forKey: "timestamp") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRawEvent(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["rawEvent"] = value } else { updated.removeValue(forKey: "rawEvent") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingSubagentRunId(_ value: AGUI1SubagentRunId?) throws -> Self {
        var updated = fields
        if let value { updated["subagentRunId"] = value.document.value } else { updated.removeValue(forKey: "subagentRunId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingStepName(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["stepName"] = value } else { updated.removeValue(forKey: "stepName") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(stepName: String) throws {
        let values: [String: Any] = [
            "type": "STEP_STARTED",
            "stepName": stepName,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1StepFinishedEvent: AGUI1Definition {
    public static let definitionName = "StepFinishedEvent"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var timestamp: Int64? { (fields["timestamp"] as? NSNumber)?.int64Value }
    public var rawEvent: Any? { fields["rawEvent"] }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public var subagentRunId: AGUI1SubagentRunId? { AGUI1Nested.decode(fields["subagentRunId"], as: AGUI1SubagentRunId.self) }
    public var stepName: String? { fields["stepName"] as? String }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingTimestamp(_ value: Int64?) throws -> Self {
        var updated = fields
        if let value { updated["timestamp"] = value } else { updated.removeValue(forKey: "timestamp") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRawEvent(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["rawEvent"] = value } else { updated.removeValue(forKey: "rawEvent") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingSubagentRunId(_ value: AGUI1SubagentRunId?) throws -> Self {
        var updated = fields
        if let value { updated["subagentRunId"] = value.document.value } else { updated.removeValue(forKey: "subagentRunId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingStepName(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["stepName"] = value } else { updated.removeValue(forKey: "stepName") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(stepName: String) throws {
        let values: [String: Any] = [
            "type": "STEP_FINISHED",
            "stepName": stepName,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1ReasoningStartEvent: AGUI1Definition {
    public static let definitionName = "ReasoningStartEvent"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var timestamp: Int64? { (fields["timestamp"] as? NSNumber)?.int64Value }
    public var rawEvent: Any? { fields["rawEvent"] }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public var subagentRunId: AGUI1SubagentRunId? { AGUI1Nested.decode(fields["subagentRunId"], as: AGUI1SubagentRunId.self) }
    public var messageId: String? { fields["messageId"] as? String }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingTimestamp(_ value: Int64?) throws -> Self {
        var updated = fields
        if let value { updated["timestamp"] = value } else { updated.removeValue(forKey: "timestamp") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRawEvent(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["rawEvent"] = value } else { updated.removeValue(forKey: "rawEvent") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingSubagentRunId(_ value: AGUI1SubagentRunId?) throws -> Self {
        var updated = fields
        if let value { updated["subagentRunId"] = value.document.value } else { updated.removeValue(forKey: "subagentRunId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMessageId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["messageId"] = value } else { updated.removeValue(forKey: "messageId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(messageId: String) throws {
        let values: [String: Any] = [
            "type": "REASONING_START",
            "messageId": messageId,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1ReasoningMessageStartEvent: AGUI1Definition {
    public static let definitionName = "ReasoningMessageStartEvent"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var timestamp: Int64? { (fields["timestamp"] as? NSNumber)?.int64Value }
    public var rawEvent: Any? { fields["rawEvent"] }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public var subagentRunId: AGUI1SubagentRunId? { AGUI1Nested.decode(fields["subagentRunId"], as: AGUI1SubagentRunId.self) }
    public var messageId: String? { fields["messageId"] as? String }
    public var role: String? { fields["role"] as? String }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingTimestamp(_ value: Int64?) throws -> Self {
        var updated = fields
        if let value { updated["timestamp"] = value } else { updated.removeValue(forKey: "timestamp") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRawEvent(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["rawEvent"] = value } else { updated.removeValue(forKey: "rawEvent") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingSubagentRunId(_ value: AGUI1SubagentRunId?) throws -> Self {
        var updated = fields
        if let value { updated["subagentRunId"] = value.document.value } else { updated.removeValue(forKey: "subagentRunId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMessageId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["messageId"] = value } else { updated.removeValue(forKey: "messageId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRole(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["role"] = value } else { updated.removeValue(forKey: "role") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(messageId: String) throws {
        let values: [String: Any] = [
            "type": "REASONING_MESSAGE_START",
            "role": "reasoning",
            "messageId": messageId,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1ReasoningMessageContentEvent: AGUI1Definition {
    public static let definitionName = "ReasoningMessageContentEvent"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var timestamp: Int64? { (fields["timestamp"] as? NSNumber)?.int64Value }
    public var rawEvent: Any? { fields["rawEvent"] }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public var subagentRunId: AGUI1SubagentRunId? { AGUI1Nested.decode(fields["subagentRunId"], as: AGUI1SubagentRunId.self) }
    public var messageId: String? { fields["messageId"] as? String }
    public var delta: String? { fields["delta"] as? String }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingTimestamp(_ value: Int64?) throws -> Self {
        var updated = fields
        if let value { updated["timestamp"] = value } else { updated.removeValue(forKey: "timestamp") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRawEvent(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["rawEvent"] = value } else { updated.removeValue(forKey: "rawEvent") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingSubagentRunId(_ value: AGUI1SubagentRunId?) throws -> Self {
        var updated = fields
        if let value { updated["subagentRunId"] = value.document.value } else { updated.removeValue(forKey: "subagentRunId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMessageId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["messageId"] = value } else { updated.removeValue(forKey: "messageId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingDelta(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["delta"] = value } else { updated.removeValue(forKey: "delta") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(messageId: String, delta: String) throws {
        let values: [String: Any] = [
            "type": "REASONING_MESSAGE_CONTENT",
            "messageId": messageId,
            "delta": delta,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1ReasoningMessageEndEvent: AGUI1Definition {
    public static let definitionName = "ReasoningMessageEndEvent"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var timestamp: Int64? { (fields["timestamp"] as? NSNumber)?.int64Value }
    public var rawEvent: Any? { fields["rawEvent"] }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public var subagentRunId: AGUI1SubagentRunId? { AGUI1Nested.decode(fields["subagentRunId"], as: AGUI1SubagentRunId.self) }
    public var messageId: String? { fields["messageId"] as? String }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingTimestamp(_ value: Int64?) throws -> Self {
        var updated = fields
        if let value { updated["timestamp"] = value } else { updated.removeValue(forKey: "timestamp") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRawEvent(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["rawEvent"] = value } else { updated.removeValue(forKey: "rawEvent") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingSubagentRunId(_ value: AGUI1SubagentRunId?) throws -> Self {
        var updated = fields
        if let value { updated["subagentRunId"] = value.document.value } else { updated.removeValue(forKey: "subagentRunId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMessageId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["messageId"] = value } else { updated.removeValue(forKey: "messageId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(messageId: String) throws {
        let values: [String: Any] = [
            "type": "REASONING_MESSAGE_END",
            "messageId": messageId,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1ReasoningMessageChunkEvent: AGUI1Definition {
    public static let definitionName = "ReasoningMessageChunkEvent"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var timestamp: Int64? { (fields["timestamp"] as? NSNumber)?.int64Value }
    public var rawEvent: Any? { fields["rawEvent"] }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public var subagentRunId: AGUI1SubagentRunId? { AGUI1Nested.decode(fields["subagentRunId"], as: AGUI1SubagentRunId.self) }
    public var messageId: String? { fields["messageId"] as? String }
    public var delta: String? { fields["delta"] as? String }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingTimestamp(_ value: Int64?) throws -> Self {
        var updated = fields
        if let value { updated["timestamp"] = value } else { updated.removeValue(forKey: "timestamp") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRawEvent(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["rawEvent"] = value } else { updated.removeValue(forKey: "rawEvent") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingSubagentRunId(_ value: AGUI1SubagentRunId?) throws -> Self {
        var updated = fields
        if let value { updated["subagentRunId"] = value.document.value } else { updated.removeValue(forKey: "subagentRunId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMessageId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["messageId"] = value } else { updated.removeValue(forKey: "messageId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingDelta(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["delta"] = value } else { updated.removeValue(forKey: "delta") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init() throws {
        let values: [String: Any] = [
            "type": "REASONING_MESSAGE_CHUNK",
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1ReasoningEndEvent: AGUI1Definition {
    public static let definitionName = "ReasoningEndEvent"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var timestamp: Int64? { (fields["timestamp"] as? NSNumber)?.int64Value }
    public var rawEvent: Any? { fields["rawEvent"] }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public var subagentRunId: AGUI1SubagentRunId? { AGUI1Nested.decode(fields["subagentRunId"], as: AGUI1SubagentRunId.self) }
    public var messageId: String? { fields["messageId"] as? String }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingTimestamp(_ value: Int64?) throws -> Self {
        var updated = fields
        if let value { updated["timestamp"] = value } else { updated.removeValue(forKey: "timestamp") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRawEvent(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["rawEvent"] = value } else { updated.removeValue(forKey: "rawEvent") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingSubagentRunId(_ value: AGUI1SubagentRunId?) throws -> Self {
        var updated = fields
        if let value { updated["subagentRunId"] = value.document.value } else { updated.removeValue(forKey: "subagentRunId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMessageId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["messageId"] = value } else { updated.removeValue(forKey: "messageId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(messageId: String) throws {
        let values: [String: Any] = [
            "type": "REASONING_END",
            "messageId": messageId,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1ReasoningEncryptedValueEvent: AGUI1Definition {
    public static let definitionName = "ReasoningEncryptedValueEvent"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var timestamp: Int64? { (fields["timestamp"] as? NSNumber)?.int64Value }
    public var rawEvent: Any? { fields["rawEvent"] }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public var subagentRunId: AGUI1SubagentRunId? { AGUI1Nested.decode(fields["subagentRunId"], as: AGUI1SubagentRunId.self) }
    public var subtype: AGUI1ReasoningEncryptedValueSubtypeValue? { AGUI1ReasoningEncryptedValueSubtypeValue(rawValue: fields["subtype"] as? String ?? "") }
    public var entityId: String? { fields["entityId"] as? String }
    public var encryptedValue: String? { fields["encryptedValue"] as? String }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingTimestamp(_ value: Int64?) throws -> Self {
        var updated = fields
        if let value { updated["timestamp"] = value } else { updated.removeValue(forKey: "timestamp") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRawEvent(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["rawEvent"] = value } else { updated.removeValue(forKey: "rawEvent") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingSubagentRunId(_ value: AGUI1SubagentRunId?) throws -> Self {
        var updated = fields
        if let value { updated["subagentRunId"] = value.document.value } else { updated.removeValue(forKey: "subagentRunId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingSubtype(_ value: AGUI1ReasoningEncryptedValueSubtypeValue?) throws -> Self {
        var updated = fields
        if let value { updated["subtype"] = value.rawValue } else { updated.removeValue(forKey: "subtype") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingEntityId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["entityId"] = value } else { updated.removeValue(forKey: "entityId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingEncryptedValue(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["encryptedValue"] = value } else { updated.removeValue(forKey: "encryptedValue") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(subtype: AGUI1ReasoningEncryptedValueSubtypeValue, entityId: String, encryptedValue: String) throws {
        let values: [String: Any] = [
            "type": "REASONING_ENCRYPTED_VALUE",
            "subtype": subtype.rawValue,
            "entityId": entityId,
            "encryptedValue": encryptedValue,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1ReasoningEncryptedValueSubtype: AGUI1Definition {
    public static let definitionName = "ReasoningEncryptedValueSubtype"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var value: AGUI1ReasoningEncryptedValueSubtypeValue? { AGUI1ReasoningEncryptedValueSubtypeValue(rawValue: document.value as? String ?? "") }
}

public struct AGUI1SubagentStartedEvent: AGUI1Definition {
    public static let definitionName = "SubagentStartedEvent"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var timestamp: Int64? { (fields["timestamp"] as? NSNumber)?.int64Value }
    public var rawEvent: Any? { fields["rawEvent"] }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public var subagentRunId: AGUI1SubagentRunId? { AGUI1Nested.decode(fields["subagentRunId"], as: AGUI1SubagentRunId.self) }
    public var name: String? { fields["name"] as? String }
    public var description: String? { fields["description"] as? String }
    public var parentSubagentRunId: AGUI1SubagentRunId? { AGUI1Nested.decode(fields["parentSubagentRunId"], as: AGUI1SubagentRunId.self) }
    public var parentToolCallId: String? { fields["parentToolCallId"] as? String }
    public var parentMessageId: String? { fields["parentMessageId"] as? String }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingTimestamp(_ value: Int64?) throws -> Self {
        var updated = fields
        if let value { updated["timestamp"] = value } else { updated.removeValue(forKey: "timestamp") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRawEvent(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["rawEvent"] = value } else { updated.removeValue(forKey: "rawEvent") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingSubagentRunId(_ value: AGUI1SubagentRunId?) throws -> Self {
        var updated = fields
        if let value { updated["subagentRunId"] = value.document.value } else { updated.removeValue(forKey: "subagentRunId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingName(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["name"] = value } else { updated.removeValue(forKey: "name") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingDescription(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["description"] = value } else { updated.removeValue(forKey: "description") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingParentSubagentRunId(_ value: AGUI1SubagentRunId?) throws -> Self {
        var updated = fields
        if let value { updated["parentSubagentRunId"] = value.document.value } else { updated.removeValue(forKey: "parentSubagentRunId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingParentToolCallId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["parentToolCallId"] = value } else { updated.removeValue(forKey: "parentToolCallId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingParentMessageId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["parentMessageId"] = value } else { updated.removeValue(forKey: "parentMessageId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(subagentRunId: AGUI1SubagentRunId, name: String) throws {
        let values: [String: Any] = [
            "type": "SUBAGENT_STARTED",
            "subagentRunId": subagentRunId.document.value,
            "name": name,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1SubagentFinishedEvent: AGUI1Definition {
    public static let definitionName = "SubagentFinishedEvent"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var timestamp: Int64? { (fields["timestamp"] as? NSNumber)?.int64Value }
    public var rawEvent: Any? { fields["rawEvent"] }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public var subagentRunId: AGUI1SubagentRunId? { AGUI1Nested.decode(fields["subagentRunId"], as: AGUI1SubagentRunId.self) }
    public var result: Any? { fields["result"] }
    public var outcome: AGUI1SubagentFinishedOutcome? { AGUI1Nested.decode(fields["outcome"], as: AGUI1SubagentFinishedOutcome.self) }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingTimestamp(_ value: Int64?) throws -> Self {
        var updated = fields
        if let value { updated["timestamp"] = value } else { updated.removeValue(forKey: "timestamp") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRawEvent(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["rawEvent"] = value } else { updated.removeValue(forKey: "rawEvent") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingSubagentRunId(_ value: AGUI1SubagentRunId?) throws -> Self {
        var updated = fields
        if let value { updated["subagentRunId"] = value.document.value } else { updated.removeValue(forKey: "subagentRunId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingResult(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["result"] = value } else { updated.removeValue(forKey: "result") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingOutcome(_ value: AGUI1SubagentFinishedOutcome?) throws -> Self {
        var updated = fields
        if let value { updated["outcome"] = value.document.value } else { updated.removeValue(forKey: "outcome") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(subagentRunId: AGUI1SubagentRunId) throws {
        let values: [String: Any] = [
            "type": "SUBAGENT_FINISHED",
            "subagentRunId": subagentRunId.document.value,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1SubagentErrorEvent: AGUI1Definition {
    public static let definitionName = "SubagentErrorEvent"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var timestamp: Int64? { (fields["timestamp"] as? NSNumber)?.int64Value }
    public var rawEvent: Any? { fields["rawEvent"] }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public var subagentRunId: AGUI1SubagentRunId? { AGUI1Nested.decode(fields["subagentRunId"], as: AGUI1SubagentRunId.self) }
    public var message: String? { fields["message"] as? String }
    public var code: String? { fields["code"] as? String }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingTimestamp(_ value: Int64?) throws -> Self {
        var updated = fields
        if let value { updated["timestamp"] = value } else { updated.removeValue(forKey: "timestamp") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRawEvent(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["rawEvent"] = value } else { updated.removeValue(forKey: "rawEvent") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingSubagentRunId(_ value: AGUI1SubagentRunId?) throws -> Self {
        var updated = fields
        if let value { updated["subagentRunId"] = value.document.value } else { updated.removeValue(forKey: "subagentRunId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMessage(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["message"] = value } else { updated.removeValue(forKey: "message") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingCode(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["code"] = value } else { updated.removeValue(forKey: "code") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(subagentRunId: AGUI1SubagentRunId, message: String) throws {
        let values: [String: Any] = [
            "type": "SUBAGENT_ERROR",
            "subagentRunId": subagentRunId.document.value,
            "message": message,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1RunFinishedOutcome: AGUI1Definition {
    public static let definitionName = "RunFinishedOutcome"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var asRunFinishedSuccessOutcome: AGUI1RunFinishedSuccessOutcome? { AGUI1Nested.decode(document.value, as: AGUI1RunFinishedSuccessOutcome.self) }
    public var asRunFinishedInterruptOutcome: AGUI1RunFinishedInterruptOutcome? { AGUI1Nested.decode(document.value, as: AGUI1RunFinishedInterruptOutcome.self) }
    public var asRunFinishedCancelledOutcome: AGUI1RunFinishedCancelledOutcome? { AGUI1Nested.decode(document.value, as: AGUI1RunFinishedCancelledOutcome.self) }
}

public struct AGUI1RunFinishedSuccessOutcome: AGUI1Definition {
    public static let definitionName = "RunFinishedSuccessOutcome"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var pendingToolCallIds: [String]? { fields["pendingToolCallIds"] as? [String] }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingPendingToolCallIds(_ value: [String]?) throws -> Self {
        var updated = fields
        if let value { updated["pendingToolCallIds"] = value } else { updated.removeValue(forKey: "pendingToolCallIds") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init() throws {
        let values: [String: Any] = [
            "type": "success",
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1RunFinishedInterruptOutcome: AGUI1Definition {
    public static let definitionName = "RunFinishedInterruptOutcome"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var interrupts: [AGUI1Interrupt]? { (fields["interrupts"] as? [Any])?.compactMap { AGUI1Nested.decode($0, as: AGUI1Interrupt.self) } }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingInterrupts(_ value: [AGUI1Interrupt]?) throws -> Self {
        var updated = fields
        if let value { updated["interrupts"] = value.map { $0.document.value } } else { updated.removeValue(forKey: "interrupts") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(interrupts: [AGUI1Interrupt]) throws {
        let values: [String: Any] = [
            "type": "interrupt",
            "interrupts": interrupts.map { $0.document.value },
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1RunFinishedCancelledOutcome: AGUI1Definition {
    public static let definitionName = "RunFinishedCancelledOutcome"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init() throws {
        let values: [String: Any] = [
            "type": "cancelled",
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1SubagentFinishedOutcome: AGUI1Definition {
    public static let definitionName = "SubagentFinishedOutcome"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var asSubagentFinishedSuccessOutcome: AGUI1SubagentFinishedSuccessOutcome? { AGUI1Nested.decode(document.value, as: AGUI1SubagentFinishedSuccessOutcome.self) }
    public var asSubagentFinishedSuspendedOutcome: AGUI1SubagentFinishedSuspendedOutcome? { AGUI1Nested.decode(document.value, as: AGUI1SubagentFinishedSuspendedOutcome.self) }
}

public struct AGUI1SubagentFinishedSuccessOutcome: AGUI1Definition {
    public static let definitionName = "SubagentFinishedSuccessOutcome"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init() throws {
        let values: [String: Any] = [
            "type": "success",
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1SubagentFinishedSuspendedOutcome: AGUI1Definition {
    public static let definitionName = "SubagentFinishedSuspendedOutcome"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var interruptIds: [String]? { fields["interruptIds"] as? [String] }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingInterruptIds(_ value: [String]?) throws -> Self {
        var updated = fields
        if let value { updated["interruptIds"] = value } else { updated.removeValue(forKey: "interruptIds") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init() throws {
        let values: [String: Any] = [
            "type": "suspended",
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1Interrupt: AGUI1Definition {
    public static let definitionName = "Interrupt"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var subagentRunId: AGUI1SubagentRunId? { AGUI1Nested.decode(fields["subagentRunId"], as: AGUI1SubagentRunId.self) }
    public var id: String? { fields["id"] as? String }
    public var reason: String? { fields["reason"] as? String }
    public var message: String? { fields["message"] as? String }
    public var toolCallId: String? { fields["toolCallId"] as? String }
    public var responseSchema: [String: Any]? { fields["responseSchema"] as? [String: Any] }
    public var expiresAt: String? { fields["expiresAt"] as? String }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public func settingSubagentRunId(_ value: AGUI1SubagentRunId?) throws -> Self {
        var updated = fields
        if let value { updated["subagentRunId"] = value.document.value } else { updated.removeValue(forKey: "subagentRunId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["id"] = value } else { updated.removeValue(forKey: "id") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingReason(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["reason"] = value } else { updated.removeValue(forKey: "reason") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMessage(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["message"] = value } else { updated.removeValue(forKey: "message") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingToolCallId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["toolCallId"] = value } else { updated.removeValue(forKey: "toolCallId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingResponseSchema(_ value: [String: Any]?) throws -> Self {
        var updated = fields
        if let value { updated["responseSchema"] = value } else { updated.removeValue(forKey: "responseSchema") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingExpiresAt(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["expiresAt"] = value } else { updated.removeValue(forKey: "expiresAt") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(id: String, reason: String) throws {
        let values: [String: Any] = [
            "id": id,
            "reason": reason,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1ResumeEntry: AGUI1Definition {
    public static let definitionName = "ResumeEntry"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var interruptId: String? { fields["interruptId"] as? String }
    public var status: String? { fields["status"] as? String }
    public var payload: Any? { fields["payload"] }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public func settingInterruptId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["interruptId"] = value } else { updated.removeValue(forKey: "interruptId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingStatus(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["status"] = value } else { updated.removeValue(forKey: "status") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingPayload(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["payload"] = value } else { updated.removeValue(forKey: "payload") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(interruptId: String, status: String) throws {
        let values: [String: Any] = [
            "interruptId": interruptId,
            "status": status,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1TokenUsage: AGUI1Definition {
    public static let definitionName = "TokenUsage"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var provider: String? { fields["provider"] as? String }
    public var model: String? { fields["model"] as? String }
    public var inputTokens: Int64? { (fields["inputTokens"] as? NSNumber)?.int64Value }
    public var outputTokens: Int64? { (fields["outputTokens"] as? NSNumber)?.int64Value }
    public var totalTokens: Int64? { (fields["totalTokens"] as? NSNumber)?.int64Value }
    public var reasoningTokens: Int64? { (fields["reasoningTokens"] as? NSNumber)?.int64Value }
    public var cachedInputTokens: Int64? { (fields["cachedInputTokens"] as? NSNumber)?.int64Value }
    public var cacheWriteInputTokens: Int64? { (fields["cacheWriteInputTokens"] as? NSNumber)?.int64Value }
    public func settingProvider(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["provider"] = value } else { updated.removeValue(forKey: "provider") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingModel(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["model"] = value } else { updated.removeValue(forKey: "model") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingInputTokens(_ value: Int64?) throws -> Self {
        var updated = fields
        if let value { updated["inputTokens"] = value } else { updated.removeValue(forKey: "inputTokens") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingOutputTokens(_ value: Int64?) throws -> Self {
        var updated = fields
        if let value { updated["outputTokens"] = value } else { updated.removeValue(forKey: "outputTokens") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingTotalTokens(_ value: Int64?) throws -> Self {
        var updated = fields
        if let value { updated["totalTokens"] = value } else { updated.removeValue(forKey: "totalTokens") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingReasoningTokens(_ value: Int64?) throws -> Self {
        var updated = fields
        if let value { updated["reasoningTokens"] = value } else { updated.removeValue(forKey: "reasoningTokens") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingCachedInputTokens(_ value: Int64?) throws -> Self {
        var updated = fields
        if let value { updated["cachedInputTokens"] = value } else { updated.removeValue(forKey: "cachedInputTokens") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingCacheWriteInputTokens(_ value: Int64?) throws -> Self {
        var updated = fields
        if let value { updated["cacheWriteInputTokens"] = value } else { updated.removeValue(forKey: "cacheWriteInputTokens") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1Message: AGUI1Definition {
    public static let definitionName = "Message"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var asDeveloperMessage: AGUI1DeveloperMessage? { AGUI1Nested.decode(document.value, as: AGUI1DeveloperMessage.self) }
    public var asSystemMessage: AGUI1SystemMessage? { AGUI1Nested.decode(document.value, as: AGUI1SystemMessage.self) }
    public var asAssistantMessage: AGUI1AssistantMessage? { AGUI1Nested.decode(document.value, as: AGUI1AssistantMessage.self) }
    public var asUserMessage: AGUI1UserMessage? { AGUI1Nested.decode(document.value, as: AGUI1UserMessage.self) }
    public var asToolMessage: AGUI1ToolMessage? { AGUI1Nested.decode(document.value, as: AGUI1ToolMessage.self) }
    public var asActivityMessage: AGUI1ActivityMessage? { AGUI1Nested.decode(document.value, as: AGUI1ActivityMessage.self) }
    public var asReasoningMessage: AGUI1ReasoningMessage? { AGUI1Nested.decode(document.value, as: AGUI1ReasoningMessage.self) }
}

public struct AGUI1BaseMessage: AGUI1Definition {
    public static let definitionName = "BaseMessage"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var subagentRunId: AGUI1SubagentRunId? { AGUI1Nested.decode(fields["subagentRunId"], as: AGUI1SubagentRunId.self) }
    public var id: String? { fields["id"] as? String }
    public var role: String? { fields["role"] as? String }
    public var name: String? { fields["name"] as? String }
    public var encryptedValue: String? { fields["encryptedValue"] as? String }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public func settingSubagentRunId(_ value: AGUI1SubagentRunId?) throws -> Self {
        var updated = fields
        if let value { updated["subagentRunId"] = value.document.value } else { updated.removeValue(forKey: "subagentRunId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["id"] = value } else { updated.removeValue(forKey: "id") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRole(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["role"] = value } else { updated.removeValue(forKey: "role") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingName(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["name"] = value } else { updated.removeValue(forKey: "name") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingEncryptedValue(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["encryptedValue"] = value } else { updated.removeValue(forKey: "encryptedValue") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(id: String, role: String) throws {
        let values: [String: Any] = [
            "id": id,
            "role": role,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1DeveloperMessage: AGUI1Definition {
    public static let definitionName = "DeveloperMessage"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var subagentRunId: AGUI1SubagentRunId? { AGUI1Nested.decode(fields["subagentRunId"], as: AGUI1SubagentRunId.self) }
    public var id: String? { fields["id"] as? String }
    public var role: String? { fields["role"] as? String }
    public var name: String? { fields["name"] as? String }
    public var encryptedValue: String? { fields["encryptedValue"] as? String }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public var content: String? { fields["content"] as? String }
    public func settingSubagentRunId(_ value: AGUI1SubagentRunId?) throws -> Self {
        var updated = fields
        if let value { updated["subagentRunId"] = value.document.value } else { updated.removeValue(forKey: "subagentRunId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["id"] = value } else { updated.removeValue(forKey: "id") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRole(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["role"] = value } else { updated.removeValue(forKey: "role") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingName(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["name"] = value } else { updated.removeValue(forKey: "name") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingEncryptedValue(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["encryptedValue"] = value } else { updated.removeValue(forKey: "encryptedValue") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingContent(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["content"] = value } else { updated.removeValue(forKey: "content") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(id: String, content: String) throws {
        let values: [String: Any] = [
            "role": "developer",
            "id": id,
            "content": content,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1SystemMessage: AGUI1Definition {
    public static let definitionName = "SystemMessage"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var subagentRunId: AGUI1SubagentRunId? { AGUI1Nested.decode(fields["subagentRunId"], as: AGUI1SubagentRunId.self) }
    public var id: String? { fields["id"] as? String }
    public var role: String? { fields["role"] as? String }
    public var name: String? { fields["name"] as? String }
    public var encryptedValue: String? { fields["encryptedValue"] as? String }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public var content: String? { fields["content"] as? String }
    public func settingSubagentRunId(_ value: AGUI1SubagentRunId?) throws -> Self {
        var updated = fields
        if let value { updated["subagentRunId"] = value.document.value } else { updated.removeValue(forKey: "subagentRunId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["id"] = value } else { updated.removeValue(forKey: "id") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRole(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["role"] = value } else { updated.removeValue(forKey: "role") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingName(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["name"] = value } else { updated.removeValue(forKey: "name") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingEncryptedValue(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["encryptedValue"] = value } else { updated.removeValue(forKey: "encryptedValue") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingContent(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["content"] = value } else { updated.removeValue(forKey: "content") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(id: String, content: String) throws {
        let values: [String: Any] = [
            "role": "system",
            "id": id,
            "content": content,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1AssistantMessage: AGUI1Definition {
    public static let definitionName = "AssistantMessage"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var subagentRunId: AGUI1SubagentRunId? { AGUI1Nested.decode(fields["subagentRunId"], as: AGUI1SubagentRunId.self) }
    public var id: String? { fields["id"] as? String }
    public var role: String? { fields["role"] as? String }
    public var name: String? { fields["name"] as? String }
    public var encryptedValue: String? { fields["encryptedValue"] as? String }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public var content: String? { fields["content"] as? String }
    public var toolCalls: [AGUI1ToolCall]? { (fields["toolCalls"] as? [Any])?.compactMap { AGUI1Nested.decode($0, as: AGUI1ToolCall.self) } }
    public func settingSubagentRunId(_ value: AGUI1SubagentRunId?) throws -> Self {
        var updated = fields
        if let value { updated["subagentRunId"] = value.document.value } else { updated.removeValue(forKey: "subagentRunId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["id"] = value } else { updated.removeValue(forKey: "id") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRole(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["role"] = value } else { updated.removeValue(forKey: "role") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingName(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["name"] = value } else { updated.removeValue(forKey: "name") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingEncryptedValue(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["encryptedValue"] = value } else { updated.removeValue(forKey: "encryptedValue") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingContent(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["content"] = value } else { updated.removeValue(forKey: "content") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingToolCalls(_ value: [AGUI1ToolCall]?) throws -> Self {
        var updated = fields
        if let value { updated["toolCalls"] = value.map { $0.document.value } } else { updated.removeValue(forKey: "toolCalls") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(id: String) throws {
        let values: [String: Any] = [
            "role": "assistant",
            "id": id,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1UserMessage: AGUI1Definition {
    public static let definitionName = "UserMessage"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var subagentRunId: AGUI1SubagentRunId? { AGUI1Nested.decode(fields["subagentRunId"], as: AGUI1SubagentRunId.self) }
    public var id: String? { fields["id"] as? String }
    public var role: String? { fields["role"] as? String }
    public var name: String? { fields["name"] as? String }
    public var encryptedValue: String? { fields["encryptedValue"] as? String }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public var content: Any? { fields["content"] }
    public func settingSubagentRunId(_ value: AGUI1SubagentRunId?) throws -> Self {
        var updated = fields
        if let value { updated["subagentRunId"] = value.document.value } else { updated.removeValue(forKey: "subagentRunId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["id"] = value } else { updated.removeValue(forKey: "id") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRole(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["role"] = value } else { updated.removeValue(forKey: "role") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingName(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["name"] = value } else { updated.removeValue(forKey: "name") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingEncryptedValue(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["encryptedValue"] = value } else { updated.removeValue(forKey: "encryptedValue") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingContent(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["content"] = value } else { updated.removeValue(forKey: "content") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(id: String, content: Any) throws {
        let values: [String: Any] = [
            "role": "user",
            "id": id,
            "content": content,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1ToolMessage: AGUI1Definition {
    public static let definitionName = "ToolMessage"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var subagentRunId: AGUI1SubagentRunId? { AGUI1Nested.decode(fields["subagentRunId"], as: AGUI1SubagentRunId.self) }
    public var id: String? { fields["id"] as? String }
    public var role: String? { fields["role"] as? String }
    public var content: Any? { fields["content"] }
    public var toolCallId: String? { fields["toolCallId"] as? String }
    public var error: String? { fields["error"] as? String }
    public var encryptedValue: String? { fields["encryptedValue"] as? String }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public func settingSubagentRunId(_ value: AGUI1SubagentRunId?) throws -> Self {
        var updated = fields
        if let value { updated["subagentRunId"] = value.document.value } else { updated.removeValue(forKey: "subagentRunId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["id"] = value } else { updated.removeValue(forKey: "id") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRole(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["role"] = value } else { updated.removeValue(forKey: "role") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingContent(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["content"] = value } else { updated.removeValue(forKey: "content") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingToolCallId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["toolCallId"] = value } else { updated.removeValue(forKey: "toolCallId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingError(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["error"] = value } else { updated.removeValue(forKey: "error") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingEncryptedValue(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["encryptedValue"] = value } else { updated.removeValue(forKey: "encryptedValue") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(id: String, content: Any, toolCallId: String) throws {
        let values: [String: Any] = [
            "role": "tool",
            "id": id,
            "content": content,
            "toolCallId": toolCallId,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1ActivityMessage: AGUI1Definition {
    public static let definitionName = "ActivityMessage"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var subagentRunId: AGUI1SubagentRunId? { AGUI1Nested.decode(fields["subagentRunId"], as: AGUI1SubagentRunId.self) }
    public var id: String? { fields["id"] as? String }
    public var role: String? { fields["role"] as? String }
    public var activityType: String? { fields["activityType"] as? String }
    public var content: [String: Any]? { fields["content"] as? [String: Any] }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public func settingSubagentRunId(_ value: AGUI1SubagentRunId?) throws -> Self {
        var updated = fields
        if let value { updated["subagentRunId"] = value.document.value } else { updated.removeValue(forKey: "subagentRunId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["id"] = value } else { updated.removeValue(forKey: "id") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRole(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["role"] = value } else { updated.removeValue(forKey: "role") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingActivityType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["activityType"] = value } else { updated.removeValue(forKey: "activityType") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingContent(_ value: [String: Any]?) throws -> Self {
        var updated = fields
        if let value { updated["content"] = value } else { updated.removeValue(forKey: "content") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(id: String, activityType: String, content: [String: Any]) throws {
        let values: [String: Any] = [
            "role": "activity",
            "id": id,
            "activityType": activityType,
            "content": content,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1ReasoningMessage: AGUI1Definition {
    public static let definitionName = "ReasoningMessage"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var subagentRunId: AGUI1SubagentRunId? { AGUI1Nested.decode(fields["subagentRunId"], as: AGUI1SubagentRunId.self) }
    public var id: String? { fields["id"] as? String }
    public var role: String? { fields["role"] as? String }
    public var content: String? { fields["content"] as? String }
    public var encryptedValue: String? { fields["encryptedValue"] as? String }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public func settingSubagentRunId(_ value: AGUI1SubagentRunId?) throws -> Self {
        var updated = fields
        if let value { updated["subagentRunId"] = value.document.value } else { updated.removeValue(forKey: "subagentRunId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["id"] = value } else { updated.removeValue(forKey: "id") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRole(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["role"] = value } else { updated.removeValue(forKey: "role") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingContent(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["content"] = value } else { updated.removeValue(forKey: "content") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingEncryptedValue(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["encryptedValue"] = value } else { updated.removeValue(forKey: "encryptedValue") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(id: String, content: String) throws {
        let values: [String: Any] = [
            "role": "reasoning",
            "id": id,
            "content": content,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1ToolCall: AGUI1Definition {
    public static let definitionName = "ToolCall"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var id: String? { fields["id"] as? String }
    public var type: String? { fields["type"] as? String }
    public var function: AGUI1FunctionCall? { AGUI1Nested.decode(fields["function"], as: AGUI1FunctionCall.self) }
    public var encryptedValue: String? { fields["encryptedValue"] as? String }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public func settingId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["id"] = value } else { updated.removeValue(forKey: "id") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingFunction(_ value: AGUI1FunctionCall?) throws -> Self {
        var updated = fields
        if let value { updated["function"] = value.document.value } else { updated.removeValue(forKey: "function") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingEncryptedValue(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["encryptedValue"] = value } else { updated.removeValue(forKey: "encryptedValue") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(id: String, function: AGUI1FunctionCall) throws {
        let values: [String: Any] = [
            "type": "function",
            "id": id,
            "function": function.document.value,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1FunctionCall: AGUI1Definition {
    public static let definitionName = "FunctionCall"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var name: String? { fields["name"] as? String }
    public var arguments: String? { fields["arguments"] as? String }
    public func settingName(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["name"] = value } else { updated.removeValue(forKey: "name") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingArguments(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["arguments"] = value } else { updated.removeValue(forKey: "arguments") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(name: String, arguments: String) throws {
        let values: [String: Any] = [
            "name": name,
            "arguments": arguments,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1ContentPart: AGUI1Definition {
    public static let definitionName = "ContentPart"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var asTextPart: AGUI1TextPart? { AGUI1Nested.decode(document.value, as: AGUI1TextPart.self) }
    public var asImagePart: AGUI1ImagePart? { AGUI1Nested.decode(document.value, as: AGUI1ImagePart.self) }
    public var asAudioPart: AGUI1AudioPart? { AGUI1Nested.decode(document.value, as: AGUI1AudioPart.self) }
    public var asVideoPart: AGUI1VideoPart? { AGUI1Nested.decode(document.value, as: AGUI1VideoPart.self) }
    public var asDocumentPart: AGUI1DocumentPart? { AGUI1Nested.decode(document.value, as: AGUI1DocumentPart.self) }
}

public struct AGUI1TextPart: AGUI1Definition {
    public static let definitionName = "TextPart"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var id: String? { fields["id"] as? String }
    public var text: String? { fields["text"] as? String }
    public var metadata: Any? { fields["metadata"] }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["id"] = value } else { updated.removeValue(forKey: "id") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingText(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["text"] = value } else { updated.removeValue(forKey: "text") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(text: String) throws {
        let values: [String: Any] = [
            "type": "text",
            "text": text,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1ImagePart: AGUI1Definition {
    public static let definitionName = "ImagePart"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var id: String? { fields["id"] as? String }
    public var source: AGUI1PartSource? { AGUI1Nested.decode(fields["source"], as: AGUI1PartSource.self) }
    public var metadata: Any? { fields["metadata"] }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["id"] = value } else { updated.removeValue(forKey: "id") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingSource(_ value: AGUI1PartSource?) throws -> Self {
        var updated = fields
        if let value { updated["source"] = value.document.value } else { updated.removeValue(forKey: "source") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(source: AGUI1PartSource) throws {
        let values: [String: Any] = [
            "type": "image",
            "source": source.document.value,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1AudioPart: AGUI1Definition {
    public static let definitionName = "AudioPart"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var id: String? { fields["id"] as? String }
    public var source: AGUI1PartSource? { AGUI1Nested.decode(fields["source"], as: AGUI1PartSource.self) }
    public var metadata: Any? { fields["metadata"] }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["id"] = value } else { updated.removeValue(forKey: "id") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingSource(_ value: AGUI1PartSource?) throws -> Self {
        var updated = fields
        if let value { updated["source"] = value.document.value } else { updated.removeValue(forKey: "source") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(source: AGUI1PartSource) throws {
        let values: [String: Any] = [
            "type": "audio",
            "source": source.document.value,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1VideoPart: AGUI1Definition {
    public static let definitionName = "VideoPart"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var id: String? { fields["id"] as? String }
    public var source: AGUI1PartSource? { AGUI1Nested.decode(fields["source"], as: AGUI1PartSource.self) }
    public var metadata: Any? { fields["metadata"] }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["id"] = value } else { updated.removeValue(forKey: "id") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingSource(_ value: AGUI1PartSource?) throws -> Self {
        var updated = fields
        if let value { updated["source"] = value.document.value } else { updated.removeValue(forKey: "source") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(source: AGUI1PartSource) throws {
        let values: [String: Any] = [
            "type": "video",
            "source": source.document.value,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1DocumentPart: AGUI1Definition {
    public static let definitionName = "DocumentPart"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var id: String? { fields["id"] as? String }
    public var source: AGUI1PartSource? { AGUI1Nested.decode(fields["source"], as: AGUI1PartSource.self) }
    public var metadata: Any? { fields["metadata"] }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["id"] = value } else { updated.removeValue(forKey: "id") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingSource(_ value: AGUI1PartSource?) throws -> Self {
        var updated = fields
        if let value { updated["source"] = value.document.value } else { updated.removeValue(forKey: "source") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(source: AGUI1PartSource) throws {
        let values: [String: Any] = [
            "type": "document",
            "source": source.document.value,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1PartSource: AGUI1Definition {
    public static let definitionName = "PartSource"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var asDataSource: AGUI1DataSource? { AGUI1Nested.decode(document.value, as: AGUI1DataSource.self) }
    public var asUrlSource: AGUI1UrlSource? { AGUI1Nested.decode(document.value, as: AGUI1UrlSource.self) }
    public var asFileSource: AGUI1FileSource? { AGUI1Nested.decode(document.value, as: AGUI1FileSource.self) }
}

public struct AGUI1DataSource: AGUI1Definition {
    public static let definitionName = "DataSource"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var value: String? { fields["value"] as? String }
    public var mimeType: String? { fields["mimeType"] as? String }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingValue(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["value"] = value } else { updated.removeValue(forKey: "value") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMimeType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["mimeType"] = value } else { updated.removeValue(forKey: "mimeType") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(value: String, mimeType: String) throws {
        let values: [String: Any] = [
            "type": "data",
            "value": value,
            "mimeType": mimeType,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1UrlSource: AGUI1Definition {
    public static let definitionName = "UrlSource"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var value: String? { fields["value"] as? String }
    public var mimeType: String? { fields["mimeType"] as? String }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingValue(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["value"] = value } else { updated.removeValue(forKey: "value") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMimeType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["mimeType"] = value } else { updated.removeValue(forKey: "mimeType") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(value: String) throws {
        let values: [String: Any] = [
            "type": "url",
            "value": value,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1FileSource: AGUI1Definition {
    public static let definitionName = "FileSource"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var type: String? { fields["type"] as? String }
    public var value: String? { fields["value"] as? String }
    public var provider: String? { fields["provider"] as? String }
    public var mimeType: String? { fields["mimeType"] as? String }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingValue(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["value"] = value } else { updated.removeValue(forKey: "value") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingProvider(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["provider"] = value } else { updated.removeValue(forKey: "provider") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMimeType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["mimeType"] = value } else { updated.removeValue(forKey: "mimeType") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(value: String) throws {
        let values: [String: Any] = [
            "type": "file",
            "value": value,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1Context: AGUI1Definition {
    public static let definitionName = "Context"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var description: String? { fields["description"] as? String }
    public var value: String? { fields["value"] as? String }
    public func settingDescription(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["description"] = value } else { updated.removeValue(forKey: "description") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingValue(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["value"] = value } else { updated.removeValue(forKey: "value") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(description: String, value: String) throws {
        let values: [String: Any] = [
            "description": description,
            "value": value,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1Tool: AGUI1Definition {
    public static let definitionName = "Tool"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var name: String? { fields["name"] as? String }
    public var description: String? { fields["description"] as? String }
    public var parameters: Any? { fields["parameters"] }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public func settingName(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["name"] = value } else { updated.removeValue(forKey: "name") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingDescription(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["description"] = value } else { updated.removeValue(forKey: "description") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingParameters(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["parameters"] = value } else { updated.removeValue(forKey: "parameters") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(name: String, description: String) throws {
        let values: [String: Any] = [
            "name": name,
            "description": description,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1RunAgentInput: AGUI1Definition {
    public static let definitionName = "RunAgentInput"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var threadId: String? { fields["threadId"] as? String }
    public var runId: String? { fields["runId"] as? String }
    public var protocolVersion: String? { fields["protocolVersion"] as? String }
    public var parentRunId: String? { fields["parentRunId"] as? String }
    public var state: AGUI1State? { AGUI1Nested.decode(fields["state"], as: AGUI1State.self) }
    public var messages: [AGUI1Message]? { (fields["messages"] as? [Any])?.compactMap { AGUI1Nested.decode($0, as: AGUI1Message.self) } }
    public var tools: [AGUI1Tool]? { (fields["tools"] as? [Any])?.compactMap { AGUI1Nested.decode($0, as: AGUI1Tool.self) } }
    public var context: [AGUI1Context]? { (fields["context"] as? [Any])?.compactMap { AGUI1Nested.decode($0, as: AGUI1Context.self) } }
    public var forwardedProps: Any? { fields["forwardedProps"] }
    public var resume: [AGUI1ResumeEntry]? { (fields["resume"] as? [Any])?.compactMap { AGUI1Nested.decode($0, as: AGUI1ResumeEntry.self) } }
    public func settingThreadId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["threadId"] = value } else { updated.removeValue(forKey: "threadId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingRunId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["runId"] = value } else { updated.removeValue(forKey: "runId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingProtocolVersion(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["protocolVersion"] = value } else { updated.removeValue(forKey: "protocolVersion") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingParentRunId(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["parentRunId"] = value } else { updated.removeValue(forKey: "parentRunId") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingState(_ value: AGUI1State?) throws -> Self {
        var updated = fields
        if let value { updated["state"] = value.document.value } else { updated.removeValue(forKey: "state") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMessages(_ value: [AGUI1Message]?) throws -> Self {
        var updated = fields
        if let value { updated["messages"] = value.map { $0.document.value } } else { updated.removeValue(forKey: "messages") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingTools(_ value: [AGUI1Tool]?) throws -> Self {
        var updated = fields
        if let value { updated["tools"] = value.map { $0.document.value } } else { updated.removeValue(forKey: "tools") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingContext(_ value: [AGUI1Context]?) throws -> Self {
        var updated = fields
        if let value { updated["context"] = value.map { $0.document.value } } else { updated.removeValue(forKey: "context") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingForwardedProps(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["forwardedProps"] = value } else { updated.removeValue(forKey: "forwardedProps") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingResume(_ value: [AGUI1ResumeEntry]?) throws -> Self {
        var updated = fields
        if let value { updated["resume"] = value.map { $0.document.value } } else { updated.removeValue(forKey: "resume") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(threadId: String, runId: String, messages: [AGUI1Message]) throws {
        let values: [String: Any] = [
            "threadId": threadId,
            "runId": runId,
            "messages": messages.map { $0.document.value },
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1SubagentInfo: AGUI1Definition {
    public static let definitionName = "SubagentInfo"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var name: String? { fields["name"] as? String }
    public var description: String? { fields["description"] as? String }
    public func settingName(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["name"] = value } else { updated.removeValue(forKey: "name") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingDescription(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["description"] = value } else { updated.removeValue(forKey: "description") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(name: String) throws {
        let values: [String: Any] = [
            "name": name,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1IdentityCapabilities: AGUI1Definition {
    public static let definitionName = "IdentityCapabilities"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var name: String? { fields["name"] as? String }
    public var type: String? { fields["type"] as? String }
    public var description: String? { fields["description"] as? String }
    public var version: String? { fields["version"] as? String }
    public var provider: String? { fields["provider"] as? String }
    public var documentationUrl: String? { fields["documentationUrl"] as? String }
    public var metadata: AGUI1Metadata? { AGUI1Nested.decode(fields["metadata"], as: AGUI1Metadata.self) }
    public func settingName(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["name"] = value } else { updated.removeValue(forKey: "name") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingType(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["type"] = value } else { updated.removeValue(forKey: "type") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingDescription(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["description"] = value } else { updated.removeValue(forKey: "description") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingVersion(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["version"] = value } else { updated.removeValue(forKey: "version") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingProvider(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["provider"] = value } else { updated.removeValue(forKey: "provider") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingDocumentationUrl(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["documentationUrl"] = value } else { updated.removeValue(forKey: "documentationUrl") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMetadata(_ value: AGUI1Metadata?) throws -> Self {
        var updated = fields
        if let value { updated["metadata"] = value.document.value } else { updated.removeValue(forKey: "metadata") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1TransportCapabilities: AGUI1Definition {
    public static let definitionName = "TransportCapabilities"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var streaming: Bool? { fields["streaming"] as? Bool }
    public var websocket: Bool? { fields["websocket"] as? Bool }
    public var httpBinary: Bool? { fields["httpBinary"] as? Bool }
    public var pushNotifications: Bool? { fields["pushNotifications"] as? Bool }
    public var resumable: Bool? { fields["resumable"] as? Bool }
    public func settingStreaming(_ value: Bool?) throws -> Self {
        var updated = fields
        if let value { updated["streaming"] = value } else { updated.removeValue(forKey: "streaming") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingWebsocket(_ value: Bool?) throws -> Self {
        var updated = fields
        if let value { updated["websocket"] = value } else { updated.removeValue(forKey: "websocket") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingHttpBinary(_ value: Bool?) throws -> Self {
        var updated = fields
        if let value { updated["httpBinary"] = value } else { updated.removeValue(forKey: "httpBinary") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingPushNotifications(_ value: Bool?) throws -> Self {
        var updated = fields
        if let value { updated["pushNotifications"] = value } else { updated.removeValue(forKey: "pushNotifications") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingResumable(_ value: Bool?) throws -> Self {
        var updated = fields
        if let value { updated["resumable"] = value } else { updated.removeValue(forKey: "resumable") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1ToolsCapabilities: AGUI1Definition {
    public static let definitionName = "ToolsCapabilities"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var supported: Bool? { fields["supported"] as? Bool }
    public var items: [AGUI1Tool]? { (fields["items"] as? [Any])?.compactMap { AGUI1Nested.decode($0, as: AGUI1Tool.self) } }
    public var parallelCalls: Bool? { fields["parallelCalls"] as? Bool }
    public var clientProvided: Bool? { fields["clientProvided"] as? Bool }
    public func settingSupported(_ value: Bool?) throws -> Self {
        var updated = fields
        if let value { updated["supported"] = value } else { updated.removeValue(forKey: "supported") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingItems(_ value: [AGUI1Tool]?) throws -> Self {
        var updated = fields
        if let value { updated["items"] = value.map { $0.document.value } } else { updated.removeValue(forKey: "items") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingParallelCalls(_ value: Bool?) throws -> Self {
        var updated = fields
        if let value { updated["parallelCalls"] = value } else { updated.removeValue(forKey: "parallelCalls") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingClientProvided(_ value: Bool?) throws -> Self {
        var updated = fields
        if let value { updated["clientProvided"] = value } else { updated.removeValue(forKey: "clientProvided") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1OutputCapabilities: AGUI1Definition {
    public static let definitionName = "OutputCapabilities"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var structuredOutput: Bool? { fields["structuredOutput"] as? Bool }
    public var supportedMimeTypes: [String]? { fields["supportedMimeTypes"] as? [String] }
    public func settingStructuredOutput(_ value: Bool?) throws -> Self {
        var updated = fields
        if let value { updated["structuredOutput"] = value } else { updated.removeValue(forKey: "structuredOutput") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingSupportedMimeTypes(_ value: [String]?) throws -> Self {
        var updated = fields
        if let value { updated["supportedMimeTypes"] = value } else { updated.removeValue(forKey: "supportedMimeTypes") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1StateCapabilities: AGUI1Definition {
    public static let definitionName = "StateCapabilities"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var snapshots: Bool? { fields["snapshots"] as? Bool }
    public var deltas: Bool? { fields["deltas"] as? Bool }
    public var memory: Bool? { fields["memory"] as? Bool }
    public var persistentState: Bool? { fields["persistentState"] as? Bool }
    public func settingSnapshots(_ value: Bool?) throws -> Self {
        var updated = fields
        if let value { updated["snapshots"] = value } else { updated.removeValue(forKey: "snapshots") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingDeltas(_ value: Bool?) throws -> Self {
        var updated = fields
        if let value { updated["deltas"] = value } else { updated.removeValue(forKey: "deltas") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMemory(_ value: Bool?) throws -> Self {
        var updated = fields
        if let value { updated["memory"] = value } else { updated.removeValue(forKey: "memory") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingPersistentState(_ value: Bool?) throws -> Self {
        var updated = fields
        if let value { updated["persistentState"] = value } else { updated.removeValue(forKey: "persistentState") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1MultiAgentCapabilities: AGUI1Definition {
    public static let definitionName = "MultiAgentCapabilities"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var supported: Bool? { fields["supported"] as? Bool }
    public var delegation: Bool? { fields["delegation"] as? Bool }
    public var handoffs: Bool? { fields["handoffs"] as? Bool }
    public var subagents: [AGUI1SubagentInfo]? { (fields["subagents"] as? [Any])?.compactMap { AGUI1Nested.decode($0, as: AGUI1SubagentInfo.self) } }
    public func settingSupported(_ value: Bool?) throws -> Self {
        var updated = fields
        if let value { updated["supported"] = value } else { updated.removeValue(forKey: "supported") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingDelegation(_ value: Bool?) throws -> Self {
        var updated = fields
        if let value { updated["delegation"] = value } else { updated.removeValue(forKey: "delegation") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingHandoffs(_ value: Bool?) throws -> Self {
        var updated = fields
        if let value { updated["handoffs"] = value } else { updated.removeValue(forKey: "handoffs") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingSubagents(_ value: [AGUI1SubagentInfo]?) throws -> Self {
        var updated = fields
        if let value { updated["subagents"] = value.map { $0.document.value } } else { updated.removeValue(forKey: "subagents") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1ReasoningCapabilities: AGUI1Definition {
    public static let definitionName = "ReasoningCapabilities"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var supported: Bool? { fields["supported"] as? Bool }
    public var streaming: Bool? { fields["streaming"] as? Bool }
    public var encrypted: Bool? { fields["encrypted"] as? Bool }
    public func settingSupported(_ value: Bool?) throws -> Self {
        var updated = fields
        if let value { updated["supported"] = value } else { updated.removeValue(forKey: "supported") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingStreaming(_ value: Bool?) throws -> Self {
        var updated = fields
        if let value { updated["streaming"] = value } else { updated.removeValue(forKey: "streaming") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingEncrypted(_ value: Bool?) throws -> Self {
        var updated = fields
        if let value { updated["encrypted"] = value } else { updated.removeValue(forKey: "encrypted") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1MultimodalInputCapabilities: AGUI1Definition {
    public static let definitionName = "MultimodalInputCapabilities"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var image: Bool? { fields["image"] as? Bool }
    public var audio: Bool? { fields["audio"] as? Bool }
    public var video: Bool? { fields["video"] as? Bool }
    public var pdf: Bool? { fields["pdf"] as? Bool }
    public var file: Bool? { fields["file"] as? Bool }
    public func settingImage(_ value: Bool?) throws -> Self {
        var updated = fields
        if let value { updated["image"] = value } else { updated.removeValue(forKey: "image") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingAudio(_ value: Bool?) throws -> Self {
        var updated = fields
        if let value { updated["audio"] = value } else { updated.removeValue(forKey: "audio") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingVideo(_ value: Bool?) throws -> Self {
        var updated = fields
        if let value { updated["video"] = value } else { updated.removeValue(forKey: "video") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingPdf(_ value: Bool?) throws -> Self {
        var updated = fields
        if let value { updated["pdf"] = value } else { updated.removeValue(forKey: "pdf") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingFile(_ value: Bool?) throws -> Self {
        var updated = fields
        if let value { updated["file"] = value } else { updated.removeValue(forKey: "file") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1MultimodalOutputCapabilities: AGUI1Definition {
    public static let definitionName = "MultimodalOutputCapabilities"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var image: Bool? { fields["image"] as? Bool }
    public var audio: Bool? { fields["audio"] as? Bool }
    public func settingImage(_ value: Bool?) throws -> Self {
        var updated = fields
        if let value { updated["image"] = value } else { updated.removeValue(forKey: "image") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingAudio(_ value: Bool?) throws -> Self {
        var updated = fields
        if let value { updated["audio"] = value } else { updated.removeValue(forKey: "audio") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1MultimodalCapabilities: AGUI1Definition {
    public static let definitionName = "MultimodalCapabilities"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var input: AGUI1MultimodalInputCapabilities? { AGUI1Nested.decode(fields["input"], as: AGUI1MultimodalInputCapabilities.self) }
    public var output: AGUI1MultimodalOutputCapabilities? { AGUI1Nested.decode(fields["output"], as: AGUI1MultimodalOutputCapabilities.self) }
    public func settingInput(_ value: AGUI1MultimodalInputCapabilities?) throws -> Self {
        var updated = fields
        if let value { updated["input"] = value.document.value } else { updated.removeValue(forKey: "input") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingOutput(_ value: AGUI1MultimodalOutputCapabilities?) throws -> Self {
        var updated = fields
        if let value { updated["output"] = value.document.value } else { updated.removeValue(forKey: "output") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1ExecutionCapabilities: AGUI1Definition {
    public static let definitionName = "ExecutionCapabilities"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var codeExecution: Bool? { fields["codeExecution"] as? Bool }
    public var sandboxed: Bool? { fields["sandboxed"] as? Bool }
    public var maxIterations: Int64? { (fields["maxIterations"] as? NSNumber)?.int64Value }
    public var maxExecutionTime: Int64? { (fields["maxExecutionTime"] as? NSNumber)?.int64Value }
    public func settingCodeExecution(_ value: Bool?) throws -> Self {
        var updated = fields
        if let value { updated["codeExecution"] = value } else { updated.removeValue(forKey: "codeExecution") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingSandboxed(_ value: Bool?) throws -> Self {
        var updated = fields
        if let value { updated["sandboxed"] = value } else { updated.removeValue(forKey: "sandboxed") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMaxIterations(_ value: Int64?) throws -> Self {
        var updated = fields
        if let value { updated["maxIterations"] = value } else { updated.removeValue(forKey: "maxIterations") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMaxExecutionTime(_ value: Int64?) throws -> Self {
        var updated = fields
        if let value { updated["maxExecutionTime"] = value } else { updated.removeValue(forKey: "maxExecutionTime") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1HumanInTheLoopCapabilities: AGUI1Definition {
    public static let definitionName = "HumanInTheLoopCapabilities"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var supported: Bool? { fields["supported"] as? Bool }
    public var approvals: Bool? { fields["approvals"] as? Bool }
    public var interventions: Bool? { fields["interventions"] as? Bool }
    public var feedback: Bool? { fields["feedback"] as? Bool }
    public var interrupts: Bool? { fields["interrupts"] as? Bool }
    public var approveWithEdits: Bool? { fields["approveWithEdits"] as? Bool }
    public func settingSupported(_ value: Bool?) throws -> Self {
        var updated = fields
        if let value { updated["supported"] = value } else { updated.removeValue(forKey: "supported") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingApprovals(_ value: Bool?) throws -> Self {
        var updated = fields
        if let value { updated["approvals"] = value } else { updated.removeValue(forKey: "approvals") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingInterventions(_ value: Bool?) throws -> Self {
        var updated = fields
        if let value { updated["interventions"] = value } else { updated.removeValue(forKey: "interventions") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingFeedback(_ value: Bool?) throws -> Self {
        var updated = fields
        if let value { updated["feedback"] = value } else { updated.removeValue(forKey: "feedback") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingInterrupts(_ value: Bool?) throws -> Self {
        var updated = fields
        if let value { updated["interrupts"] = value } else { updated.removeValue(forKey: "interrupts") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingApproveWithEdits(_ value: Bool?) throws -> Self {
        var updated = fields
        if let value { updated["approveWithEdits"] = value } else { updated.removeValue(forKey: "approveWithEdits") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1AgentCapabilities: AGUI1Definition {
    public static let definitionName = "AgentCapabilities"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var identity: AGUI1IdentityCapabilities? { AGUI1Nested.decode(fields["identity"], as: AGUI1IdentityCapabilities.self) }
    public var transport: AGUI1TransportCapabilities? { AGUI1Nested.decode(fields["transport"], as: AGUI1TransportCapabilities.self) }
    public var tools: AGUI1ToolsCapabilities? { AGUI1Nested.decode(fields["tools"], as: AGUI1ToolsCapabilities.self) }
    public var output: AGUI1OutputCapabilities? { AGUI1Nested.decode(fields["output"], as: AGUI1OutputCapabilities.self) }
    public var state: AGUI1StateCapabilities? { AGUI1Nested.decode(fields["state"], as: AGUI1StateCapabilities.self) }
    public var multiAgent: AGUI1MultiAgentCapabilities? { AGUI1Nested.decode(fields["multiAgent"], as: AGUI1MultiAgentCapabilities.self) }
    public var reasoning: AGUI1ReasoningCapabilities? { AGUI1Nested.decode(fields["reasoning"], as: AGUI1ReasoningCapabilities.self) }
    public var multimodal: AGUI1MultimodalCapabilities? { AGUI1Nested.decode(fields["multimodal"], as: AGUI1MultimodalCapabilities.self) }
    public var execution: AGUI1ExecutionCapabilities? { AGUI1Nested.decode(fields["execution"], as: AGUI1ExecutionCapabilities.self) }
    public var humanInTheLoop: AGUI1HumanInTheLoopCapabilities? { AGUI1Nested.decode(fields["humanInTheLoop"], as: AGUI1HumanInTheLoopCapabilities.self) }
    public var custom: [String: Any]? { fields["custom"] as? [String: Any] }
    public func settingIdentity(_ value: AGUI1IdentityCapabilities?) throws -> Self {
        var updated = fields
        if let value { updated["identity"] = value.document.value } else { updated.removeValue(forKey: "identity") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingTransport(_ value: AGUI1TransportCapabilities?) throws -> Self {
        var updated = fields
        if let value { updated["transport"] = value.document.value } else { updated.removeValue(forKey: "transport") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingTools(_ value: AGUI1ToolsCapabilities?) throws -> Self {
        var updated = fields
        if let value { updated["tools"] = value.document.value } else { updated.removeValue(forKey: "tools") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingOutput(_ value: AGUI1OutputCapabilities?) throws -> Self {
        var updated = fields
        if let value { updated["output"] = value.document.value } else { updated.removeValue(forKey: "output") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingState(_ value: AGUI1StateCapabilities?) throws -> Self {
        var updated = fields
        if let value { updated["state"] = value.document.value } else { updated.removeValue(forKey: "state") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMultiAgent(_ value: AGUI1MultiAgentCapabilities?) throws -> Self {
        var updated = fields
        if let value { updated["multiAgent"] = value.document.value } else { updated.removeValue(forKey: "multiAgent") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingReasoning(_ value: AGUI1ReasoningCapabilities?) throws -> Self {
        var updated = fields
        if let value { updated["reasoning"] = value.document.value } else { updated.removeValue(forKey: "reasoning") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingMultimodal(_ value: AGUI1MultimodalCapabilities?) throws -> Self {
        var updated = fields
        if let value { updated["multimodal"] = value.document.value } else { updated.removeValue(forKey: "multimodal") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingExecution(_ value: AGUI1ExecutionCapabilities?) throws -> Self {
        var updated = fields
        if let value { updated["execution"] = value.document.value } else { updated.removeValue(forKey: "execution") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingHumanInTheLoop(_ value: AGUI1HumanInTheLoopCapabilities?) throws -> Self {
        var updated = fields
        if let value { updated["humanInTheLoop"] = value.document.value } else { updated.removeValue(forKey: "humanInTheLoop") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingCustom(_ value: [String: Any]?) throws -> Self {
        var updated = fields
        if let value { updated["custom"] = value } else { updated.removeValue(forKey: "custom") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1JsonPatch: AGUI1Definition {
    public static let definitionName = "JsonPatch"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var items: [AGUI1JsonPatchOperation]? { (document.value as? [Any])?.compactMap { AGUI1Nested.decode($0, as: AGUI1JsonPatchOperation.self) } }
}

public struct AGUI1JsonPatchOperation: AGUI1Definition {
    public static let definitionName = "JsonPatchOperation"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var asAddOperation: AGUI1AddOperation? { AGUI1Nested.decode(document.value, as: AGUI1AddOperation.self) }
    public var asRemoveOperation: AGUI1RemoveOperation? { AGUI1Nested.decode(document.value, as: AGUI1RemoveOperation.self) }
    public var asReplaceOperation: AGUI1ReplaceOperation? { AGUI1Nested.decode(document.value, as: AGUI1ReplaceOperation.self) }
    public var asMoveOperation: AGUI1MoveOperation? { AGUI1Nested.decode(document.value, as: AGUI1MoveOperation.self) }
    public var asCopyOperation: AGUI1CopyOperation? { AGUI1Nested.decode(document.value, as: AGUI1CopyOperation.self) }
    public var asTestOperation: AGUI1TestOperation? { AGUI1Nested.decode(document.value, as: AGUI1TestOperation.self) }
}

public struct AGUI1AddOperation: AGUI1Definition {
    public static let definitionName = "AddOperation"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var op: String? { fields["op"] as? String }
    public var path: AGUI1JsonPointer? { AGUI1Nested.decode(fields["path"], as: AGUI1JsonPointer.self) }
    public var value: Any? { fields["value"] }
    public func settingOp(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["op"] = value } else { updated.removeValue(forKey: "op") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingPath(_ value: AGUI1JsonPointer?) throws -> Self {
        var updated = fields
        if let value { updated["path"] = value.document.value } else { updated.removeValue(forKey: "path") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingValue(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["value"] = value } else { updated.removeValue(forKey: "value") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(path: AGUI1JsonPointer, value: Any) throws {
        let values: [String: Any] = [
            "op": "add",
            "path": path.document.value,
            "value": value,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1RemoveOperation: AGUI1Definition {
    public static let definitionName = "RemoveOperation"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var op: String? { fields["op"] as? String }
    public var path: AGUI1JsonPointer? { AGUI1Nested.decode(fields["path"], as: AGUI1JsonPointer.self) }
    public func settingOp(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["op"] = value } else { updated.removeValue(forKey: "op") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingPath(_ value: AGUI1JsonPointer?) throws -> Self {
        var updated = fields
        if let value { updated["path"] = value.document.value } else { updated.removeValue(forKey: "path") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(path: AGUI1JsonPointer) throws {
        let values: [String: Any] = [
            "op": "remove",
            "path": path.document.value,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1ReplaceOperation: AGUI1Definition {
    public static let definitionName = "ReplaceOperation"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var op: String? { fields["op"] as? String }
    public var path: AGUI1JsonPointer? { AGUI1Nested.decode(fields["path"], as: AGUI1JsonPointer.self) }
    public var value: Any? { fields["value"] }
    public func settingOp(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["op"] = value } else { updated.removeValue(forKey: "op") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingPath(_ value: AGUI1JsonPointer?) throws -> Self {
        var updated = fields
        if let value { updated["path"] = value.document.value } else { updated.removeValue(forKey: "path") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingValue(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["value"] = value } else { updated.removeValue(forKey: "value") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(path: AGUI1JsonPointer, value: Any) throws {
        let values: [String: Any] = [
            "op": "replace",
            "path": path.document.value,
            "value": value,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1MoveOperation: AGUI1Definition {
    public static let definitionName = "MoveOperation"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var op: String? { fields["op"] as? String }
    public var from: AGUI1JsonPointer? { AGUI1Nested.decode(fields["from"], as: AGUI1JsonPointer.self) }
    public var path: AGUI1JsonPointer? { AGUI1Nested.decode(fields["path"], as: AGUI1JsonPointer.self) }
    public func settingOp(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["op"] = value } else { updated.removeValue(forKey: "op") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingFrom(_ value: AGUI1JsonPointer?) throws -> Self {
        var updated = fields
        if let value { updated["from"] = value.document.value } else { updated.removeValue(forKey: "from") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingPath(_ value: AGUI1JsonPointer?) throws -> Self {
        var updated = fields
        if let value { updated["path"] = value.document.value } else { updated.removeValue(forKey: "path") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(from: AGUI1JsonPointer, path: AGUI1JsonPointer) throws {
        let values: [String: Any] = [
            "op": "move",
            "from": from.document.value,
            "path": path.document.value,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1CopyOperation: AGUI1Definition {
    public static let definitionName = "CopyOperation"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var op: String? { fields["op"] as? String }
    public var from: AGUI1JsonPointer? { AGUI1Nested.decode(fields["from"], as: AGUI1JsonPointer.self) }
    public var path: AGUI1JsonPointer? { AGUI1Nested.decode(fields["path"], as: AGUI1JsonPointer.self) }
    public func settingOp(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["op"] = value } else { updated.removeValue(forKey: "op") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingFrom(_ value: AGUI1JsonPointer?) throws -> Self {
        var updated = fields
        if let value { updated["from"] = value.document.value } else { updated.removeValue(forKey: "from") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingPath(_ value: AGUI1JsonPointer?) throws -> Self {
        var updated = fields
        if let value { updated["path"] = value.document.value } else { updated.removeValue(forKey: "path") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(from: AGUI1JsonPointer, path: AGUI1JsonPointer) throws {
        let values: [String: Any] = [
            "op": "copy",
            "from": from.document.value,
            "path": path.document.value,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1TestOperation: AGUI1Definition {
    public static let definitionName = "TestOperation"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var op: String? { fields["op"] as? String }
    public var path: AGUI1JsonPointer? { AGUI1Nested.decode(fields["path"], as: AGUI1JsonPointer.self) }
    public var value: Any? { fields["value"] }
    public func settingOp(_ value: String?) throws -> Self {
        var updated = fields
        if let value { updated["op"] = value } else { updated.removeValue(forKey: "op") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingPath(_ value: AGUI1JsonPointer?) throws -> Self {
        var updated = fields
        if let value { updated["path"] = value.document.value } else { updated.removeValue(forKey: "path") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public func settingValue(_ value: Any?) throws -> Self {
        var updated = fields
        if let value { updated["value"] = value } else { updated.removeValue(forKey: "value") }
        return try Self(JSONSerialization.data(withJSONObject: updated, options: [.fragmentsAllowed]))
    }
    public init(path: AGUI1JsonPointer, value: Any) throws {
        let values: [String: Any] = [
            "op": "test",
            "path": path.document.value,
            "value": value,
        ]
        self = try Self(JSONSerialization.data(withJSONObject: values, options: [.fragmentsAllowed]))
    }
}

public struct AGUI1JsonPointer: AGUI1Definition {
    public static let definitionName = "JsonPointer"
    public let document: AGUISchemaDocument
    public init(document: AGUISchemaDocument) { self.document = document }
    public var value: String? { document.value as? String }
}

public enum AGUI1Definitions {
    public static let all: [String: any AGUI1Definition.Type] = [
        "Event": AGUI1Event.self,
        "EventType": AGUI1EventType.self,
        "BaseEvent": AGUI1BaseEvent.self,
        "Attributable": AGUI1Attributable.self,
        "SubagentRunId": AGUI1SubagentRunId.self,
        "Metadata": AGUI1Metadata.self,
        "State": AGUI1State.self,
        "TextMessageRole": AGUI1TextMessageRole.self,
        "Role": AGUI1Role.self,
        "TextMessageStartEvent": AGUI1TextMessageStartEvent.self,
        "TextMessageContentEvent": AGUI1TextMessageContentEvent.self,
        "TextMessageEndEvent": AGUI1TextMessageEndEvent.self,
        "TextMessageChunkEvent": AGUI1TextMessageChunkEvent.self,
        "ToolCallStartEvent": AGUI1ToolCallStartEvent.self,
        "ToolCallArgsEvent": AGUI1ToolCallArgsEvent.self,
        "ToolCallEndEvent": AGUI1ToolCallEndEvent.self,
        "ToolCallChunkEvent": AGUI1ToolCallChunkEvent.self,
        "ToolCallResultEvent": AGUI1ToolCallResultEvent.self,
        "StateSnapshotEvent": AGUI1StateSnapshotEvent.self,
        "StateDeltaEvent": AGUI1StateDeltaEvent.self,
        "MessagesSnapshotEvent": AGUI1MessagesSnapshotEvent.self,
        "ActivitySnapshotEvent": AGUI1ActivitySnapshotEvent.self,
        "ActivityDeltaEvent": AGUI1ActivityDeltaEvent.self,
        "RawEvent": AGUI1RawEvent.self,
        "CustomEvent": AGUI1CustomEvent.self,
        "RunStartedEvent": AGUI1RunStartedEvent.self,
        "RunFinishedEvent": AGUI1RunFinishedEvent.self,
        "RunErrorEvent": AGUI1RunErrorEvent.self,
        "StepStartedEvent": AGUI1StepStartedEvent.self,
        "StepFinishedEvent": AGUI1StepFinishedEvent.self,
        "ReasoningStartEvent": AGUI1ReasoningStartEvent.self,
        "ReasoningMessageStartEvent": AGUI1ReasoningMessageStartEvent.self,
        "ReasoningMessageContentEvent": AGUI1ReasoningMessageContentEvent.self,
        "ReasoningMessageEndEvent": AGUI1ReasoningMessageEndEvent.self,
        "ReasoningMessageChunkEvent": AGUI1ReasoningMessageChunkEvent.self,
        "ReasoningEndEvent": AGUI1ReasoningEndEvent.self,
        "ReasoningEncryptedValueEvent": AGUI1ReasoningEncryptedValueEvent.self,
        "ReasoningEncryptedValueSubtype": AGUI1ReasoningEncryptedValueSubtype.self,
        "SubagentStartedEvent": AGUI1SubagentStartedEvent.self,
        "SubagentFinishedEvent": AGUI1SubagentFinishedEvent.self,
        "SubagentErrorEvent": AGUI1SubagentErrorEvent.self,
        "RunFinishedOutcome": AGUI1RunFinishedOutcome.self,
        "RunFinishedSuccessOutcome": AGUI1RunFinishedSuccessOutcome.self,
        "RunFinishedInterruptOutcome": AGUI1RunFinishedInterruptOutcome.self,
        "RunFinishedCancelledOutcome": AGUI1RunFinishedCancelledOutcome.self,
        "SubagentFinishedOutcome": AGUI1SubagentFinishedOutcome.self,
        "SubagentFinishedSuccessOutcome": AGUI1SubagentFinishedSuccessOutcome.self,
        "SubagentFinishedSuspendedOutcome": AGUI1SubagentFinishedSuspendedOutcome.self,
        "Interrupt": AGUI1Interrupt.self,
        "ResumeEntry": AGUI1ResumeEntry.self,
        "TokenUsage": AGUI1TokenUsage.self,
        "Message": AGUI1Message.self,
        "BaseMessage": AGUI1BaseMessage.self,
        "DeveloperMessage": AGUI1DeveloperMessage.self,
        "SystemMessage": AGUI1SystemMessage.self,
        "AssistantMessage": AGUI1AssistantMessage.self,
        "UserMessage": AGUI1UserMessage.self,
        "ToolMessage": AGUI1ToolMessage.self,
        "ActivityMessage": AGUI1ActivityMessage.self,
        "ReasoningMessage": AGUI1ReasoningMessage.self,
        "ToolCall": AGUI1ToolCall.self,
        "FunctionCall": AGUI1FunctionCall.self,
        "ContentPart": AGUI1ContentPart.self,
        "TextPart": AGUI1TextPart.self,
        "ImagePart": AGUI1ImagePart.self,
        "AudioPart": AGUI1AudioPart.self,
        "VideoPart": AGUI1VideoPart.self,
        "DocumentPart": AGUI1DocumentPart.self,
        "PartSource": AGUI1PartSource.self,
        "DataSource": AGUI1DataSource.self,
        "UrlSource": AGUI1UrlSource.self,
        "FileSource": AGUI1FileSource.self,
        "Context": AGUI1Context.self,
        "Tool": AGUI1Tool.self,
        "RunAgentInput": AGUI1RunAgentInput.self,
        "SubagentInfo": AGUI1SubagentInfo.self,
        "IdentityCapabilities": AGUI1IdentityCapabilities.self,
        "TransportCapabilities": AGUI1TransportCapabilities.self,
        "ToolsCapabilities": AGUI1ToolsCapabilities.self,
        "OutputCapabilities": AGUI1OutputCapabilities.self,
        "StateCapabilities": AGUI1StateCapabilities.self,
        "MultiAgentCapabilities": AGUI1MultiAgentCapabilities.self,
        "ReasoningCapabilities": AGUI1ReasoningCapabilities.self,
        "MultimodalInputCapabilities": AGUI1MultimodalInputCapabilities.self,
        "MultimodalOutputCapabilities": AGUI1MultimodalOutputCapabilities.self,
        "MultimodalCapabilities": AGUI1MultimodalCapabilities.self,
        "ExecutionCapabilities": AGUI1ExecutionCapabilities.self,
        "HumanInTheLoopCapabilities": AGUI1HumanInTheLoopCapabilities.self,
        "AgentCapabilities": AGUI1AgentCapabilities.self,
        "JsonPatch": AGUI1JsonPatch.self,
        "JsonPatchOperation": AGUI1JsonPatchOperation.self,
        "AddOperation": AGUI1AddOperation.self,
        "RemoveOperation": AGUI1RemoveOperation.self,
        "ReplaceOperation": AGUI1ReplaceOperation.self,
        "MoveOperation": AGUI1MoveOperation.self,
        "CopyOperation": AGUI1CopyOperation.self,
        "TestOperation": AGUI1TestOperation.self,
        "JsonPointer": AGUI1JsonPointer.self,
    ]
}
