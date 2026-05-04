/// Stream adapter for converting SSE messages to typed AG-UI events.
library;

import 'dart:async';

import '../client/errors.dart';
import '../events/events.dart';
import '../sse/sse_message.dart';
import '../types/base.dart';
import 'decoder.dart';

/// Adapter for converting streams of SSE messages to typed AG-UI events.
///
/// This class provides utilities to:
/// - Convert SSE message streams to typed event streams
/// - Handle partial messages and buffering
/// - Filter and transform events
/// - Handle errors gracefully
class EventStreamAdapter {
  final EventDecoder _decoder;

  /// Creates a new stream adapter with an optional custom decoder.
  ///
  /// SSE line-buffering state for [fromRawSseStream] lives in locals scoped
  /// to each invocation, not on the adapter instance. This means the same
  /// adapter can safely process multiple streams sequentially or
  /// concurrently — abnormal termination of one stream cannot leak partial
  /// `data:` payloads or a stale `inDataBlock` flag into the next.
  EventStreamAdapter({EventDecoder? decoder})
      : _decoder = decoder ?? const EventDecoder();
  
  /// Adapts JSON data to AG-UI events.
  ///
  /// Returns a list of events parsed from the JSON data.
  /// If the JSON is a single event, returns a list with one event.
  /// If the JSON is an array of events, returns all events.
  List<BaseEvent> adaptJsonToEvents(dynamic jsonData) {
    try {
      if (jsonData is Map<String, dynamic>) {
        // Single event
        return [_decoder.decodeJson(jsonData)];
      } else if (jsonData is List) {
        // Array of events
        final events = <BaseEvent>[];
        for (var i = 0; i < jsonData.length; i++) {
          final element = jsonData[i];
          if (element is! Map<String, dynamic>) {
            // Reject non-object elements explicitly so a list with a
            // primitive or non-record entry produces a structured error
            // naming the bad index, rather than silently skipping or
            // throwing a `TypeError` swallowed by the catch-all below.
            throw DecodingError(
              'Expected JSON object at index $i',
              field: 'jsonData[$i]',
              expectedType: 'Map<String, dynamic>',
              actualValue: element,
            );
          }
          try {
            events.add(_decoder.decodeJson(element));
          } catch (e) {
            throw DecodingError(
              'Failed to decode event at index $i',
              field: 'jsonData[$i]',
              expectedType: 'BaseEvent',
              actualValue: element,
              cause: e,
            );
          }
        }
        return events;
      } else {
        throw DecodingError(
          'Invalid JSON data type',
          field: 'jsonData',
          expectedType: 'Map<String, dynamic> or List',
          actualValue: jsonData,
        );
      }
    } on AgUiError {
      rethrow;
    } catch (e) {
      throw DecodingError(
        'Failed to adapt JSON to events',
        field: 'jsonData',
        expectedType: 'BaseEvent or List<BaseEvent>',
        actualValue: jsonData,
        cause: e,
      );
    }
  }

