import Foundation

/// Pull-based SSE stream. It does not launch a producer task or buffer events:
/// each `next()` reads only until the next complete frame. This lets a slow
/// consumer apply backpressure all the way to the underlying byte iterator.
public struct BoundedSSEStream<Bytes: AsyncSequence>: AsyncSequence where Bytes.Element == UInt8 {
    public typealias Element = SseEvent
    private let bytes: Bytes
    private let maximumFrameBytes: Int

    public init(bytes: Bytes, maximumFrameBytes: Int) throws {
        guard maximumFrameBytes > 0 else { throw BoundedSSEParser.Failure.invalidLimit }
        self.bytes = bytes
        self.maximumFrameBytes = maximumFrameBytes
    }

    public func makeAsyncIterator() -> Iterator {
        Iterator(bytes: bytes.makeAsyncIterator(), parser: try! BoundedSSEParser(maximumFrameBytes: maximumFrameBytes))
    }

    public struct Iterator: AsyncIteratorProtocol {
        private var bytes: Bytes.AsyncIterator
        private var parser: BoundedSSEParser

        fileprivate init(bytes: Bytes.AsyncIterator, parser: BoundedSSEParser) {
            self.bytes = bytes
            self.parser = parser
        }

        public mutating func next() async throws -> SseEvent? {
            while let byte = try await bytes.next() {
                try Task.checkCancellation()
                if let event = try parser.feed(byte) { return event }
            }
            try parser.finish()
            return nil
        }
    }
}
