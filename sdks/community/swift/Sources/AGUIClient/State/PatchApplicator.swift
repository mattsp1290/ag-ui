import AGUICore
import Foundation

/// RFC 6902 edits over lossless JSON, preserving large integer lexemes.
public struct PatchApplicator: Sendable {
    public enum PatchError: Error, LocalizedError {
        case invalidJSON(String), invalidPatch(String), invalidOperation(String), pathNotFound(String), testFailed(String)
        public var errorDescription: String? {
            switch self {
            case .invalidJSON(let s): return "Invalid JSON: \(s)"
            case .invalidPatch(let s): return "Invalid patch: \(s)"
            case .invalidOperation(let s): return "Invalid operation: \(s)"
            case .pathNotFound(let s): return "Path not found: \(s)"
            case .testFailed(let s): return "Test failed: \(s)"
            }
        }
    }
    public init() {}

    public func apply(patch: Data, to state: Data) throws -> Data {
        let parsed: AGUIJSON
        do { parsed = try AGUIJSON.parse(state) }
        catch { throw PatchError.invalidJSON("Unable to parse state") }
        let operations: [AGUIJSON]
        do {
            guard let array = try AGUIJSON.parse(patch).array else { throw PatchError.invalidPatch("Expected array") }
            operations = array
        } catch { throw PatchError.invalidPatch("Unable to parse patch: \(error)") }
        var document = parsed
        for (index, entry) in operations.enumerated() {
            guard let fields = entry.object, let op = fields["op"]?.string, let path = fields["path"]?.string
            else { throw PatchError.invalidPatch("Malformed operation") }
            guard ["add", "remove", "replace", "move", "copy", "test"].contains(op) else {
                FileHandle.standardError.write(Data("Dropped unknown patch operation at /delta/\(index)\n".utf8))
                continue
            }
            let tokens = try pointer(path)
            switch op {
            case "add", "replace":
                guard let value = fields["value"] else { throw PatchError.invalidOperation("\(op) missing value") }
                document = try edit(document, tokens, value, op == "add" ? .add : .replace)
            case "remove": document = try edit(document, tokens, nil, .remove)
            case "move", "copy":
                guard let from = fields["from"]?.string else { throw PatchError.invalidOperation("\(op) missing from") }
                let source = try pointer(from)
                let value = try get(document, source)
                if op == "move" { document = try edit(document, source, nil, .remove) }
                document = try edit(document, tokens, value, .add)
            case "test":
                guard let value = fields["value"] else { throw PatchError.invalidOperation("test missing value") }
                guard try get(document, tokens) == value else { throw PatchError.testFailed(path) }
            default: break
            }
        }
        return try document.encoded()
    }
    private enum Edit { case add, remove, replace }
    private func pointer(_ path: String) throws -> [String] {
        if path.isEmpty { return [] }
        guard path.hasPrefix("/") else { throw PatchError.invalidOperation("Invalid pointer: \(path)") }
        return try path.dropFirst().split(separator: "/", omittingEmptySubsequences: false).map { part in
            let token = String(part)
            let unescaped = token.replacingOccurrences(of: "~0", with: "").replacingOccurrences(of: "~1", with: "")
            guard !unescaped.contains("~") else { throw PatchError.invalidOperation("Invalid pointer escape: \(path)") }
            return token.replacingOccurrences(of: "~1", with: "/").replacingOccurrences(of: "~0", with: "~")
        }
    }
    private func get(_ document: AGUIJSON, _ tokens: [String]) throws -> AGUIJSON {
        guard let head = tokens.first else { return document }
        let child: AGUIJSON?
        switch document {
        case .object(let fields): child = fields[head]
        case .array(let values):
            if let index = Int(head), values.indices.contains(index) { child = values[index] } else { child = nil }
        default: child = nil
        }
        guard let child else { throw PatchError.pathNotFound("/" + tokens.joined(separator: "/")) }
        return try get(child, Array(tokens.dropFirst()))
    }
    private func edit(_ document: AGUIJSON, _ tokens: [String], _ value: AGUIJSON?, _ kind: Edit) throws -> AGUIJSON {
        guard let head = tokens.first else {
            if kind == .remove { throw PatchError.invalidOperation("Cannot remove root") }
            return value!
        }
        let tail = Array(tokens.dropFirst())
        switch document {
        case .object(var fields):
            if tail.isEmpty {
                if kind != .add && fields[head] == nil { throw PatchError.pathNotFound(head) }
                if kind == .remove { fields.removeValue(forKey: head) } else { fields[head] = value }
            } else {
                guard let child = fields[head] else { throw PatchError.pathNotFound(head) }
                fields[head] = try edit(child, tail, value, kind)
            }
            return .object(fields)
        case .array(var values):
            let index = head == "-" && kind == .add && tail.isEmpty ? values.count : Int(head)
            guard let index, index >= 0 else { throw PatchError.pathNotFound(head) }
            if tail.isEmpty {
                if kind == .add {
                    guard index <= values.count else { throw PatchError.pathNotFound(head) }
                    values.insert(value!, at: index)
                } else {
                    guard values.indices.contains(index) else { throw PatchError.pathNotFound(head) }
                    if kind == .remove { values.remove(at: index) } else { values[index] = value! }
                }
            } else {
                guard values.indices.contains(index) else { throw PatchError.pathNotFound(head) }
                values[index] = try edit(values[index], tail, value, kind)
            }
            return .array(values)
        default: throw PatchError.pathNotFound(head)
        }
    }
}
