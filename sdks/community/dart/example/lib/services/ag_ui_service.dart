import 'dart:async';
import 'dart:convert';

import 'package:ag_ui/ag_ui.dart';
import 'package:flutter/foundation.dart';
import 'package:http/http.dart' as http;

import 'ids.dart';

class AgUiService {
  static const _defaultBaseUrl = 'http://127.0.0.1:8080';
  final String baseUrl;
  late final AgUiClient _client;
  final StreamController<ConnectionStatus> _connectionController =
      StreamController<ConnectionStatus>.broadcast();
  CancelToken? _activeToken;
  bool _busy = false;
  bool _disposed = false;
  Future<void>? _closeFuture;

  AgUiService({String? baseUrl, http.Client? httpClient})
    : baseUrl = resolveBaseUrl(baseUrl) {
    _client = AgUiClient(
      config: AgUiClientConfig(baseUrl: this.baseUrl),
      httpClient: httpClient,
    );
    _emitStatus(ConnectionStatus.disconnected);
  }

  Stream<ConnectionStatus> get connectionStatus => _connectionController.stream;
  bool get isBusy => _busy;

  Stream<BaseEvent> run(
    String endpoint, {
    required String threadId,
    required List<Message> messages,
    List<Tool> tools = const [],
    dynamic state,
    Map<String, String> extraQuery = const {},
  }) {
    if (_disposed) throw StateError('AgUiService is closed');
    if (_busy) throw StateError('An AG-UI exchange is already active');
    final messageSnapshot = List<Message>.unmodifiable(
      messages.map(
        (message) => Message.fromJson(
          jsonDecode(jsonEncode(message.toJson())) as Map<String, dynamic>,
        ),
      ),
    );
    final toolSnapshot = List<Tool>.unmodifiable(
      tools.map(
        (tool) => Tool.fromJson(
          jsonDecode(jsonEncode(tool.toJson())) as Map<String, dynamic>,
        ),
      ),
    );
    final stateSnapshot = _deepSnapshot(state ?? <String, dynamic>{});
    final path = _endpointWithQuery(endpoint, extraQuery);

    _busy = true;
    final token = CancelToken();
    _activeToken = token;
    return _executeRun(
      path,
      threadId: threadId,
      messages: messageSnapshot,
      tools: toolSnapshot,
      state: stateSnapshot,
      token: token,
    );
  }

  Stream<BaseEvent> _executeRun(
    String path, {
    required String threadId,
    required List<Message> messages,
    required List<Tool> tools,
    required dynamic state,
    required CancelToken token,
  }) async* {
    try {
      _emitStatus(ConnectionStatus.connecting);
      final input = SimpleRunAgentInput(
        threadId: threadId,
        runId: uid('run'),
        messages: messages,
        tools: tools,
        context: const [],
        state: state,
        forwardedProps: <String, dynamic>{},
      );
      _emitStatus(ConnectionStatus.connected);
      await for (final event in _client.runAgent(
        path,
        input,
        cancelToken: token,
      )) {
        if (_disposed || token.isCancelled) break;
        yield event;
      }
      _emitStatus(ConnectionStatus.disconnected);
    } catch (error) {
      if (!_disposed) {
        _emitStatus(ConnectionStatus.error);
        debugPrint('Error in AG-UI run: $error');
      }
      rethrow;
    } finally {
      if (identical(_activeToken, token)) _activeToken = null;
      _busy = false;
    }
  }

  Stream<BaseEvent> sendMessage(String endpoint, String message) => run(
    endpoint,
    threadId: uid('thread'),
    messages: [UserMessage(id: uid('user'), content: message)],
  );

  Stream<BaseEvent> sendMultimodalMessage(
    String endpoint,
    List<InputContent> parts,
  ) => run(
    endpoint,
    threadId: uid('thread'),
    messages: [UserMessage.multimodal(id: uid('user'), parts: parts)],
  );

  Future<void> close() => _closeFuture ??= _close();

  Future<void> _close() async {
    if (_disposed) return;
    _disposed = true;
    _activeToken?.cancel();
    _activeToken = null;
    await _client.close();
    if (!_connectionController.isClosed) await _connectionController.close();
  }

  void dispose() {
    unawaited(
      close().catchError((Object error, StackTrace stackTrace) {
        debugPrint('Error closing AG-UI service: $error');
      }),
    );
  }

  void _emitStatus(ConnectionStatus status) {
    if (!_disposed && !_connectionController.isClosed) {
      _connectionController.add(status);
    }
  }

  static String _normalizeBaseUrl(String value) {
    final trimmed = value.trim();
    final uri = Uri.tryParse(trimmed);
    if (trimmed.isEmpty ||
        uri == null ||
        !uri.hasAuthority ||
        uri.host.isEmpty ||
        (uri.scheme != 'http' && uri.scheme != 'https') ||
        uri.query.isNotEmpty ||
        uri.fragment.isNotEmpty) {
      throw ArgumentError.value(value, 'baseUrl', 'must be an HTTP(S) URL');
    }
    return uri
        .replace(path: uri.path.replaceFirst(RegExp(r'/+$'), ''))
        .toString()
        .replaceFirst(RegExp(r'/+$'), '');
  }

  /// Resolves and validates the explicit override or compile-time app URL
  /// without allocating an HTTP client. The app shell can use this to render a
  /// configuration error instead of failing during widget construction.
  static String resolveBaseUrl([String? override]) => _normalizeBaseUrl(
    override ??
        const String.fromEnvironment(
          'AG_UI_BASE_URL',
          defaultValue: _defaultBaseUrl,
        ),
  );

  static String _endpointWithQuery(String endpoint, Map<String, String> query) {
    final clean = endpoint.replaceFirst(RegExp(r'^/+'), '');
    final uri = Uri.tryParse(clean);
    if (clean.isEmpty || uri == null || uri.isAbsolute || uri.path.isEmpty) {
      throw ArgumentError.value(
        endpoint,
        'endpoint',
        'must be a relative endpoint path',
      );
    }
    if (query.isEmpty) return uri.toString();
    return uri
        .replace(queryParameters: {...uri.queryParameters, ...query})
        .toString();
  }

  static dynamic _deepSnapshot(dynamic value) => jsonDecode(jsonEncode(value));
}

enum ConnectionStatus { connected, connecting, disconnected, error }
