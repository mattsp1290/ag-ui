import XCTest
@testable import AGUIClient

final class BoundedSSEClientTests: XCTestCase {
    private func client() -> BoundedSSEClient {
        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [BoundedFixtureProtocol.self]
        return BoundedSSEClient(configuration: configuration)
    }

    func testGETAndPOSTExposeRawFramesAndRequestFields() async throws {
        let client = client()
        for method in ["GET", "POST"] {
            var request = URLRequest(url: URL(string: "https://fixture.test/normal")!)
            request.httpMethod = method
            request.setValue("watch-token", forHTTPHeaderField: "X-Watch")
            if method == "POST" { request.httpBody = Data("admission".utf8) }
            let response = try await client.open(request, maximumFrameBytes: 256, maximumQueuedBytes: 256)
            XCTAssertEqual(response.statusCode, 200)
            XCTAssertEqual(response.contentType, "text/event-stream; charset=utf-8")
            var frames: [SseEvent] = []
            for try await frame in response.events { frames.append(frame) }
            XCTAssertEqual(frames.count, 1)
            XCTAssertEqual(frames[0].event, "agentcraft-control")
            XCTAssertEqual(frames[0].id, method)
            XCTAssertEqual(frames[0].data, "watch-token|\(method == "POST" ? "admission" : "")")
        }
    }

    func testOversizedNetworkCallbackFailsWithoutDropping() async throws {
        for path in ["overflow", "many-small-chunks"] {
            let request = URLRequest(url: URL(string: "https://fixture.test/\(path)")!)
            do {
                let response = try await client().open(request, maximumFrameBytes: 512, maximumQueuedBytes: 16)
                var iterator = response.events.makeAsyncIterator()
                do {
                    _ = try await iterator.next()
                    XCTFail("Expected queue overflow")
                } catch {
                    XCTAssertEqual(error as? SSETransportFailure, .queuedBytesExceeded)
                }
            } catch {
                XCTAssertEqual(error as? SSETransportFailure, .queuedBytesExceeded)
            }
        }
    }

    func testCancelDuringHeaders() async throws {
        let opening = Task {
            try await client().open(
                URLRequest(url: URL(string: "https://fixture.test/wait-headers")!),
                maximumFrameBytes: 256, maximumQueuedBytes: 256)
        }
        opening.cancel()
        do {
            _ = try await opening.value
            XCTFail("Cancelled open must fail")
        } catch {
            XCTAssertEqual(error as? ClientError, .cancelled)
        }
    }

    func testStatusAndContentTypeAreValidated() async throws {
        for path in ["bad-status", "bad-type"] {
            let request = URLRequest(url: URL(string: "https://fixture.test/\(path)")!)
            do {
                _ = try await client().open(request, maximumFrameBytes: 256, maximumQueuedBytes: 256)
                XCTFail("Expected rejection for \(path)")
            } catch {
                if path == "bad-status" {
                    XCTAssertEqual(error as? ClientError, .httpError(statusCode: 302))
                } else {
                    XCTAssertEqual(error as? ClientError, .invalidResponse)
                }
            }
        }
    }

    func testCancelOneResponseLeavesAnotherUsable() async throws {
        let client = client()
        let stalled = try await client.open(
            URLRequest(url: URL(string: "https://fixture.test/stall")!),
            maximumFrameBytes: 256, maximumQueuedBytes: 256)
        let normal = try await client.open(
            URLRequest(url: URL(string: "https://fixture.test/normal")!),
            maximumFrameBytes: 256, maximumQueuedBytes: 256)
        stalled.cancel()
        var stalledIterator = stalled.events.makeAsyncIterator()
        do {
            _ = try await stalledIterator.next()
            XCTFail("Cancelled stream must fail")
        } catch {
            XCTAssertEqual(error as? ClientError, .cancelled)
        }
        var normalIterator = normal.events.makeAsyncIterator()
        let next = try await normalIterator.next()
        XCTAssertEqual(next?.event, "agentcraft-control")
    }
}

private final class BoundedFixtureProtocol: URLProtocol, @unchecked Sendable {
    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }

    override func startLoading() {
        let path = request.url!.lastPathComponent
        if path == "wait-headers" { return }
        let status = path == "bad-status" ? 302 : 200
        let type = path == "bad-type" ? "text/plain" : "text/event-stream; charset=utf-8"
        let response = HTTPURLResponse(
            url: request.url!, statusCode: status, httpVersion: "HTTP/1.1",
            headerFields: ["Content-Type": type])!
        client?.urlProtocol(self, didReceive: response, cacheStoragePolicy: .notAllowed)
        if path == "stall" { return }
        if path == "overflow" {
            client?.urlProtocol(self, didLoad: Data(repeating: 65, count: 64))
        } else if path == "many-small-chunks" {
            for _ in 0..<64 { client?.urlProtocol(self, didLoad: Data([65])) }
        } else if path == "normal" {
            let method = request.httpMethod ?? "GET"
            var body = request.httpBody.flatMap { String(data: $0, encoding: .utf8) } ?? ""
            if body.isEmpty, let stream = request.httpBodyStream {
                stream.open()
                defer { stream.close() }
                var buffer = [UInt8](repeating: 0, count: 128)
                let count = stream.read(&buffer, maxLength: buffer.count)
                if count > 0 { body = String(decoding: buffer.prefix(count), as: UTF8.self) }
            }
            let payload = "event: agentcraft-control\nid: \(method)\ndata: \(request.value(forHTTPHeaderField: "X-Watch") ?? "")|\(body)\n\n"
            client?.urlProtocol(self, didLoad: Data(payload.utf8))
        }
        client?.urlProtocolDidFinishLoading(self)
    }

    override func stopLoading() {}
}