  /// Converts a stream of SSE messages to a stream of typed AG-UI events.
  ///
  /// This method handles:
  /// - Decoding SSE data fields to JSON
  /// - Parsing JSON to typed event objects
  /// - Filtering out non-data messages (comments, etc.)
  /// - Error handling with optional recovery
  ///
  /// When [skipInvalidEvents] is `true`, decode failures (malformed JSON,
  /// unknown event types, validation errors) are routed to [onError] and
  /// the stream continues. This includes silent loss of any
  /// `REASONING_ENCRYPTED_VALUE` event whose `subtype` is unknown to this
  /// SDK version: there is no sensible default for an encrypted-payload
  /// subtype, so the event becomes a `DecodingError` and is dropped under
  /// the flag. Most other enums (`ReasoningMessageRole`, `TextMessageRole`)
  /// absorb unknown values at the event-decoding boundary instead.
  /// Consumers that need to react to such drops should observe [onError].
  Stream<BaseEvent> fromSseStream(
    Stream<SseMessage> sseStream, {
    bool skipInvalidEvents = false,
    void Function(Object error, StackTrace stackTrace)? onError,
  }) {
    return sseStream.transform(
      StreamTransformer<SseMessage, BaseEvent>.fromHandlers(
        handleData: (message, sink) {
          try {
            // Only process data messages
            final data = message.data;
            if (data != null && data.isNotEmpty) {
              // Skip keep-alive sentinels (data field whose trimmed value is
              // `:`) silently. This differs from `decodeSSE` / `flushDataBlock`
              // in `fromRawSseStream`, which route keep-alives through
              // `onError` / `skipInvalidEvents`. The distinction is intentional:
              // `fromSseStream` receives pre-parsed `SseMessage` objects where
              // keep-alive detection must run on `data`, while `fromRawSseStream`
              // and `decodeSSE` operate on the raw SSE text where the `:` comment
              // line is a distinct field. Both paths ultimately discard the
              // keep-alive; only the routing path differs.
              if (data.trim() == ':') {
                return;
              }
              
              // `decode` already runs `validate` via `decodeJson`; no
              // second pass needed here.
              sink.add(_decoder.decode(data));
            }
            // Ignore non-data messages (id, event, retry, comments)
          } catch (e, stack) {
            // Preserve any `AGUIError` subtype (covers `AgUiError`,
            // `AGUIValidationError`, and `EncoderError` siblings) so the
            // unified error-surface contract documented on `EventDecoder`
            // is not undone by re-wrapping at the stream-adapter layer.
            final error = e is AGUIError ? e : DecodingError(
              'Failed to process SSE message',
              field: 'message',
              expectedType: 'BaseEvent',
              actualValue: message.data,
              cause: e,
            );

            if (skipInvalidEvents) {
              // Log error but continue processing
              onError?.call(error, stack);
            } else {
              // Propagate error to stream
              sink.addError(error, stack);
            }
          }
        },
        handleError: (error, stack, sink) {
          if (skipInvalidEvents) {
            // Log error but continue processing
            onError?.call(error, stack);
          } else {
            // Propagate error to stream
            sink.addError(error, stack);
          }
        },
      ),
    );
  }

