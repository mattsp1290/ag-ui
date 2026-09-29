import Foundation

/// HTTP metadata and raw SSE frames. Decode `SseEvent.data` with AGUICore only
/// after the host has handled named control events such as Agentcraft's controls.
public struct SSEWatchResponse {
    public let statusCode: Int
    public let contentType: String
    public let events: BoundedSSEStream<URLSession.AsyncBytes>
}

private final class NoRedirects: NSObject, URLSessionTaskDelegate {
    func urlSession(
        _ session: URLSession, task: URLSessionTask,
        willPerformHTTPRedirection response: HTTPURLResponse,
        newRequest request: URLRequest,
        completionHandler: @escaping (URLRequest?) -> Void
    ) {
        completionHandler(nil)
    }
}

/// Opens arbitrary GET or POST SSE requests without RunAgentInput or lifecycle
/// validation. The provided URLSession remains owned by its caller.
public struct BoundedSSEClient {
    private let session: URLSession

    public init(session: URLSession = .shared) {
        self.session = session
    }

    public func open(_ request: URLRequest, maximumFrameBytes: Int) async throws -> SSEWatchResponse {
        guard maximumFrameBytes > 0 else { throw BoundedSSEParser.Failure.invalidLimit }
        let (bytes, response) = try await session.bytes(for: request, delegate: NoRedirects())
        guard let response = response as? HTTPURLResponse else { throw ClientError.invalidResponse }
        guard (200...299).contains(response.statusCode) else {
            throw ClientError.httpError(statusCode: response.statusCode)
        }
        let contentType = response.value(forHTTPHeaderField: "Content-Type") ?? ""
        guard contentType.lowercased().split(separator: ";", maxSplits: 1).first?.trimmingCharacters(in: .whitespaces) == "text/event-stream" else {
            throw ClientError.invalidResponse
        }
        return SSEWatchResponse(
            statusCode: response.statusCode,
            contentType: contentType,
            events: try BoundedSSEStream(bytes: bytes, maximumFrameBytes: maximumFrameBytes)
        )
    }
}
