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
              // Keep-alive sentinels (data field whose trimmed value is `:`).
              // Silently discard regardless of `skipInvalidEvents` — a
              // keep-alive is not a protocol error; routing it through
              // `onError` would cause consumers that log on `onError` to
              // receive spurious noise on every server keep-alive ping.
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
  /// **Semantic divergence from [EventDecoder.decodeSSE]:**
  /// - `decodeSSE` receives a complete SSE message string and throws a
  ///   structured [DecodingError] for keep-alive frames (comment-only or
  ///   `data: :` payloads) and for frames with no `data:` lines.
  /// - `fromRawSseStream` receives raw streaming chunks; keep-alives
  ///   (`data.trim() == ':'`) are silently discarded in [flushDataBlock]
  ///   and partial frames accumulate across chunks. The two methods share
  ///   the same final `decode` call but differ on keep-alive routing and
  ///   partial-frame handling.
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
    // `sync: true` means `controller.add(...)` calls downstream listeners
    // synchronously on the same call stack. Re-entrancy contract:
    // consumers MUST NOT call `subscription.cancel()` synchronously from
    // inside a `listen` data handler — doing so cancels the underlying
    // subscription while it is still being iterated and can cause a
    // `ConcurrentModificationError` or double-close. If you need to
    // cancel on a received event, schedule it via `Future.microtask`.
    //
    // Re-entrancy guard: if synchronous re-entry through controller.add
    // is detected (e.g. a downstream data handler cancels the subscription
    // during dispatch), flushDataBlock throws StateError before state is
    // corrupted. Note this guard only covers the dispatch site inside
    // flushDataBlock, not the buffer-mutation path.
    final controller = StreamController<BaseEvent>(sync: true);
    var inDispatch = false;

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
    // Returns `true` if an error was routed to the controller so callers
    // can suppress a redundant second `addError` from their own catch.
    bool flushDataBlock() {
      if (!inDataBlock) return false;
      final data = dataBuffer.toString();
      dataBuffer.clear();
      inDataBlock = false;

      if (data.isEmpty || data.trim() == ':') return false;

      // Programmer-error guard sits outside the wire-error catch so a
      // re-entrancy bug doesn't masquerade as a decoding failure.
      if (inDispatch) {
        throw StateError(
          'sync re-entrancy: cancel() must not be called synchronously '
          'from inside a data handler; use Future.microtask. See '
          'fromRawSseStream dartdoc for details.',
        );
      }

      try {
        // `decode` already runs `validate` via `decodeJson`; no
        // second pass needed here.
        inDispatch = true;
        try {
          controller.add(_decoder.decode(data));
        } finally {
          inDispatch = false;
        }
        return false;
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
        return true; // error was already routed
      }
    }

    // Whether the current chunk's `flushDataBlock` call already routed an
    // error so the outer `onListen` catch can skip a second `addError`.
    var errorRoutedInChunk = false;

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
          if (flushDataBlock()) errorRoutedInChunk = true;
        } else {
          appendDataLine(line);
        }
      }
      // Do NOT reset errorRoutedInChunk here. The flag is reset per-chunk
      // at the start of the listen handler (line above processChunk call).
      // Resetting here would nullify the deduplication invariant before the
      // outer catch can read it, allowing double-addError on the same event.
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
          errorRoutedInChunk = false;
          try {
            processChunk(chunk);
          } catch (e, stack) {
            // If `flushDataBlock` already routed an error to the controller
            // (via `controller.addError`), skip a second `addError` here to
            // avoid double-firing the same error at the stream consumer.
            if (errorRoutedInChunk) return;
            final error = e is AGUIError
                ? e
                : DecodingError(
                    'Internal error processing SSE chunk',
                    field: 'chunk',
                    expectedType: 'String',
                    actualValue: chunk,
                    cause: e,
                  );
            if (!skipInvalidEvents) {
              controller.addError(error, stack);
            } else {
              onError?.call(error, stack);
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
          errorRoutedInChunk = false; // defensive reset; flag lifecycle ends at chunk handler
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

    // Edge case: when `lastWasLoneCrAtStart` is true, the previous scan
    // consumed a lone-CR at its boundary immediately (because the exception
    // that skips deferral for known-lone-CR producers applied). If the new
    // chunk starts with `\n`, that `\n` is the second half of a
    // chunk-spanning CRLF pair — skip it so the pair does not dispatch an
    // extra empty-line boundary.
    String s;
    bool lastWasLoneCr;
    if (lastWasLoneCrAtStart &&
        input.isNotEmpty &&
        input.codeUnitAt(0) == 0x0A /* \n */) {
      s = input.substring(1);
      lastWasLoneCr = false; // was actually CRLF, not lone-CR
    } else {
      s = input;
      lastWasLoneCr = lastWasLoneCrAtStart;
    }
    // Single-pass O(n) scan: advance index `i` forward rather than
    // repeatedly calling indexOf + substring (which was O(n²) on inputs
    // with many lines, since each iteration re-scanned the remaining string).
    var i = 0;
    while (i < s.length) {
      // Scan forward for the next \r or \n terminator.
      int brk = -1;
      for (var j = i; j < s.length; j++) {
        final c = s.codeUnitAt(j);
        if (c == 0x0A /* \n */ || c == 0x0D /* \r */) {
          brk = j;
          break;
        }
      }
      if (brk == -1) break; // no more terminators in remaining input

      // Defer a trailing `\r` so a chunk-spanning `\r\n` doesn't appear
      // as two terminators (lone `\r` then lone `\n`). Skip the deferral
      // when the previous terminator was lone-CR — the producer is
      // clearly using lone-CR style, so the trailing `\r` IS its own
      // terminator. See class-level scan rationale above.
      //
      // NOTE on the "chunk ends exactly at \r" case (e.g. chunk = "foo\r"):
      // This deferral fires and leaves `\r` in the unconsumed suffix.
      // `lastWasLoneCrAtStart` is NOT involved here — that flag is only set
      // when a PREVIOUS scan already consumed a lone-CR at its boundary
      // (the producer was confirmed lone-CR style). In this path the `\r`
      // is tentative: the next chunk may start with `\n` (making it CRLF)
      // or not (making it lone-CR). The next scan will resolve it via the
      // `lastWasLoneCrAtStart` edge-case check at the top of `_scanLines`.
      if (!endOfStream &&
          !lastWasLoneCr &&
          s.codeUnitAt(brk) == 0x0D /* \r */ &&
          brk == s.length - 1) {
        break;
      }

      final isCrLf = s.codeUnitAt(brk) == 0x0D &&
          brk + 1 < s.length &&
          s.codeUnitAt(brk + 1) == 0x0A /* \n */;
      lastWasLoneCr =
          s.codeUnitAt(brk) == 0x0D /* \r */ && !isCrLf;
      lines.add(s.substring(i, brk));
      i = brk + (isCrLf ? 2 : 1);
    }
    return (lines: lines, unconsumed: s.substring(i), lastWasLoneCr: lastWasLoneCr);
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
  ///
  /// **Duplicate-start policy.** If a second `*Start` event arrives with
  /// the same id while the prior group is still open, the prior group's
  /// accumulated events are discarded silently and a new group begins
  /// ("last-Start-wins"). This matches the behavior of the TS/Python
  /// reference SDKs. Consumers that need strict sequencing should validate
  /// the upstream event stream before passing it here.
  ///
  /// **On stream close:** any open groups (where a `*Start` was received
  /// but `*End` has not yet arrived) are emitted in `*Start` arrival order.
  /// Consumers should treat such groups as potentially incomplete — they
  /// will be missing the terminal `*End` event and any final content that
  /// never arrived.
  ///
  /// **Reasoning event asymmetry.** Only message-level
  /// `REASONING_MESSAGE_START` / `REASONING_MESSAGE_CONTENT` /
  /// `REASONING_MESSAGE_END` events are grouped (under the key
  /// `reasoning:<messageId>`). The phase-level `REASONING_START` /
  /// `REASONING_END` events are emitted as standalone singletons — they
  /// fall through to the `default` case. Consumers that need to associate
  /// phase-level markers with the messages they wrap should track the phase
  /// boundary in their own state, or subscribe to the typed event stream
  /// directly.
  static Stream<List<BaseEvent>> groupRelatedEvents(
    Stream<BaseEvent> eventStream,
  ) {
    // `sync: true` — see re-entrancy note on [fromRawSseStream].
    final controller = StreamController<List<BaseEvent>>(sync: true);
    // LinkedHashMap insertion order is relied upon by the onDone flush —
    // incomplete groups are emitted in *Start arrival order. Do NOT replace
    // with HashMap (unordered) or SplayTreeMap (sorted).
    final Map<String, List<BaseEvent>> activeGroups = {};
    StreamSubscription<BaseEvent>? subscription;
    var inDispatch = false;

    // Defer subscription to `onListen` so that:
    //   • A caller that stores the stream but never subscribes does not
    //     leak the upstream listener.
    //   • Backpressure (pause/resume/cancel) propagates correctly to
    //     the upstream, matching the pattern used by `fromRawSseStream`.
    controller.onListen = () {
      subscription = eventStream.listen(
        (event) {
          if (inDispatch) {
            throw StateError(
              'sync re-entrancy: cancel() must not be called synchronously '
              'from inside a groupRelatedEvents data handler; use '
              'Future.microtask.',
            );
          }
          inDispatch = true;
          try {
          switch (event) {
            // Keys are namespaced by event family ('text:', 'reasoning:',
            // 'tool:') so that a producer reusing the same id across families
            // (e.g. a text message and a reasoning step sharing a messageId)
            // does not overwrite one group with another.
            case TextMessageStartEvent(:final messageId):
              activeGroups['text:$messageId'] = [event];
            case TextMessageContentEvent(:final messageId):
              activeGroups['text:$messageId']?.add(event);
            case TextMessageEndEvent(:final messageId):
              final group = activeGroups.remove('text:$messageId');
              if (group != null) {
                group.add(event);
                controller.add(group);
              }
            case ToolCallStartEvent(:final toolCallId):
              activeGroups['tool:$toolCallId'] = [event];
            case ToolCallArgsEvent(:final toolCallId):
              activeGroups['tool:$toolCallId']?.add(event);
            case ToolCallEndEvent(:final toolCallId):
              final group = activeGroups.remove('tool:$toolCallId');
              if (group != null) {
                group.add(event);
                controller.add(group);
              }
            case ReasoningMessageStartEvent(:final messageId):
              activeGroups['reasoning:$messageId'] = [event];
            case ReasoningMessageContentEvent(:final messageId):
              activeGroups['reasoning:$messageId']?.add(event);
            case ReasoningMessageEndEvent(:final messageId):
              final group = activeGroups.remove('reasoning:$messageId');
              if (group != null) {
                group.add(event);
                controller.add(group);
              }
            case TextMessageChunkEvent(:final messageId):
              // Fold into the open text group when one exists; otherwise emit
              // standalone — chunks may arrive without a preceding *Start.
              if (messageId != null &&
                  activeGroups.containsKey('text:$messageId')) {
                activeGroups['text:$messageId']!.add(event);
              } else {
                controller.add([event]);
              }
            case ToolCallChunkEvent(:final toolCallId):
              // Fold into the open tool group when one exists; otherwise emit
              // standalone — chunks may arrive without a preceding *Start.
              if (toolCallId != null &&
                  activeGroups.containsKey('tool:$toolCallId')) {
                activeGroups['tool:$toolCallId']!.add(event);
              } else {
                controller.add([event]);
              }
            case ReasoningMessageChunkEvent(:final messageId):
              // Fold into the open reasoning group when one exists; otherwise
              // emit standalone — chunks may arrive without a preceding *Start.
              if (messageId != null &&
                  activeGroups.containsKey('reasoning:$messageId')) {
                activeGroups['reasoning:$messageId']!.add(event);
              } else {
                controller.add([event]);
              }
            default:
              // Single events not part of a group
              controller.add([event]);
          }
          } finally {
            inDispatch = false;
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

  /// Accumulates user-visible text message content into complete messages.
  ///
  /// **Scope: user-visible text only.** Only `TEXT_MESSAGE_*` and
  /// `TEXT_MESSAGE_CHUNK` events are handled. `REASONING_MESSAGE_*` events
  /// (model-internal reasoning chains, not shown to the end user) are
  /// intentionally excluded — consumers that need to accumulate reasoning
  /// content should use [groupRelatedEvents] and filter by type, or write
  /// a dedicated sibling accumulator.
  ///
  /// Emits one [String] per logical message when its `TextMessageEnd` event
  /// arrives. **On stream close:** any accumulated-but-not-ended message
  /// buffers are flushed in `*Start` arrival order as a final [String],
  /// matching [groupRelatedEvents]' "emit incomplete groups on close"
  /// behavior. Empty buffers are not emitted. Consumers cannot distinguish
  /// between a normally-completed message and a flushed-on-close partial
  /// without observing the absence of `TextMessageEnd` upstream.
  static Stream<String> accumulateTextMessages(
    Stream<BaseEvent> eventStream,
  ) {
    // `sync: true` — see re-entrancy note on [fromRawSseStream].
    final controller = StreamController<String>(sync: true);
    // LinkedHashMap insertion order is relied upon by the onDone flush —
    // incomplete messages are emitted in *Start arrival order. Do NOT replace
    // with HashMap (unordered) or SplayTreeMap (sorted).
    final Map<String, StringBuffer> activeMessages = {};
    StreamSubscription<BaseEvent>? subscription;
    var inDispatch = false;

    // Defer subscription to `onListen` — mirrors `groupRelatedEvents`
    // and `fromRawSseStream` so upstream leaks and backpressure issues
    // are avoided. Uses `sync: true` to match the synchronous-emit
    // contract of the other stream helpers in this class.
    controller.onListen = () {
      subscription = eventStream.listen(
        (event) {
          if (inDispatch) {
            throw StateError(
              'sync re-entrancy: cancel() must not be called synchronously '
              'from inside an accumulateTextMessages data handler; use '
              'Future.microtask.',
            );
          }
          inDispatch = true;
          try {
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
              // A chunk is a standalone text fragment. If a Start/End cycle is
              // open for the same messageId, route it into the active buffer —
              // otherwise a standalone chunk would appear before the eventual
              // End-triggered buffer flush (Start/Content events have not been
              // emitted yet at that point). When messageId is null or no open
              // buffer exists, emit the delta immediately.
              if (delta == null) break; // genuinely nothing to emit
              if (messageId != null) {
                final activeBuffer = activeMessages[messageId];
                if (activeBuffer != null) {
                  activeBuffer.write(delta);
                  break;
                }
              }
              controller.add(delta); // standalone fragment — emit even when messageId is null
            default:
              // Ignore other event types
              break;
          }
          } finally {
            inDispatch = false;
          }
        },
        onError: controller.addError,
        onDone: () {
          // Emit accumulated content for messages that never received
          // TextMessageEnd (e.g. abnormal stream close). Mirrors
          // groupRelatedEvents which emits incomplete groups on close.
          for (final entry in activeMessages.entries) {
            final content = entry.value.toString();
            if (content.isNotEmpty) controller.add(content);
          }
          activeMessages.clear();
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
}