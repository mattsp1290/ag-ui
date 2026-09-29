// Copyright (c) 2025 Perfect Aduh. MIT License. See LICENSE for details.

import Foundation

struct ToolCallResultEventDTO: Decodable {
    let messageId: String
    let toolCallId: String
    let content: String
    let role: String?
    let timestamp: Int64?

    private enum CodingKeys: String, CodingKey {
        case messageId, toolCallId, content, role, timestamp
    }

    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        messageId = try container.decode(String.self, forKey: .messageId)
        toolCallId = try container.decode(String.self, forKey: .toolCallId)
        role = try container.decodeIfPresent(String.self, forKey: .role)
        timestamp = try container.decodeIfPresent(Int64.self, forKey: .timestamp)
        if let text = try? container.decode(String.self, forKey: .content) {
            content = text
        } else {
            // Keep structured result content available as JSON until the message
            // model can represent content parts directly.
            let value = try container.decode([JSONValue].self, forKey: .content)
            content = String(data: try JSONEncoder().encode(value), encoding: .utf8) ?? "[]"
        }
    }

    func toDomain(rawEvent: Data? = nil) -> ToolCallResultEvent {
        ToolCallResultEvent(
            messageId: messageId,
            toolCallId: toolCallId,
            content: content,
            role: role,
            timestamp: timestamp,
            rawEvent: rawEvent
        )
    }
}

private enum JSONValue: Codable {
    case object([String: JSONValue]), array([JSONValue]), string(String), number(Double), bool(Bool), null

    init(from decoder: Decoder) throws {
        let container = try decoder.singleValueContainer()
        if container.decodeNil() { self = .null }
        else if let value = try? container.decode(Bool.self) { self = .bool(value) }
        else if let value = try? container.decode(String.self) { self = .string(value) }
        else if let value = try? container.decode(Double.self) { self = .number(value) }
        else if let value = try? container.decode([JSONValue].self) { self = .array(value) }
        else { self = .object(try container.decode([String: JSONValue].self)) }
    }

    func encode(to encoder: Encoder) throws {
        var container = encoder.singleValueContainer()
        switch self {
        case .object(let value): try container.encode(value)
        case .array(let value): try container.encode(value)
        case .string(let value): try container.encode(value)
        case .number(let value): try container.encode(value)
        case .bool(let value): try container.encode(value)
        case .null: try container.encodeNil()
        }
    }
}
