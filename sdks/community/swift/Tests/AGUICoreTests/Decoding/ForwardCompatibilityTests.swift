import Foundation
import XCTest
@testable import AGUICore

final class ForwardCompatibilityTests: XCTestCase {
    private var decoder: AGUIEventDecoder {
        var configuration = AGUIEventDecoder.Configuration()
        configuration.enforceForwardCompatibility = true
        return AGUIEventDecoder(config: configuration)
    }

    func testSchemaCoversEveryWireEvent() {
        XCTAssertEqual(AGUISchemaDocument.sanitizableEventTypes, Set(EventType.allCases))
    }

    func testUnknownFieldsAreRemovedBeforeRegistryForTextAndStepStarts() throws {
        for input in [
            #"{"type":"TEXT_MESSAGE_START","messageId":"m","role":"assistant","future":1,"metadata":{"future":2}}"#,
            #"{"type":"STEP_STARTED","stepName":"work","future":1,"metadata":{"future":2}}"#
        ] {
            let event = try decoder.decode(Data(input.utf8))
            let raw = try XCTUnwrap(event.rawEvent)
            let object = try XCTUnwrap(JSONSerialization.jsonObject(with: raw) as? [String: Any])
            XCTAssertNil(object["future"])
            XCTAssertEqual((object["metadata"] as? [String: Int])?["future"], 2)
        }
    }

    func testMalformedKnownOptionalsFailEvenWithUnknownFields() {
        for input in [
            #"{"type":"TEXT_MESSAGE_START","messageId":"m","name":42,"future":1}"#,
            #"{"type":"STEP_STARTED","stepName":"work","subagentRunId":42,"future":1}"#,
            #"{"type":"STEP_STARTED","stepName":"work","timestamp":"now","future":1}"#
        ] {
            XCTAssertThrowsError(try decoder.decode(Data(input.utf8)), input)
        }
    }

    func testNestedClosedFieldsStripWhileOpenValuesSurvive() throws {
        let data = Data(#"{"type":"MESSAGES_SNAPSHOT","messages":[{"id":"m","role":"assistant","content":"hello","future":1,"metadata":{"future":2}}],"future":3}"#.utf8)
        let event = try decoder.decode(data)
        let object = try XCTUnwrap(JSONSerialization.jsonObject(with: XCTUnwrap(event.rawEvent)) as? [String: Any])
        XCTAssertNil(object["future"])
        let message = try XCTUnwrap((object["messages"] as? [[String: Any]])?.first)
        XCTAssertNil(message["future"])
        XCTAssertEqual((message["metadata"] as? [String: Int])?["future"], 2)
    }

    func testUnknownRequiredSourceDropsOnlyItsPart() throws {
        let data = Data(#"{"type":"MESSAGES_SNAPSHOT","messages":[{"id":"m","role":"user","content":[{"type":"text","text":"before"},{"type":"image","source":{"type":"hologram","value":"x"}},{"type":"text","text":"after"}]}]}"#.utf8)
        let event = try decoder.decode(data)
        let object = try XCTUnwrap(JSONSerialization.jsonObject(with: XCTUnwrap(event.rawEvent)) as? [String: Any])
        let message = try XCTUnwrap((object["messages"] as? [[String: Any]])?.first)
        let content = try XCTUnwrap(message["content"] as? [[String: Any]])
        XCTAssertEqual(content.compactMap { $0["text"] as? String }, ["before", "after"])
    }

    func testPatchOperationRemainsOpen() throws {
        let data = Data(#"{"type":"STATE_DELTA","delta":[{"op":"remove","path":"/x","value":"keep"}],"future":1}"#.utf8)
        let event = try decoder.decode(data)
        let object = try XCTUnwrap(JSONSerialization.jsonObject(with: XCTUnwrap(event.rawEvent)) as? [String: Any])
        XCTAssertNil(object["future"])
        XCTAssertEqual(((object["delta"] as? [[String: Any]])?.first)?["value"] as? String, "keep")
    }

    func testOpenStateNumberKeepsExactLexeme() throws {
        let digits = "1234567890123456789012345678901234567890"
        let event = try decoder.decode(Data("{\"type\":\"STATE_SNAPSHOT\",\"snapshot\":{\"value\":\(digits)},\"future\":1}".utf8))
        let raw = try XCTUnwrap(event.rawEvent)
        XCTAssertTrue(String(decoding: raw, as: UTF8.self).contains(digits))
        XCTAssertFalse(String(decoding: raw, as: UTF8.self).contains("future"))
    }
}
