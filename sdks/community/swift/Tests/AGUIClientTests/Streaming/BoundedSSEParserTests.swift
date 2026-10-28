import XCTest
@testable import AGUIClient
import AGUICore

final class BoundedSSEParserTests: XCTestCase {
    private func parse(_ bytes: [UInt8], limit: Int = 4096) throws -> [SseEvent] {
        var parser = try BoundedSSEParser(maximumFrameBytes: limit)
        var events: [SseEvent] = []
        for byte in bytes {
            if let event = try parser.feed(byte) { events.append(event) }
        }
        try parser.finish()
        return events
    }

    func testEveryByteBoundaryWithBOMUnicodeAndMixedLineEndings() throws {
        let input = Array("\u{FEFF}event: control\r\nid: 7\rdata: café\ndata: two\r\nretry: 100\r\n\r\n".utf8)
        let event = try XCTUnwrap(parse(input).first)
        XCTAssertEqual(event.event, "control")
        XCTAssertEqual(event.id, "7")
        XCTAssertEqual(event.retry, 100)
        XCTAssertEqual(event.data, "café\ntwo")
    }

    func testExactLimitAndLimitPlusOne() throws {
        let bytes = Array("data: x\n\n".utf8)
        XCTAssertEqual(try parse(bytes, limit: bytes.count).first?.data, "x")
        XCTAssertThrowsError(try parse(bytes, limit: bytes.count - 1)) { error in
            XCTAssertEqual(error as? BoundedSSEParser.Failure, .frameTooLarge)
        }
        let crlf = Array("data: x\r\n\r\n".utf8)
        XCTAssertEqual(try parse(crlf, limit: crlf.count - 1).first?.data, "x")
        XCTAssertThrowsError(try parse(crlf, limit: crlf.count - 2)) { error in
            XCTAssertEqual(error as? BoundedSSEParser.Failure, .frameTooLarge)
        }
    }

    func testLargeSingleChunkAndSmallChunksBothFailClosed() throws {
        for prefix in [":", "data: ", "unknown: "] {
            let bytes = Array((prefix + String(repeating: "x", count: 200) + "\n\n").utf8)
            XCTAssertThrowsError(try parse(bytes, limit: 32)) { error in
                XCTAssertEqual(error as? BoundedSSEParser.Failure, .frameTooLarge)
            }
        }
        XCTAssertThrowsError(try parse(Array("data: x\n".utf8))) { error in
            XCTAssertEqual(error as? BoundedSSEParser.Failure, .truncatedFrame)
        }
        XCTAssertThrowsError(try parse([0xff, 10, 10])) { error in
            XCTAssertEqual(error as? BoundedSSEParser.Failure, .invalidUTF8)
        }
    }

    func testMultilineDataAggregateOverflowsEvenWhenEachLineFits() throws {
        let wire = Array("data:a\ndata:b\ndata:c\n\n".utf8)
        XCTAssertThrowsError(try parse(wire, limit: 20)) { error in
            XCTAssertEqual(error as? BoundedSSEParser.Failure, .frameTooLarge)
        }
    }

    func testMalformedJSONRemainsRawUntilTypedDecode() throws {
        let wire = Array("event: MESSAGES_SNAPSHOT\ndata: {bad}\n\n".utf8)
        let frame = try XCTUnwrap(parse(wire).first)
        XCTAssertEqual(frame.event, "MESSAGES_SNAPSHOT")
        XCTAssertEqual(frame.data, "{bad}")
        XCTAssertThrowsError(try AGUIEventDecoder().decode(Data(frame.data.utf8)))
    }

    func testAgentcraftFixturesStayRawAndPreserveSnapshotNumbers() throws {
        let directory = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent()
            .appendingPathComponent("Fixtures")
        for name in ["agentcraft-preadmission", "agentcraft-sample-session"] {
            let bytes = try [UInt8](Data(contentsOf: directory.appendingPathComponent("\(name).sse")))
            let events = try parse(bytes, limit: 4096)
            XCTAssertFalse(events.isEmpty)
            XCTAssertTrue(events.allSatisfy { $0.id != nil && $0.event == "message" })
            let types = try events.map { event -> String in
                let json = try XCTUnwrap(JSONSerialization.jsonObject(with: Data(event.data.utf8)) as? [String: Any])
                return try XCTUnwrap(json["type"] as? String)
            }
            XCTAssertEqual(types.count % 2, 0)
            XCTAssertEqual(types.first, "MESSAGES_SNAPSHOT")
            XCTAssertEqual(types.last, "STATE_SNAPSHOT")
        }
    }

    func testSnapshotRevisionAboveDoublePrecisionSurvivesTypedDecode() throws {
        let wire = "data: {\"type\":\"STATE_SNAPSHOT\",\"snapshot\":{\"Watermark\":{\"Revision\":9007199254740993}}}\n\n"
        let frame = try XCTUnwrap(parse(Array(wire.utf8)).first)
        let decoded = try AGUIEventDecoder().decode(Data(frame.data.utf8))
        let snapshot = try XCTUnwrap(decoded as? StateSnapshotEvent)
        XCTAssertTrue(String(decoding: snapshot.snapshot, as: UTF8.self).contains("9007199254740993"))
    }
}
