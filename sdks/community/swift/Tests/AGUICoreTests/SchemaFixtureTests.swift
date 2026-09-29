import Foundation
import XCTest
@testable import AGUICore

final class SchemaFixtureTests: XCTestCase {
    private var repository: URL {
        URL(fileURLWithPath: #filePath).deletingLastPathComponent()
            .appendingPathComponent("../../../../../")
            .standardizedFileURL
    }

    func testManifestAndFixtures() throws {
        let root = repository.appendingPathComponent("spec/1.0/fixtures")
        let manifest = try String(contentsOf: root.appendingPathComponent("MANIFEST.txt"), encoding: .utf8)
            .split(separator: "\n").map(String.init).filter { !$0.hasPrefix("#") && !$0.isEmpty }
        let actual = FileManager.default.enumerator(at: root, includingPropertiesForKeys: nil)!.allObjects
            .compactMap { $0 as? URL }.filter { $0.pathExtension == "json" }
            .map { String($0.path.dropFirst(root.path.count + 1)) }.sorted()
        XCTAssertEqual(manifest, actual)
        var valid = 0, invalid = 0
        for path in manifest where !path.hasSuffix(".expect.json") {
            let parts = path.split(separator: "/")
            let definition = String(parts[0])
            let input = try Data(contentsOf: root.appendingPathComponent(path))
            if parts[1] == "valid" {
                let output: Data
                do { output = try AGUI1Definitions.all[definition]!.init(input).encoded() } catch { XCTFail("\(path): \(error)"); continue }
                let original = try JSONSerialization.jsonObject(with: input, options: .fragmentsAllowed) as? NSObject
                let result = try JSONSerialization.jsonObject(with: output, options: .fragmentsAllowed) as? NSObject
                XCTAssertEqual(original, result, path)
                valid += 1
            } else {
                XCTAssertThrowsError(try AGUI1Definitions.all[definition]!.init(input), path)
                invalid += 1
            }
        }
        XCTAssertEqual(valid, 135)
        XCTAssertEqual(invalid, 84)
    }

    func testSDKFixtures() throws {
        for name in ["null-omission", "agent-capabilities"] {
            let data = try Data(contentsOf: repository.appendingPathComponent("sdks/fixtures/\(name).json"))
            let suite = try JSONSerialization.jsonObject(with: data) as! [String: Any]
            let cases = (suite["cases"] ?? suite["stream"]) as! [[String: Any]]
            for test in cases {
                let input = try JSONSerialization.data(withJSONObject: test["input"]!)
                let definition = name == "agent-capabilities" ? "AgentCapabilities" : "Event"
                let output = try AGUISchemaDocument(normalizing: input, definition: definition).encoded()
                let expected = try JSONSerialization.data(withJSONObject: test["expected"]!)
                XCTAssertEqual(try JSONSerialization.jsonObject(with: output) as? NSObject,
                               try JSONSerialization.jsonObject(with: expected) as? NSObject,
                               test["name"] as? String ?? "")
            }
        }
    }

    func testEventValuesAndLargeInteger() throws {
        XCTAssertEqual(EventType.protocolCases.count, 31)
        XCTAssertFalse(EventType.protocolCases.contains { $0.rawValue.hasPrefix("THINKING_") })
        let schemaData = try Data(contentsOf: repository.appendingPathComponent("spec/1.0/schema.json"))
        let schema = try JSONSerialization.jsonObject(with: schemaData) as! [String: Any]
        let definitions = schema["$defs"] as! [String: [String: Any]]
        let wireEvents = Set(definitions["EventType"]!["enum"] as! [String])
        XCTAssertEqual(Set(EventType.allCases.map(\.rawValue)), wireEvents)
        XCTAssertThrowsError(try JSONEncoder().encode(EventType.unknown))
        XCTAssertThrowsError(try JSONDecoder().decode(EventType.self, from: Data("\"__UNKNOWN__\"".utf8)))
        for type in EventType.allCases {
            XCTAssertEqual(try JSONDecoder().decode(EventType.self, from: JSONEncoder().encode(type)), type)
        }
        XCTAssertEqual(Set(AGUI1RoleValue.allCases.map(\.rawValue)),
                       Set(definitions["Role"]!["enum"] as! [String]))
        XCTAssertEqual(AGUI1Definitions.all.count, definitions.count)
        let original = Data(#"{"type":"STATE_SNAPSHOT","snapshot":{"large":9007199254740993}}"#.utf8)
        let output = try AGUISchemaDocument(original).encoded()
        XCTAssertTrue(String(decoding: output, as: UTF8.self).contains("9007199254740993"))
        XCTAssertEqual(String(decoding: try AGUISchemaDocument(original).sseFrame(), as: UTF8.self).prefix(6), "data: ")
    }

    func testNamedDefinitions() throws {
        let fixtures = repository.appendingPathComponent("spec/1.0/fixtures")
        let run = try AGUI1RunAgentInput(Data(contentsOf: fixtures.appendingPathComponent("RunAgentInput/valid/full.json")))
        XCTAssertNotNil(run.threadId)
        XCTAssertEqual(try AGUI1RunAgentInput(run.encoded()).threadId, run.threadId)

        let capabilities = try AGUI1AgentCapabilities(Data(contentsOf: fixtures.appendingPathComponent("AgentCapabilities/valid/full.json")))
        XCTAssertEqual(capabilities.identity?.name, "Research Assistant")
        XCTAssertEqual(capabilities.multiAgent?.subagents?.count, 2)

        let usage = try AGUI1TokenUsage(Data(#"{"inputTokens":9007199254740991,"outputTokens":1}"#.utf8))
        XCTAssertEqual(usage.inputTokens, 9007199254740991)
        XCTAssertTrue(String(decoding: try usage.encoded(), as: UTF8.self).contains("9007199254740991"))

        let part = try AGUI1ContentPart(Data(#"{"type":"text","text":"hello"}"#.utf8))
        XCTAssertEqual(part.asTextPart?.text, "hello")

        let created = try AGUI1RunAgentInput(threadId: "thread", runId: "run", messages: [])
            .settingProtocolVersion("1.0")
        XCTAssertEqual(created.protocolVersion, "1.0")
        XCTAssertEqual(try AGUI1RunAgentInput(created.encoded()).messages?.count, 0)

        let nullState = try AGUI1State(Data("null".utf8))
        let snapshot = try AGUI1StateSnapshotEvent(snapshot: nullState)
        XCTAssertTrue(String(decoding: try snapshot.encoded(), as: UTF8.self).contains("\"snapshot\":null"))
    }

    func testForwardCompatibleUnknownFieldAndUnboundedInteger() throws {
        let digits = "123456789012345678901234567890123456789012345678901234567890"
        let json = Data("{\"type\":\"STATE_SNAPSHOT\",\"snapshot\":{\"value\":\(digits)},\"futureField\":true}".utf8)
        XCTAssertThrowsError(try AGUI1StateSnapshotEvent(json))
        let forwarded = try AGUI1StateSnapshotEvent(forwardCompatible: json)
        XCTAssertTrue(String(decoding: try forwarded.encoded(), as: UTF8.self).contains(digits))
        XCTAssertTrue(String(decoding: try forwarded.snapshot!.encoded(), as: UTF8.self).contains(digits))
        let edited = try forwarded.settingTimestamp(1)
        XCTAssertTrue(String(decoding: try edited.encoded(), as: UTF8.self).contains(digits))
        XCTAssertEqual(edited.timestamp, 1)
        XCTAssertNotNil(edited.document.jsonFields["futureField"])
        let union = try AGUI1Event(forwardCompatible: json)
        XCTAssertTrue(String(decoding: try union.asStateSnapshotEvent!.snapshot!.encoded(), as: UTF8.self).contains(digits))
        let request = Data(#"{"threadId":"t","runId":"r","messages":[{"id":"m","role":"user","content":"hello","futureField":true}]}"#.utf8)
        let projected = try AGUI1RunAgentInput(forwardCompatible: request).messages
        XCTAssertEqual(projected?.count, 1)
        XCTAssertNotNil(projected?.first?.asUserMessage)
        XCTAssertTrue(String(decoding: try projected!.first!.encoded(), as: UTF8.self).contains("futureField"))
        XCTAssertEqual(String(decoding: try forwarded.document.sseFrame(), as: UTF8.self),
                       "data: \(String(decoding: try forwarded.encoded(), as: UTF8.self))\n\n")

        let pretty = Data("{\n  \"type\": \"STATE_SNAPSHOT\",\n  \"snapshot\": {\"value\": \(digits)}\n}".utf8)
        let frame = String(decoding: try AGUI1Event(pretty).document.sseFrame(), as: UTF8.self)
        XCTAssertEqual(frame.filter { $0 == "\n" }.count, 2)
        XCTAssertTrue(frame.hasPrefix("data: {"))
    }

    func testLosslessJSONRejectsInvalidNumbersAndDuplicateKeys() throws {
        for lexeme in ["01", "+1", "1.", "1e", "NaN", "--2"] {
            XCTAssertThrowsError(try AGUIJSON.number(lexeme).encoded(), lexeme)
        }
        for json in [#"{"x":1,"x":2}"#, #"{"x":1,"\u0078":2}"#,
                     "{\"x\":\"bad\nescape\"}", #"[1,]"#] {
            XCTAssertThrowsError(try AGUIJSON.parse(Data(json.utf8)), json)
        }
        let escaped = try AGUIJSON.parse(Data(#"{"text":"line\n\"quote\"","emoji":"\uD83D\uDE00"}"#.utf8))
        XCTAssertEqual(try AGUIJSON.parse(escaped.encoded()), escaped)
    }
}