  /// Converts a stream of raw SSE strings to typed AG-UI events.
  ///
  /// This handles partial messages that may be split across multiple
  /// stream events, buffering as needed.
  ///
  /// Line terminators: per the WHATWG SSE spec, `\r\n`, lone `\n`, and
  /// lone `\r` are all valid. This implementation supports all three.
  /// A trailing `\r` at the end of a chunk is deferred to the next chunk
  /// to disambiguate from a chunk-spanning `\r\n`; on stream close the
  /// deferred `\r` is consumed as a complete lone-CR terminator.
  ///
  /// See [fromSseStream] for the [skipInvalidEvents] / [onError]
  /// semantics, including the silent-drop note for
  /// `REASONING_ENCRYPTED_VALUE` events with unknown subtypes.
  ///
  /// Edge case on abnormal termination: when the stream ends mid-line
  /// (no trailing terminator) AND the partial line in the buffer is NOT
  /// `data:`-prefixed (e.g. it is `event:`, `id:`, `retry:`, a `:`-comment,
  /// or an in-progress continuation of a multi-line `data:` block), that
  /// partial line is silently dropped. Steady-state SSE parsing already
  /// ignores those lines per the spec; the drop only affects truly
  /// abnormal close-without-newline cases. A trailing `data:`-prefixed
  /// partial line, by contrast, is flushed and decoded.
  Stream<BaseEvent> fromRawSseStream(
    Stream<String> rawStream, {
    bool skipInvalidEvents = false,
    void Function(Object error, StackTrace stackTrace)? onError,
  }) {
    final controller = StreamController<BaseEvent>(sync: true);

    // Per-invocation state. Keeping these local (not instance fields)
    // ensures abnormal termination of one stream cannot leak partial
    // `data:` payloads or a stale `inDataBlock` flag into a subsequent
    // invocation on the same adapter.
    final buffer = StringBuffer();
    final dataBuffer = StringBuffer();
    var inDataBlock = false;
    // Tracks whether the last terminator seen across ALL prior chunks was a
    // lone CR. Persisting this across processChunk calls lets _scanLines
    // skip the trailing-\r deferral for producers that use lone-CR style
    // and deliver each terminator in its own chunk — without persistence the
    // flag resets to false on every call, adding a full chunk-RTT of latency
    // per event. See Important #II2 (review-fix pass).
    var lastWasLoneCr = false;

    // Append the value portion of a `data:` or `data: ` line to the
    // active data block. Lines that aren't `data:`-prefixed are silently
    // ignored per the WHATWG SSE spec (event:, id:, retry:, comments).
    // Closes over `dataBuffer` and `inDataBlock` so the per-line loop
    // and the `onDone` final flush share the same logic.
    void appendDataLine(String line) {
      String value;
      if (line.startsWith('data: ')) {
        value = line.substring(6);
      } else if (line.startsWith('data:')) {
        value = line.substring(5);
      } else {
        return; // Not a data line — ignore per spec.
      }
      if (inDataBlock) {
        // Multi-line data: add newline between lines per spec.
        dataBuffer.write('\n');
        dataBuffer.write(value);
      } else {
        dataBuffer.clear();
        dataBuffer.write(value);
        inDataBlock = true;
      }
    }

    // Flush the accumulated data block as a single decoded event.
    // Used by the empty-line dispatch and the `onDone` final flush.
    void flushDataBlock() {
      if (!inDataBlock) return;
      final data = dataBuffer.toString();
      dataBuffer.clear();
      inDataBlock = false;

      if (data.isEmpty || data.trim() == ':') return;

      try {
        // `decode` already runs `validate` via `decodeJson`; no
        // second pass needed here.
        controller.add(_decoder.decode(data));
      } catch (e, stack) {
        // Preserve any `AGUIError` subtype (`AgUiError`,
        // `AGUIValidationError`, `EncoderError`) so the unified
        // error-surface contract from `EventDecoder` is not undone by
        // re-wrapping here. Only foreign exceptions become a generic
        // `DecodingError`.
        final error = e is AGUIError
            ? e
            : DecodingError(
                'Failed to decode SSE data',
                field: 'data',
                expectedType: 'BaseEvent',
                actualValue: data,
                cause: e,
              );

        if (!skipInvalidEvents) {
          controller.addError(error, stack);
        } else {
          onError?.call(error, stack);
        }
      }
    }

    void processChunk(String chunk) {
      // Add chunk to buffer to handle partial lines.
      buffer.write(chunk);

      // Multi-terminator scan: see [_scanLines] for the spec rationale.
      // `endOfStream: false` defers a trailing `\r` so a chunk-spanning
      // `\r\n` doesn't double-fire as two empty lines.
      // Pass `lastWasLoneCrAtStart` so the flag survives chunk boundaries
      // and capture the updated value for the next call.
      final scan = _scanLines(
        buffer.toString(),
        endOfStream: false,
        lastWasLoneCrAtStart: lastWasLoneCr,
      );
      lastWasLoneCr = scan.lastWasLoneCr;
      buffer.clear();
      buffer.write(scan.unconsumed);

      for (final line in scan.lines) {
        if (line.isEmpty) {
          // Empty line signals end of SSE message — flush the data block.
          flushDataBlock();
        } else {
          appendDataLine(line);
        }
      }
    }

    // Defer the upstream subscription to `onListen` so a caller that
    // obtains the returned stream but never subscribes does not leak the
    // upstream connection. Without deferral, `rawStream.listen(...)` fires
    // immediately on the `fromRawSseStream` call — a caller that stores the
    // stream for later or abandons it would keep the upstream alive until the
    // server closes the SSE connection. Mirroring the standard Dart lazy-
    // subscription idiom also makes the backpressure propagation below
    // consistent: `onCancel` only fires after `onListen`, so `subscription`
    // is always initialized by the time any lifecycle callback runs.
    StreamSubscription<String>? subscription;

    controller.onListen = () {
      subscription = rawStream.listen(
        (chunk) {
          try {
            processChunk(chunk);
          } catch (e, stack) {
            if (!skipInvalidEvents) {
              controller.addError(e, stack);
            } else {
              onError?.call(e, stack);
            }
          }
        },
        onError: (Object error, StackTrace stack) {
          if (!skipInvalidEvents) {
            controller.addError(error, stack);
          } else {
            onError?.call(error, stack);
          }
        },
        onDone: () {
          // End-of-stream: any deferred trailing `\r` is now a complete
          // terminator. Run the scanner with `endOfStream: true` to
          // consume it (and any other complete lines still in the buffer).
          final scan = _scanLines(buffer.toString(), endOfStream: true);
          buffer.clear();

          for (final line in scan.lines) {
            if (line.isEmpty) {
              flushDataBlock();
            } else {
              appendDataLine(line);
            }
          }

          // Any unconsumed suffix is a final partial line with no
          // terminator. The pre-CRLF-fix code only handled `data:`-prefixed
          // partials here; `appendDataLine` preserves that behavior because
          // it ignores non-`data:` lines per spec.
          if (scan.unconsumed.isNotEmpty) {
            appendDataLine(scan.unconsumed);
          }

          // Final flush — emits any leftover data block accumulated from
          // either the deferred-line scan or the partial-line append above.
          flushDataBlock();
          controller.close();
        },
        cancelOnError: false,
      );
    };
    controller.onCancel = () async {
      await subscription?.cancel();
      subscription = null;
    };
    controller.onPause = () => subscription?.pause();
    controller.onResume = () => subscription?.resume();

    return controller.stream;
  }

