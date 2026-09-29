import AGUICore
import AGUIClient
import Foundation
import Network
import XCTest

final class ConsumerTests: XCTestCase {
    func testPublishedBoundedParserAndQueueFailClosed() async throws {
        let exact = Array("data: x\n\n".utf8)
        var parser = try BoundedSSEParser(maximumFrameBytes: exact.count)
        var frame: SseEvent?
        for byte in exact { if let event = try parser.feed(byte) { frame = event } }
        try parser.finish()
        XCTAssertEqual(frame?.data, "x")

        var tooSmall = try BoundedSSEParser(maximumFrameBytes: exact.count - 1)
        do {
            for byte in exact { _ = try tooSmall.feed(byte) }
            XCTFail("Parser must reject limit plus one")
        } catch {
            XCTAssertEqual(error as? BoundedSSEParser.Failure, .frameTooLarge)
        }

        let server = try FixtureServer()
        defer { server.stop() }
        do {
            let response = try await BoundedSSEClient().open(
                URLRequest(url: server.url("overflow")),
                maximumFrameBytes: 512, maximumQueuedBytes: 16)
            var iterator = response.events.makeAsyncIterator()
            _ = try await iterator.next()
            XCTFail("Queued bytes must overflow")
        } catch {
            XCTAssertEqual(error as? SSETransportFailure, .queuedBytesExceeded)
        }
    }

    func testRealLocalAgentcraftGETWatchAndGenericPOST() async throws {
        let server = try FixtureServer()
        defer { server.stop() }
        let client = BoundedSSEClient()

        for path in ["preadmission", "watch"] {
            var request = URLRequest(url: server.url(path))
            request.httpMethod = "GET"
            let response = try await client.open(request, maximumFrameBytes: 4096, maximumQueuedBytes: 8192)
            XCTAssertEqual(response.statusCode, 200)
            XCTAssertEqual(response.contentType, "text/event-stream")
            var frames: [SseEvent] = []
            for try await event in response.events { frames.append(event) }
            XCTAssertFalse(frames.isEmpty)
            XCTAssertEqual(frames.first?.id?.hasPrefix("MESSAGES_SNAPSHOT"), true)
            XCTAssertEqual(frames.last?.id?.hasPrefix("STATE_SNAPSHOT"), true)
            let decoded = try AGUIEventDecoder().decode(Data(try XCTUnwrap(frames.last).data.utf8))
            XCTAssertTrue(decoded is StateSnapshotEvent)
            if path == "watch" {
                let snapshot = try XCTUnwrap(decoded as? StateSnapshotEvent)
                XCTAssertTrue(String(decoding: snapshot.snapshot, as: UTF8.self).contains("\"Revision\":2"))
            }
        }

        var request = URLRequest(url: server.url("run"))
        request.httpMethod = "POST"
        request.httpBody = Data("{\"threadId\":\"consumer\"}".utf8)
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        let response = try await client.open(request, maximumFrameBytes: 1024, maximumQueuedBytes: 1024)
        var iterator = response.events.makeAsyncIterator()
        let next = try await iterator.next()
        let frame = try XCTUnwrap(next)
        XCTAssertEqual(frame.id, "post-1")
        XCTAssertTrue(try AGUIEventDecoder().decode(Data(frame.data.utf8)) is RunStartedEvent)
        let observed = server.requests()
        XCTAssertTrue(observed.contains("POST /run {\"threadId\":\"consumer\"}"))
    }

    func testRealLocalCancellationKeepsAnotherWatchAlive() async throws {
        let server = try FixtureServer()
        defer { server.stop() }
        let client = BoundedSSEClient()
        let stalled = try await client.open(
            URLRequest(url: server.url("stall")), maximumFrameBytes: 4096, maximumQueuedBytes: 8192)
        var iterator = stalled.events.makeAsyncIterator()
        let first = try await iterator.next()
        XCTAssertEqual(first?.event, "control")
        stalled.cancel()
        do {
            _ = try await iterator.next()
            XCTFail("Cancelled watch must terminate")
        } catch {
            XCTAssertEqual(error as? ClientError, .cancelled)
        }
        let other = try await client.open(
            URLRequest(url: server.url("preadmission")), maximumFrameBytes: 4096, maximumQueuedBytes: 8192)
        var count = 0
        for try await _ in other.events { count += 1 }
        XCTAssertEqual(count, 2)
    }

    func testRealLocalRedirectIsRejectedWithoutFollowingIt() async throws {
        let server = try FixtureServer()
        defer { server.stop() }
        do {
            _ = try await BoundedSSEClient().open(
                URLRequest(url: server.url("redirect")),
                maximumFrameBytes: 4096, maximumQueuedBytes: 8192)
            XCTFail("Redirect must be rejected")
        } catch {
            XCTAssertEqual(error as? ClientError, .httpError(statusCode: 302))
        }
        XCTAssertEqual(server.requests(), ["GET /redirect "])
    }
}

