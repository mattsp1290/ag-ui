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
        XCTAssertEqual(try forwarded.encoded(), json)
        XCTAssertEqual(String(decoding: try forwarded.document.sseFrame(), as: UTF8.self),
                       "data: \(String(decoding: json, as: UTF8.self))\n\n")
    }
}