  /// Scans [input] for complete lines, returning the complete lines and
  /// the unconsumed suffix. Per the WHATWG SSE spec, line terminators
  /// can be `\r\n`, lone `\n`, or lone `\r`.
  ///
  /// When [endOfStream] is `false`, a trailing `\r` at the end of the
  /// buffer is left in the unconsumed suffix to disambiguate a
  /// chunk-spanning `\r\n` (the next chunk could start with `\n`).
  /// EXCEPTION: when the immediately preceding terminator in this scan
  /// was also a lone `\r`, the producer is committed to lone-CR style and
  /// the trailing `\r` is consumed immediately — without this exception
  /// a single-chunk `data: foo\r\r` would defer the event-boundary `\r`
  /// and stall steady-state lone-CR streams. CRLF producers cannot
  /// trigger this exception because every `\r` is paired with `\n`
  /// (so `lastWasLoneCr` never becomes `true` in the same scan).
  ///
  /// When [endOfStream] is `true`, the deferral is disabled entirely —
  /// any trailing `\r` is consumed as a lone-CR terminator since no
  /// further chunks are coming.
  static ({List<String> lines, String unconsumed, bool lastWasLoneCr}) _scanLines(
    String input, {
    required bool endOfStream,
    bool lastWasLoneCrAtStart = false,
  }) {
    final lines = <String>[];
    var s = input;
    var lastWasLoneCr = lastWasLoneCrAtStart;
    while (true) {
      final lf = s.indexOf('\n');
      final cr = s.indexOf('\r');
      int breakIndex;
      if (lf == -1 && cr == -1) break;
      if (lf == -1) {
        breakIndex = cr;
      } else if (cr == -1) {
        breakIndex = lf;
      } else {
        breakIndex = lf < cr ? lf : cr;
      }

      // Defer a trailing `\r` so a chunk-spanning `\r\n` doesn't appear
      // as two terminators (lone `\r` then lone `\n`). Skip the deferral
      // when the previous terminator was lone-CR — the producer is
      // clearly using lone-CR style, so the trailing `\r` IS its own
      // terminator. See class-level scan rationale above.
      if (!endOfStream &&
          !lastWasLoneCr &&
          s.codeUnitAt(breakIndex) == 0x0D /* \r */ &&
          breakIndex == s.length - 1) {
        break;
      }

      final isCrLf = s.codeUnitAt(breakIndex) == 0x0D &&
          breakIndex + 1 < s.length &&
          s.codeUnitAt(breakIndex + 1) == 0x0A /* \n */;
      lastWasLoneCr =
          s.codeUnitAt(breakIndex) == 0x0D /* \r */ && !isCrLf;
      final line = s.substring(0, breakIndex);
      lines.add(line);
      s = s.substring(breakIndex + (isCrLf ? 2 : 1));
    }
    return (lines: lines, unconsumed: s, lastWasLoneCr: lastWasLoneCr);
  }

  /// Filters a stream of events to only include specific event types.
  static Stream<T> filterByType<T extends BaseEvent>(
    Stream<BaseEvent> eventStream,
  ) {
    return eventStream.where((event) => event is T).cast<T>();
  }

