/// Stream adapter for converting SSE messages to typed AG-UI events.
library;

import 'dart:async';

import '../client/errors.dart';
import '../events/events.dart';
import '../sse/sse_message.dart';
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
              // Skip keep-alive messages
              if (data.trim() == ':') {
                return;
              }
              
              // `decode` already runs `validate` via `decodeJson`; no
              // second pass needed here.
              sink.add(_decoder.decode(data));
            }
            // Ignore non-data messages (id, event, retry, comments)
          } catch (e, stack) {
            final error = e is AgUiError ? e : DecodingError(
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
  /// See [fromSseStream] for the [skipInvalidEvents] / [onError]
  /// semantics, including the silent-drop note for
  /// `REASONING_ENCRYPTED_VALUE` events with unknown subtypes.
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

    void processChunk(String chunk) {
      // Add chunk to buffer to handle partial lines
      buffer.write(chunk);

      // Process complete lines only
      String bufferStr = buffer.toString();
      final lines = <String>[];

      // Extract complete lines (those ending with \n)
      while (bufferStr.contains('\n')) {
        final lineEnd = bufferStr.indexOf('\n');
        final line = bufferStr.substring(0, lineEnd);
        lines.add(line);
        bufferStr = bufferStr.substring(lineEnd + 1);
      }

      // Keep any incomplete line in the buffer
      buffer.clear();
      buffer.write(bufferStr);

      // Process each complete line
      for (final line in lines) {
        if (line.isEmpty) {
          // Empty line signals end of SSE message
          if (inDataBlock) {
            final data = dataBuffer.toString();
            dataBuffer.clear();
            inDataBlock = false;

            if (data.isNotEmpty && data.trim() != ':') {
              try {
                // `decode` already runs `validate` via `decodeJson`; no
                // second pass needed here.
                controller.add(_decoder.decode(data));
              } catch (e, stack) {
                final error = e is AgUiError
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
          }
        } else if (line.startsWith('data: ')) {
          // Extract data value (after "data: ")
          final value = line.substring(6);
          if (inDataBlock) {
            // Multi-line data: add newline between lines
            dataBuffer.write('\n');
            dataBuffer.write(value);
          } else {
            // Start new data block
            dataBuffer.clear();
            dataBuffer.write(value);
            inDataBlock = true;
          }
        } else if (line.startsWith('data:')) {
          // Handle no space after colon
          final value = line.substring(5);
          if (inDataBlock) {
            dataBuffer.write('\n');
            dataBuffer.write(value);
          } else {
            dataBuffer.clear();
            dataBuffer.write(value);
            inDataBlock = true;
          }
        }
        // Ignore other lines (comments, event:, id:, retry:, etc.)
      }
    }

    rawStream.listen(
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
        // Process any remaining incomplete line in buffer
        final remaining = buffer.toString();
        if (remaining.isNotEmpty) {
          // Treat remaining content as a complete line
          if (remaining.startsWith('data: ')) {
            final value = remaining.substring(6);
            if (inDataBlock) {
              dataBuffer.write('\n');
              dataBuffer.write(value);
            } else {
              dataBuffer.clear();
              dataBuffer.write(value);
              inDataBlock = true;
            }
          } else if (remaining.startsWith('data:')) {
            final value = remaining.substring(5);
            if (inDataBlock) {
              dataBuffer.write('\n');
              dataBuffer.write(value);
            } else {
              dataBuffer.clear();
              dataBuffer.write(value);
              inDataBlock = true;
            }
          }
        }

        // Process any accumulated data
        if (inDataBlock && dataBuffer.isNotEmpty) {
          final data = dataBuffer.toString();
          try {
            final event = _decoder.decode(data);
            controller.add(event);
          } catch (e, stack) {
            if (!skipInvalidEvents) {
              controller.addError(e, stack);
            } else {
              onError?.call(e, stack);
            }
          }
        }
        controller.close();
      },
      cancelOnError: false,
    );

    return controller.stream;
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
  static Stream<List<BaseEvent>> groupRelatedEvents(
    Stream<BaseEvent> eventStream,
  ) {
    final controller = StreamController<List<BaseEvent>>(sync: true);
    final Map<String, List<BaseEvent>> activeGroups = {};
    
    eventStream.listen(
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
    
    return controller.stream;
  }

  /// Accumulates text message content into complete messages.
  static Stream<String> accumulateTextMessages(
    Stream<BaseEvent> eventStream,
  ) {
    final controller = StreamController<String>();
    final Map<String, StringBuffer> activeMessages = {};
    
    eventStream.listen(
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
    
    return controller.stream;
  }
}