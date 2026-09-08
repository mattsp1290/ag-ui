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
  late final _OwnedHttpClient _httpClient;
  final StreamController<ConnectionStatus> _connectionController =
      StreamController<ConnectionStatus>.broadcast();
  CancelToken? _activeToken;
  bool _busy = false;
  bool _disposed = false;
  Future<void>? _closeFuture;

  AgUiService({String? baseUrl, http.Client? httpClient})
    : baseUrl = resolveBaseUrl(baseUrl) {
    _httpClient = _OwnedHttpClient(httpClient ?? http.Client());
    _client = AgUiClient(
      config: AgUiClientConfig(baseUrl: this.baseUrl),
      httpClient: _httpClient,
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

    return _ownedRunStream(
      path,
      threadId: threadId,
      messages: messageSnapshot,
      tools: toolSnapshot,
      state: stateSnapshot,
    );
  }

  Stream<BaseEvent> _ownedRunStream(
    String path, {
    required String threadId,
    required List<Message> messages,
    required List<Tool> tools,
    required dynamic state,
  }) {
    StreamSubscription<BaseEvent>? upstream;
    CancelToken? token;
    String? runId;
    var completed = false;
    var cancelRequested = false;
    late final StreamController<BaseEvent> controller;

    void release() {
      final owned = token;
      if (owned != null && identical(_activeToken, owned)) {
        _activeToken = null;
        _busy = false;
      }
    }

    controller = StreamController<BaseEvent>(
      onListen: () {
        if (_disposed) {
          controller.addError(StateError('AgUiService is closed'));
          unawaited(controller.close());
          return;
        }
        if (_busy) {
          controller.addError(
            StateError('An AG-UI exchange is already active'),
          );
          unawaited(controller.close());
          return;
        }
        _busy = true;
        token = CancelToken();
        runId = uid('run');
        _activeToken = token;
        _emitStatus(ConnectionStatus.connecting);
        final input = SimpleRunAgentInput(
          threadId: threadId,
          runId: runId,
          messages: messages,
          tools: tools,
          context: const [],
          state: state,
          forwardedProps: <String, dynamic>{},
        );
        _emitStatus(ConnectionStatus.connected);
        try {
          upstream = _client
              .runAgent(path, input, cancelToken: token)
              .listen(
                (BaseEvent event) {
                  if (!_disposed && !cancelRequested && !token!.isCancelled) {
                    controller.add(event);
                  }
                },
                onError: (Object error, StackTrace stackTrace) {
                  if (!cancelRequested && !token!.isCancelled) {
                    _emitStatus(ConnectionStatus.error);
                    debugPrint('Error in AG-UI run: $error');
                    controller.addError(error, stackTrace);
                  }
                },
                onDone: () {
                  completed = true;
                  _emitStatus(ConnectionStatus.disconnected);
                  release();
                  if (!cancelRequested) unawaited(controller.close());
                },
              );
        } catch (error, stackTrace) {
          _emitStatus(ConnectionStatus.error);
          release();
          controller.addError(error, stackTrace);
          unawaited(controller.close());
        }
      },
      onCancel: () async {
        if (completed) return;
        cancelRequested = true;
        token?.cancel();
        try {
          await _httpClient.cancelActiveResponse();
          try {
            await upstream?.cancel();
          } catch (_) {
            if (!(token?.isCancelled ?? false)) rethrow;
          }
          final ownedRunId = runId;
          if (ownedRunId != null) {
            try {
              await _client.cancelRun(ownedRunId);
            } catch (_) {
              if (!(token?.isCancelled ?? false)) rethrow;
            }
          }
        } finally {
          release();
        }
      },
      onPause: () => upstream?.pause(),
      onResume: () => upstream?.resume(),
    );
    return controller.stream;
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
    _busy = false;
    try {
      await _httpClient.cancelActiveResponse();
      await _client.close();
      await _httpClient.closeCleanup;
    } finally {
      if (!_connectionController.isClosed) {
        await _connectionController.close();
      }
    }
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

/// Gives the application explicit ownership of the currently streaming HTTP
/// response. The SDK's stateless SSE parser does not retain that subscription,
/// so cancelling its event stream alone cannot abort a response whose server
/// remains open.
class _OwnedHttpClient extends http.BaseClient {
  final http.Client _delegate;
  _OwnedResponseBody? _activeBody;
  int _generation = 0;
  bool _closed = false;
  Future<void> _closeCleanup = Future<void>.value();

  _OwnedHttpClient(this._delegate);

  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) async {
    final requestGeneration = ++_generation;
    final response = await _delegate.send(request);
    late final _OwnedResponseBody body;
    body = _OwnedResponseBody(
      response.stream,
      mayActivate: () => !_closed && requestGeneration == _generation,
      onActivate: () => _activeBody = body,
      onRelease: () {
        if (identical(_activeBody, body)) _activeBody = null;
      },
    );

    return http.StreamedResponse(
      body.stream,
      response.statusCode,
      contentLength: response.contentLength,
      request: response.request,
      headers: response.headers,
      isRedirect: response.isRedirect,
      persistentConnection: response.persistentConnection,
      reasonPhrase: response.reasonPhrase,
    );
  }

  Future<void> cancelActiveResponse() async {
    _generation++;
    final body = _activeBody;
    _activeBody = null;
    await body?.cancel();
  }

  Future<void> get closeCleanup => _closeCleanup;

  @override
  void close() {
    if (_closed) return;
    _closed = true;
    _closeCleanup = cancelActiveResponse();
    _delegate.close();
  }
}

class _OwnedResponseBody {
  final Stream<List<int>> _source;
  final bool Function() _mayActivate;
  final void Function() _onActivate;
  final void Function() _onRelease;
  late final StreamController<List<int>> _controller;
  StreamSubscription<List<int>>? _subscription;

  _OwnedResponseBody(
    this._source, {
    required bool Function() mayActivate,
    required void Function() onActivate,
    required void Function() onRelease,
  }) : _mayActivate = mayActivate,
       _onActivate = onActivate,
       _onRelease = onRelease {
    _controller = StreamController<List<int>>(
      onListen: _listen,
      onCancel: cancel,
      onPause: () => _subscription?.pause(),
      onResume: () => _subscription?.resume(),
    );
  }

  Stream<List<int>> get stream => _controller.stream;

  void _listen() {
    _subscription = _source.listen(
      _controller.add,
      onError: _controller.addError,
      onDone: () {
        _onRelease();
        unawaited(_controller.close());
      },
    );
    if (_mayActivate()) {
      _onActivate();
    } else {
      unawaited(cancel());
    }
  }

  Future<void> cancel() async {
    _onRelease();
    final subscription = _subscription;
    _subscription = null;
    await subscription?.cancel();
    if (!_controller.isClosed) await _controller.close();
  }
}
