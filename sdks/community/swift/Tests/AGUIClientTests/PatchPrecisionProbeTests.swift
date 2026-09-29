import AGUIClient
import Foundation
import XCTest

final class PatchPrecisionProbeTests: XCTestCase {
    func testPatchPreservesExistingAndInsertedLargeIntegers() throws {
        let existing = "1234567890123456789012345678901234567891234567890123456789"
        let inserted = "9876543210987654321098765432109876543210987654321098765432"
        let state = Data("{\"existing\":\(existing),\"inserted\":0}".utf8)
        let patch = Data("[{\"op\":\"replace\",\"path\":\"/inserted\",\"value\":\(inserted)}]".utf8)
        let output = try PatchApplicator().apply(patch: patch, to: state)
        let json = String(decoding: output, as: UTF8.self)
        XCTAssertTrue(json.contains(existing), "unchanged state integer was rounded: \(json)")
        XCTAssertTrue(json.contains(inserted), "patch integer was rounded: \(json)")
    }
}
