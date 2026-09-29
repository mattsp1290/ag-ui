import Foundation

/// Errors do not include response bodies or stream contents.
public enum SSETransportFailure: Error, Equatable, Sendable {
    case invalidLimit
    case queuedBytesExceeded
    case alreadyConsumed
}

/// HTTP metadata and raw SSE frames. Handle named control events before decoding
/// AG-UI JSON with `AGUIEventDecoder`; this layer does not verify run lifecycle.
public struct SSEWatchResponse {
    public let statusCode: Int
    public let contentType: String
    public let events: BoundedSSEStream<BoundedHTTPBytes>
    private let connection: SSEByteConnection

    fileprivate init(statusCode: Int, contentType: String, events: BoundedSSEStream<BoundedHTTPBytes>, connection: SSEByteConnection) {
        self.statusCode = statusCode
        self.contentType = contentType
        self.events = events
        self.connection = connection
    }

    /// Cancel this response's task and release its dedicated URLSession.
    public func cancel() { connection.cancel() }
}

/// A single-consumer byte sequence whose in-process queue has a strict limit.
/// An incoming callback larger than the available capacity terminates the task.
public struct BoundedHTTPBytes: AsyncSequence {
    public typealias Element = UInt8
    private let connection: SSEByteConnection

    fileprivate init(connection: SSEByteConnection) { self.connection = connection }

    public func makeAsyncIterator() -> Iterator {
        Iterator(connection: connection, permitted: connection.claimIterator())
    }

    public struct Iterator: AsyncIteratorProtocol {
        private let connection: SSEByteConnection
        private let permitted: Bool

        fileprivate init(connection: SSEByteConnection, permitted: Bool) {
            self.connection = connection
            self.permitted = permitted
        }

        public mutating func next() async throws -> UInt8? {
            guard permitted else { throw SSETransportFailure.alreadyConsumed }
            return try await connection.nextByte()
        }
    }
}

/// Opens arbitrary GET or POST SSE requests without `RunAgentInput`. Each open
/// request receives a separate URLSession; cancellation cannot invalidate a
/// caller-owned networking session or another response. The host supplies the
/// method, URL, headers and body in a URLRequest.
public struct BoundedSSEClient {
    private let configuration: URLSessionConfiguration

    public init(configuration: URLSessionConfiguration = .ephemeral) {
        self.configuration = configuration
    }

    public func open(
        _ request: URLRequest,
        maximumFrameBytes: Int,
        maximumQueuedBytes: Int
    ) async throws -> SSEWatchResponse {
        guard maximumFrameBytes > 0, maximumQueuedBytes > 0 else {
            throw SSETransportFailure.invalidLimit
        }
        let connection = SSEByteConnection(maximumQueuedBytes: maximumQueuedBytes)
        connection.start(request: request, configuration: configuration)
        let response = try await connection.response()
        if Task.isCancelled {
            connection.cancel()
            throw ClientError.cancelled
        }
        guard (200...299).contains(response.statusCode) else {
            connection.cancel()
            throw ClientError.httpError(statusCode: response.statusCode)
        }
        let contentType = response.value(forHTTPHeaderField: "Content-Type") ?? ""
        guard contentType.lowercased().split(separator: ";", maxSplits: 1).first?
            .trimmingCharacters(in: .whitespaces) == "text/event-stream" else {
            connection.cancel()
            throw ClientError.invalidResponse
        }
        return SSEWatchResponse(
            statusCode: response.statusCode,
            contentType: contentType,
            events: try BoundedSSEStream(
                bytes: BoundedHTTPBytes(connection: connection),
                maximumFrameBytes: maximumFrameBytes
            ),
            connection: connection
        )
    }
}

/// URLSession callbacks enqueue synchronously under a lock. Creating a Task per
/// callback would itself be an unbounded queue, so none is used.
private final class SSEByteConnection: NSObject, URLSessionDataDelegate, @unchecked Sendable {
    private let lock = NSLock()
    private let maximumQueuedBytes: Int
    private var session: URLSession?
    private var task: URLSessionDataTask?
    private var responseValue: HTTPURLResponse?
    private var responseWaiter: CheckedContinuation<HTTPURLResponse, Error>?
    private var chunks: [Data] = []
    private var offset = 0
    private var queuedBytes = 0
    private var byteWaiter: CheckedContinuation<UInt8?, Error>?
    private var completion: Result<Void, Error>?
    private var iteratorClaimed = false

    init(maximumQueuedBytes: Int) {
        self.maximumQueuedBytes = maximumQueuedBytes
    }

    func start(request: URLRequest, configuration: URLSessionConfiguration) {
        let session = URLSession(configuration: configuration, delegate: self, delegateQueue: nil)
        let task = session.dataTask(with: request)
        lock.lock()
        self.session = session
        self.task = task
        lock.unlock()
        task.resume()
    }

