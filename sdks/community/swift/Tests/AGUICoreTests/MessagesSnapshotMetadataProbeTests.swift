import Foundation
import XCTest
@testable import AGUICore

final class MessagesSnapshotMetadataProbeTests: XCTestCase {
    func testAssistantSnapshotPreservesOpenMetadataInDomainMessage() throws {
        let wire = Data("{\"type\":\"MESSAGES_SNAPSHOT\",\"messages\":[{\"id\":\"m\",\"role\":\"assistant\",\"content\":\"x\",\"metadata\":{\"provider\":\"acme\"}}]}".utf8)
        let event = try XCTUnwrap(try AGUIEventDecoder().decode(wire) as? MessagesSnapshotEvent)
        let assistant = try XCTUnwrap(event.messages.first as? AssistantMessage)
        let metadata = try XCTUnwrap(assistant.metadata)
        XCTAssertTrue(String(decoding: metadata, as: UTF8.self).contains("acme"))
    }
}