private final class FixtureServer: @unchecked Sendable {
    private let queue = DispatchQueue(label: "AGUIConsumerQualification.FixtureServer")
    private let listener: NWListener
    private let lock = NSLock()
    private var observed: [String] = []
    private var port: UInt16 = 0

    init() throws {
        listener = try NWListener(using: .tcp, on: .any)
        let ready = DispatchSemaphore(value: 0)
        listener.stateUpdateHandler = { state in
            if case .ready = state { ready.signal() }
            if case .failed = state { ready.signal() }
        }
        listener.newConnectionHandler = { [weak self] connection in
            self?.accept(connection)
        }
        listener.start(queue: queue)
        guard ready.wait(timeout: .now() + 5) == .success,
              let assigned = listener.port else {
            throw NSError(domain: "FixtureServer", code: 1)
        }
        port = assigned.rawValue
    }

    func url(_ path: String) -> URL {
        URL(string: "http://127.0.0.1:\(port)/\(path)")!
    }

    func requests() -> [String] {
        lock.lock(); defer { lock.unlock() }
        return observed
    }

    func stop() { listener.cancel() }

    private func accept(_ connection: NWConnection) {
        connection.start(queue: queue)
        receive(connection, accumulated: Data())
    }

    private func receive(_ connection: NWConnection, accumulated: Data) {
        connection.receive(minimumIncompleteLength: 1, maximumLength: 8192) { [weak self] data, _, complete, _ in
            guard let self else { connection.cancel(); return }
            var request = accumulated
            if let data { request.append(data) }
            guard request.count <= 65536 else { connection.cancel(); return }
            guard let boundary = request.range(of: Data("\r\n\r\n".utf8)) else {
                if complete { connection.cancel() }
                else { self.receive(connection, accumulated: request) }
                return
            }
            let header = String(decoding: request[..<boundary.lowerBound], as: UTF8.self)
            let contentLength = header.components(separatedBy: "\r\n").first {
                $0.lowercased().hasPrefix("content-length:")
            }.flatMap { Int($0.split(separator: ":", maxSplits: 1).last?.trimmingCharacters(in: .whitespaces) ?? "") } ?? 0
            guard request.count - boundary.upperBound >= contentLength else {
                self.receive(connection, accumulated: request)
                return
            }
            let first = header.components(separatedBy: "\r\n").first ?? ""
            let parts = first.split(separator: " ")
            let method = parts.first.map(String.init) ?? ""
            let path = parts.dropFirst().first.map(String.init) ?? ""
            let body = String(decoding: request[boundary.upperBound..<(boundary.upperBound + contentLength)], as: UTF8.self)
            self.lock.lock()
            self.observed.append("\(method) \(path) \(body)")
            self.lock.unlock()
            self.respond(connection, path: path)
        }
    }

    private func respond(_ connection: NWConnection, path: String) {
        if path == "/redirect" {
            let response = Data("HTTP/1.1 302 Found\r\nLocation: /watch\r\nContent-Length: 0\r\nConnection: close\r\n\r\n".utf8)
            connection.send(content: response, completion: .contentProcessed { _ in connection.cancel() })
            return
        }
        let body: Data
        switch path {
        case "/preadmission", "/watch":
            let name = path == "/watch" ? "sample_session" : "preadmission"
            body = try! Data(contentsOf: Bundle.module.url(forResource: name, withExtension: "sse", subdirectory: "Fixtures")!)
        case "/stall":
            body = Data("event: control\ndata: waiting\n\n".utf8)
        case "/overflow":
            body = Data(repeating: 65, count: 64)
        default:
            body = Data("id: post-1\ndata: {\"type\":\"RUN_STARTED\",\"threadId\":\"consumer\",\"runId\":\"run\"}\n\n".utf8)
        }
        let length = path == "/stall" ? "" : "Content-Length: \(body.count)\r\n"
        let headers = Data("HTTP/1.1 200 OK\r\nContent-Type: text/event-stream\r\n\(length)Connection: close\r\n\r\n".utf8)
        let midpoint = body.count / 2
        var first = headers
        first.append(body.prefix(midpoint))
        connection.send(content: first, completion: .contentProcessed { _ in
            self.queue.asyncAfter(deadline: .now() + 0.01) {
                connection.send(content: Data(body.suffix(from: midpoint)), completion: .contentProcessed { _ in
                    if path != "/stall" { connection.cancel() }
                })
            }
        })
    }
}