  /// Groups related events together.
  ///
  /// For example, groups TEXT_MESSAGE_START, TEXT_MESSAGE_CONTENT,
  /// and TEXT_MESSAGE_END events for the same messageId.
  ///
  /// **Unbounded-state warning.** Open groups (where `*Start` was
  /// received but `*End` has not yet arrived) are held in memory until
  /// the matching `*End` event arrives or the upstream stream
  /// completes. A producer that opens IDs without closing them — for
  /// instance, an interrupted upstream connection or a buggy server —
  /// will grow the internal map indefinitely. For long-lived streams
  /// from untrusted producers, sanitize upstream or wrap with a
  /// timeout. The same caveat applies to [accumulateTextMessages].
  static Stream<List<BaseEvent>> groupRelatedEvents(
    Stream<BaseEvent> eventStream,
  ) {
    final controller = StreamController<List<BaseEvent>>(sync: true);
    final Map<String, List<BaseEvent>> activeGroups = {};
    StreamSubscription<BaseEvent>? subscription;

    // Defer subscription to `onListen` so that:
    //   • A caller that stores the stream but never subscribes does not
    //     leak the upstream listener.
    //   • Backpressure (pause/resume/cancel) propagates correctly to
    //     the upstream, matching the pattern used by `fromRawSseStream`.
    controller.onListen = () {
      subscription = eventStream.listen(
        (event) {
          switch (event) {
            case TextMessageStartEvent(:final messageId):
              activeGroups[messageId] = [event];
            case TextMessageContentEvent(:final messageId):
              activeGroups[messageId]?.add(event);
            case TextMessageEndEvent(:final messageId):
              final group = activeGroups.remove(messageId);
              if (group != null) {
                group.add(event);
                controller.add(group);
              }
            case ToolCallStartEvent(:final toolCallId):
              activeGroups[toolCallId] = [event];
            case ToolCallArgsEvent(:final toolCallId):
              activeGroups[toolCallId]?.add(event);
            case ToolCallEndEvent(:final toolCallId):
              final group = activeGroups.remove(toolCallId);
              if (group != null) {
                group.add(event);
                controller.add(group);
              }
            default:
              // Single events not part of a group
              controller.add([event]);
          }
        },
        onError: controller.addError,
        onDone: () {
          // Emit any incomplete groups
          for (final group in activeGroups.values) {
            if (group.isNotEmpty) {
              controller.add(group);
            }
          }
          controller.close();
        },
        cancelOnError: false,
      );
    };
    controller.onCancel = () async {
      await subscription?.cancel();
      subscription = null;
    };
    controller.onPause = () => subscription?.pause();
    controller.onResume = () => subscription?.resume();

    return controller.stream;
  }

  /// Accumulates text message content into complete messages.
  static Stream<String> accumulateTextMessages(
    Stream<BaseEvent> eventStream,
  ) {
    final controller = StreamController<String>(sync: true);
    final Map<String, StringBuffer> activeMessages = {};
    StreamSubscription<BaseEvent>? subscription;

    // Defer subscription to `onListen` — mirrors `groupRelatedEvents`
    // and `fromRawSseStream` so upstream leaks and backpressure issues
    // are avoided. Uses `sync: true` to match the synchronous-emit
    // contract of the other stream helpers in this class.
    controller.onListen = () {
      subscription = eventStream.listen(
        (event) {
          switch (event) {
            case TextMessageStartEvent(:final messageId):
              activeMessages[messageId] = StringBuffer();
            case TextMessageContentEvent(:final messageId, :final delta):
              activeMessages[messageId]?.write(delta);
            case TextMessageEndEvent(:final messageId):
              final buffer = activeMessages.remove(messageId);
              if (buffer != null) {
                controller.add(buffer.toString());
              }
            case TextMessageChunkEvent(:final messageId, :final delta):
              // Handle chunk events (single event with complete content)
              if (messageId != null && delta != null) {
                controller.add(delta);
              }
            default:
              // Ignore other event types
              break;
          }
        },
        onError: controller.addError,
        onDone: controller.close,
        cancelOnError: false,
      );
    };
    controller.onCancel = () async {
      await subscription?.cancel();
      subscription = null;
    };
    controller.onPause = () => subscription?.pause();
    controller.onResume = () => subscription?.resume();

    return controller.stream;
  }
}