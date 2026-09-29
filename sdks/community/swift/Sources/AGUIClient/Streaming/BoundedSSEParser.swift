import Foundation

/// A byte-oriented SSE parser. Each incomplete frame, including comments and unknown
/// fields, has a finite byte limit. A caller feeds one byte at a time so no input
/// chunk or output event queue is retained by this parser.
public struct BoundedSSEParser {
    public enum Failure: Error, Equatable, Sendable {
        case invalidLimit
        case frameTooLarge
        case invalidUTF8
        case truncatedFrame
    }

    public let maximumFrameBytes: Int
    private var line: [UInt8] = []
    private var frameBytes = 0
    private var dataLines: [String] = []
    private var eventName: String?
    private var eventID: String?
    private var retry: Int?
    private var sawCR = false
    private var firstLine = true
    private var hasFrameContent = false

    public init(maximumFrameBytes: Int) throws {
        guard maximumFrameBytes > 0 else { throw Failure.invalidLimit }
        self.maximumFrameBytes = maximumFrameBytes
    }

    /// At most `maximumFrameBytes` of source bytes plus decoded frame strings
    /// are retained. UTF-8 decoding can temporarily duplicate a line.
    public mutating func feed(_ byte: UInt8) throws -> SseEvent? {
        if sawCR {
            sawCR = false
            if byte == 10 {
                if frameBytes > 0 {
                    guard frameBytes < maximumFrameBytes else { throw Failure.frameTooLarge }
                    frameBytes += 1
                }
                return nil
            }
        }
        guard frameBytes < maximumFrameBytes else { throw Failure.frameTooLarge }
        frameBytes += 1
        if byte == 13 || byte == 10 {
            if byte == 13 { sawCR = true }
            return try endLine()
        }
        line.append(byte)
        return nil
    }

    /// EOF after an unterminated line or frame is an error, never a silent drop.
    public mutating func finish() throws {
        if !line.isEmpty || hasFrameContent { throw Failure.truncatedFrame }
    }

    private mutating func endLine() throws -> SseEvent? {
        var bytes = line
        line.removeAll(keepingCapacity: true)
        if firstLine {
            firstLine = false
            if bytes.starts(with: [0xEF, 0xBB, 0xBF]) { bytes.removeFirst(3) }
        }
        guard let text = String(bytes: bytes, encoding: .utf8) else { throw Failure.invalidUTF8 }
        if text.isEmpty {
            let result: SseEvent? = dataLines.isEmpty ? nil : SseEvent(
                data: dataLines.joined(separator: "\n"), id: eventID,
                event: eventName.flatMap { $0.isEmpty ? nil : $0 } ?? "message", retry: retry)
            dataLines.removeAll(keepingCapacity: true)
            eventName = nil
            eventID = nil
            retry = nil
            frameBytes = 0
            hasFrameContent = false
            return result
        }
        hasFrameContent = true
        if text.first == ":" { return nil }
        let pieces = text.split(separator: ":", maxSplits: 1, omittingEmptySubsequences: false)
        let field = String(pieces[0])
        var value = pieces.count == 2 ? String(pieces[1]) : ""
        if value.first == " " { value.removeFirst() }
        switch field {
        case "data": dataLines.append(value)
        case "event": eventName = value
        case "id": if !value.contains("\0") { eventID = value }
        case "retry": if !value.isEmpty && value.utf8.allSatisfy({ $0 >= 48 && $0 <= 57 }) { retry = Int(value) }
        default: break
        }
        return nil
    }
}