    func claimIterator() -> Bool {
        lock.lock(); defer { lock.unlock() }
        guard !iteratorClaimed else { return false }
        iteratorClaimed = true
        return true
    }

    func response() async throws -> HTTPURLResponse {
        try await withTaskCancellationHandler {
            try await withCheckedThrowingContinuation { continuation in
                lock.lock()
                if let completion, case .failure(let error) = completion {
                    lock.unlock()
                    continuation.resume(throwing: error)
                } else if let responseValue {
                    lock.unlock()
                    continuation.resume(returning: responseValue)
                } else if let completion {
                    lock.unlock()
                    switch completion {
                    case .success: continuation.resume(throwing: ClientError.invalidResponse)
                    case .failure(let error): continuation.resume(throwing: error)
                    }
                } else {
                    responseWaiter = continuation
                    lock.unlock()
                }
            }
        } onCancel: { cancel() }
    }

    func nextByte() async throws -> UInt8? {
        try await withTaskCancellationHandler {
            try await withCheckedThrowingContinuation { continuation in
                lock.lock()
                if let byte = popByteLocked() {
                    lock.unlock()
                    continuation.resume(returning: byte)
                } else if let completion {
                    lock.unlock()
                    switch completion {
                    case .success: continuation.resume(returning: nil)
                    case .failure(let error): continuation.resume(throwing: error)
                    }
                } else if byteWaiter != nil {
                    lock.unlock()
                    continuation.resume(throwing: SSETransportFailure.alreadyConsumed)
                } else {
                    byteWaiter = continuation
                    lock.unlock()
                }
            }
        } onCancel: { cancel() }
    }

    private func popByteLocked() -> UInt8? {
        guard !chunks.isEmpty else { return nil }
        let byte = chunks[0][offset]
        offset += 1
        queuedBytes -= 1
        if offset == chunks[0].count {
            chunks.removeFirst()
            offset = 0
        }
        return byte
    }

    private func finish(_ result: Result<Void, Error>) {
        lock.lock()
        guard completion == nil else { lock.unlock(); return }
        completion = result
        if case .failure = result {
            chunks.removeAll()
            queuedBytes = 0
            offset = 0
        }
        let responseWaiter = self.responseWaiter
        self.responseWaiter = nil
        let byteWaiter = self.byteWaiter
        self.byteWaiter = nil
        let session = self.session
        self.session = nil
        self.task = nil
        lock.unlock()

        if let responseWaiter {
            switch result {
            case .success: responseWaiter.resume(throwing: ClientError.invalidResponse)
            case .failure(let error): responseWaiter.resume(throwing: error)
            }
        }
        if let byteWaiter {
            switch result {
            case .success: byteWaiter.resume(returning: nil)
            case .failure(let error): byteWaiter.resume(throwing: error)
            }
        }
        session?.invalidateAndCancel()
    }

    func cancel() {
        lock.lock()
        let task = self.task
        lock.unlock()
        task?.cancel()
        finish(.failure(ClientError.cancelled))
    }

    func urlSession(
        _ session: URLSession, task: URLSessionTask,
        willPerformHTTPRedirection response: HTTPURLResponse,
        newRequest request: URLRequest,
        completionHandler: @escaping (URLRequest?) -> Void
    ) {
        completionHandler(nil)
    }

    func urlSession(
        _ session: URLSession, dataTask: URLSessionDataTask,
        didReceive response: URLResponse,
        completionHandler: @escaping (URLSession.ResponseDisposition) -> Void
    ) {
        guard let http = response as? HTTPURLResponse else {
            completionHandler(.cancel)
            finish(.failure(ClientError.invalidResponse))
            return
        }
        lock.lock()
        if completion != nil {
            lock.unlock()
            completionHandler(.cancel)
            return
        }
        responseValue = http
        let waiter = responseWaiter
        responseWaiter = nil
        lock.unlock()
        waiter?.resume(returning: http)
        completionHandler(.allow)
    }

    func urlSession(_ session: URLSession, dataTask: URLSessionDataTask, didReceive data: Data) {
        lock.lock()
        guard completion == nil else { lock.unlock(); return }
        guard !data.isEmpty else { lock.unlock(); return }
        if data.count > maximumQueuedBytes - queuedBytes {
            lock.unlock()
            dataTask.cancel()
            finish(.failure(SSETransportFailure.queuedBytesExceeded))
            return
        }
        if !data.isEmpty {
            chunks.append(data)
            queuedBytes += data.count
        }
        let waiter = byteWaiter
        byteWaiter = nil
        let byte = waiter == nil ? nil : popByteLocked()
        lock.unlock()
        if let waiter, let byte { waiter.resume(returning: byte) }
    }

    func urlSession(_ session: URLSession, task: URLSessionTask, didCompleteWithError error: Error?) {
        if let error { finish(.failure(error)) }
        else { finish(.success(())) }
    }
}
