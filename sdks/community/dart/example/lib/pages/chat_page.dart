import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:ag_ui/ag_ui.dart';
import 'package:file_picker/file_picker.dart';
import '../models/chat_message.dart';
import '../models/endpoint_config.dart';
import '../services/ag_ui_service.dart';
import '../services/ids.dart';
import 'agui_event_handling.dart';
import '../widgets/chat_message_widget.dart';
import '../widgets/chat_input_widget.dart';

class ChatPage extends StatelessWidget {
  final EndpointConfig endpoint;

  const ChatPage({super.key, required this.endpoint});

  @override
  Widget build(BuildContext context) {
    return ChangeNotifierProvider(
      create: (_) => ChatPageState(endpoint: endpoint),
      child: const ChatPageView(),
    );
  }
}

class ChatPageView extends StatelessWidget {
  const ChatPageView({super.key});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final state = context.watch<ChatPageState>();

    return Scaffold(
      backgroundColor: theme.colorScheme.surface,
      appBar: AppBar(
        title: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(state.endpoint.name, style: theme.textTheme.titleMedium),
            Text(
              state.endpoint.description,
              style: theme.textTheme.bodySmall?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
          ],
        ),
        actions: [
          if (state.connectionStatus != ConnectionStatus.disconnected)
            Padding(
              padding: const EdgeInsets.only(right: 16),
              child: Icon(
                _getConnectionIcon(state.connectionStatus),
                color: _getConnectionColor(state.connectionStatus, theme),
                size: 20,
              ),
            ),
        ],
        bottom: state.isLoading
            ? PreferredSize(
                preferredSize: const Size.fromHeight(2),
                child: LinearProgressIndicator(
                  backgroundColor: Colors.transparent,
                  valueColor: AlwaysStoppedAnimation<Color>(
                    theme.colorScheme.primary,
                  ),
                ),
              )
            : null,
      ),
      body: Column(
        children: [
          Expanded(
            child: state.messages.isEmpty
                ? Center(
                    child: Column(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        Icon(
                          state.endpoint.icon,
                          size: 64,
                          color: theme.colorScheme.outline,
                        ),
                        const SizedBox(height: 16),
                        Text(
                          'Start a conversation',
                          style: theme.textTheme.titleLarge?.copyWith(
                            color: theme.colorScheme.outline,
                          ),
                        ),
                        const SizedBox(height: 8),
                        Text(
                          'Send a message to begin',
                          style: theme.textTheme.bodyMedium?.copyWith(
                            color: theme.colorScheme.outline,
                          ),
                        ),
                      ],
                    ),
                  )
                : ListView.builder(
                    reverse: true,
                    padding: const EdgeInsets.symmetric(vertical: 16),
                    itemCount: state.messages.length,
                    itemBuilder: (context, index) {
                      final reversedIndex = state.messages.length - 1 - index;
                      return ChatMessageWidget(
                        message: state.messages[reversedIndex],
                      );
                    },
                  ),
          ),
          ChatInputWidget(
            onSendMessage: state.sendMessage,
            isEnabled: !state.isLoading,
          ),
        ],
      ),
    );
  }

  IconData _getConnectionIcon(ConnectionStatus status) {
    switch (status) {
      case ConnectionStatus.connected:
        return Icons.check_circle;
      case ConnectionStatus.connecting:
        return Icons.sync;
      case ConnectionStatus.error:
        return Icons.error;
      case ConnectionStatus.disconnected:
        return Icons.circle_outlined;
    }
  }

  Color _getConnectionColor(ConnectionStatus status, ThemeData theme) {
    switch (status) {
      case ConnectionStatus.connected:
        return Colors.green;
      case ConnectionStatus.connecting:
        return theme.colorScheme.primary;
      case ConnectionStatus.error:
        return theme.colorScheme.error;
      case ConnectionStatus.disconnected:
        return theme.colorScheme.outline;
    }
  }
}

class ChatPageState extends ChangeNotifier with AgUiEventHandling {
  final EndpointConfig endpoint;
  final AgUiService _service;
  bool _isLoading = false;
  bool _disposed = false;
  ConnectionStatus _connectionStatus = ConnectionStatus.disconnected;
  final List<Message> _history = [];
  final String _threadId = uid('thread');
  late final StreamSubscription<ConnectionStatus> _connectionSubscription;

  ChatPageState({required this.endpoint, AgUiService? service})
    : _service = service ?? AgUiService() {
    _connectionSubscription = _service.connectionStatus.listen((status) {
      if (_disposed) return;
      _connectionStatus = status;
      notifyListeners();
    });
  }

  bool get isLoading => _isLoading;
  ConnectionStatus get connectionStatus => _connectionStatus;

  void sendMessage(String text) async {
    final trimmed = text.trim();
    if (trimmed.isEmpty || _isLoading || _disposed) return;
    beginRun();
    final userId = uid('user');

    final userMessage = ChatMessage(
      id: userId,
      type: ChatMessageType.user,
      content: trimmed,
      timestamp: DateTime.now(),
    );

    messages.add(userMessage);
    _history.add(UserMessage(id: userId, content: trimmed));
    _isLoading = true;
    notifyListeners();

    try {
      final outgoingHistory = List<Message>.unmodifiable(_history);
      await for (final event in _service.run(
        endpoint.path,
        threadId: _threadId,
        messages: outgoingHistory,
      )) {
        if (_disposed) break;
        _handleEvent(event);
        if (runIsTerminal) break;
      }
      if (!runIsTerminal && !_disposed) {
        finishStreaming();
        messages.add(
          ChatMessage(
            id: uid('interrupted'),
            type: ChatMessageType.system,
            content: 'Connection closed before the run finished.',
            timestamp: DateTime.now(),
          ),
        );
      }
    } catch (e) {
      if (!_disposed) {
        messages.add(
          ChatMessage(
            id: 'error_${DateTime.now().millisecondsSinceEpoch}',
            type: ChatMessageType.system,
            content: 'Error: ${e.toString()}',
            timestamp: DateTime.now(),
          ),
        );
      }
    } finally {
      if (!_disposed) {
        _isLoading = false;
        finishStreaming();
        notifyListeners();
      }
    }
  }

  void _handleEvent(BaseEvent event) {
    final handled = handleCommonEvent(event);
    if (handled) {
      if (event is RunErrorEvent) _isLoading = false;
      notifyListeners();
      return;
    }
    if (event is ToolCallResultEvent) {
      _updateTool(event.toolCallId, result: event.content, isStreaming: false);
    } else if (event is ToolCallStartEvent) {
      _updateTool(
        event.toolCallId,
        name: event.toolCallName,
        isStreaming: true,
      );
    } else if (event is ToolCallArgsEvent) {
      _updateTool(event.toolCallId, argsDelta: event.delta);
    } else if (event is ToolCallEndEvent) {
      _updateTool(event.toolCallId, isStreaming: false);
    } else if (event is MessagesSnapshotEvent) {
      reconcileSnapshot(event.messages);
      for (final message in event.messages) {
        if (message is AssistantMessage || message is ReasoningMessage) {
          _replaceHistory(message);
        }
        if (message is! AssistantMessage) continue;
        for (final toolCall in message.toolCalls ?? <ToolCall>[]) {
          _updateTool(
            toolCall.id,
            name: toolCall.function.name,
            args: toolCall.function.arguments,
            isStreaming: false,
          );
        }
      }
    } else if (event is CustomEvent && event.name == 'image_generated') {
      final value = event.value;
      final url = value is Map && value['url'] is String
          ? value['url'] as String
          : null;
      if (url != null && _isValidImageDataUrl(url)) {
        messages.add(
          ChatMessage(
            id: 'image_${DateTime.now().millisecondsSinceEpoch}',
            type: ChatMessageType.image,
            content: url,
            timestamp: DateTime.now(),
          ),
        );
      } else {
        messages.add(
          ChatMessage(
            id: uid('error'),
            type: ChatMessageType.system,
            content: 'The image response was invalid.',
            timestamp: DateTime.now(),
          ),
        );
      }
    } else if (event is StateSnapshotEvent) {
      final snapshot = event.snapshot;
      if (snapshot != null && snapshot is Map) {
        if (snapshot.containsKey('steps') || snapshot.containsKey('content')) {
          String stateContent = '📊 UI State Generated:\n';
          if (snapshot.containsKey('steps') && snapshot['steps'] is List) {
            final steps = snapshot['steps'] as List;
            stateContent += 'Progress Steps:\n';
            for (int i = 0; i < steps.length; i++) {
              final step = steps[i];
              if (step is Map) {
                final description = step['description'] ?? 'Step ${i + 1}';
                final status = step['status'] ?? 'pending';
                final statusIcon = status == 'completed'
                    ? '✅'
                    : status == 'in_progress'
                    ? '🔄'
                    : status == 'enabled'
                    ? '⚡'
                    : '⏳';
                stateContent += '  $statusIcon $description\n';
              }
            }
          } else {
            stateContent += snapshot['content'].toString();
          }
          messages.add(
            ChatMessage(
              id: 'state_${DateTime.now().millisecondsSinceEpoch}',
              type: ChatMessageType.system,
              content: stateContent,
              timestamp: DateTime.now(),
            ),
          );
        }
        // Unknown snapshot shapes (e.g. image-gen {status, prompt}) are silently ignored.
      }
    } else if (event is StateDeltaEvent) {
      // Handle state delta updates
      final delta = event.delta;
      if (delta.isNotEmpty) {
        // Find the last state message to update
        final stateMessages = messages
            .where(
              (m) =>
                  m.type == ChatMessageType.system &&
                  m.content.startsWith('📊'),
            )
            .toList();

        if (stateMessages.isNotEmpty) {
          final lastState = stateMessages.last;
          final index = messages.indexOf(lastState);

          // Apply JSON patch operations to show what changed
          String updateInfo = '';
          for (final op in delta) {
            final operation = op['op'] ?? '';
            final path = op['path'] ?? '';
            final value = op['value'];

            if (operation == 'replace' && path.contains('/status')) {
              // Extract step number from path like "/steps/0/status"
              final stepMatch = RegExp(r'/steps/(\d+)/status').firstMatch(path);
              if (stepMatch != null) {
                final stepNum = int.parse(stepMatch.group(1)!) + 1;
                final statusIcon = value == 'completed'
                    ? '✅'
                    : value == 'in_progress'
                    ? '🔄'
                    : value == 'enabled'
                    ? '⚡'
                    : '⏳';
                updateInfo += '\n  $statusIcon Step $stepNum → $value';
              }
            }
          }

          if (updateInfo.isNotEmpty && index != -1) {
            messages[index] = lastState.copyWith(
              content: lastState.content + updateInfo,
            );
          }
        }
      }
    } else if (event is RunFinishedEvent || event is RunStartedEvent) {
      // These are lifecycle events, we can ignore them or show status
    }

    notifyListeners();
  }

  void _updateTool(
    String id, {
    String? name,
    String? args,
    String? argsDelta,
    String? result,
    bool? isStreaming,
  }) {
    final index = messages.indexWhere((message) => message.id == id);
    final previous = index < 0 ? null : messages[index];
    final toolName = name ?? previous?.toolName ?? 'Tool';
    final toolArgs = args ?? '${previous?.toolArgs ?? ''}${argsDelta ?? ''}';
    final toolResult = result ?? previous?.toolResult;
    final message = ChatMessage(
      id: id,
      type: ChatMessageType.tool,
      content: [
        '🔧 Tool: $toolName',
        if (toolArgs.isNotEmpty) 'Arguments: $toolArgs',
        if (toolResult != null) 'Result: $toolResult',
      ].join('\n'),
      timestamp: previous?.timestamp ?? DateTime.now(),
      isStreaming: isStreaming ?? previous?.isStreaming ?? false,
      toolName: toolName,
      toolArgs: toolArgs,
      toolResult: toolResult,
    );
    if (index < 0) {
      messages.add(message);
    } else {
      messages[index] = message;
    }
  }

  void _replaceHistory(Message message) {
    final id = message.id;
    if (id == null) return;
    final index = _history.indexWhere((item) => item.id == id);
    if (index < 0) {
      _history.add(message);
    } else {
      _history[index] = message;
    }
  }

  bool _isValidImageDataUrl(String value) {
    try {
      final uri = Uri.parse(value);
      if (uri.scheme != 'data') return false;
      final data = UriData.fromUri(uri);
      return data.mimeType.startsWith('image/') &&
          data.contentAsBytes().isNotEmpty;
    } on Object {
      return false;
    }
  }

  @override
  void dispose() {
    _disposed = true;
    disposed = true;
    _connectionSubscription.cancel();
    _service.dispose();
    super.dispose();
  }
}

class MultimodalChatPageState extends ChatPageState {
  Uint8List? _pickedBytes;
  String? _pickedFileName;
  String? _pickedMimeType;
  int _pickGeneration = 0;
  final Future<PlatformFile?> Function(List<String> allowedExtensions)
  _pickFile;

  MultimodalChatPageState({
    required super.endpoint,
    super.service,
    Future<PlatformFile?> Function(List<String> allowedExtensions)? filePicker,
  }) : _pickFile = filePicker ?? _defaultPickFile;

  static Future<PlatformFile?> _defaultPickFile(List<String> extensions) async {
    final result = await FilePicker.platform.pickFiles(
      type: FileType.custom,
      allowedExtensions: extensions,
      withData: true,
    );
    return result == null || result.files.isEmpty ? null : result.files.first;
  }

  Uint8List? get pickedBytes => _pickedBytes;
  String? get pickedFileName => _pickedFileName;
  String? get pickedMimeType => _pickedMimeType;
  bool get hasFile => _pickedBytes != null;

  Future<void> pickFile(
    List<String> allowedExtensions,
    BuildContext context,
  ) async {
    final generation = ++_pickGeneration;
    PlatformFile? file;
    try {
      file = await _pickFile(List<String>.unmodifiable(allowedExtensions));
    } catch (error) {
      if (!_disposed && generation == _pickGeneration) {
        messages.add(
          ChatMessage(
            id: uid('error'),
            type: ChatMessageType.system,
            content: 'Could not pick file: $error',
            timestamp: DateTime.now(),
          ),
        );
        notifyListeners();
      }
      return;
    }
    if (_disposed || generation != _pickGeneration || file == null) return;
    final bytes = file.bytes;
    if (bytes == null) return;

    const maxBytes = 5 * 1024 * 1024;
    if (bytes.length > maxBytes) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('File too large — pick a file under 5 MB'),
          ),
        );
      }
      return;
    }

    _pickedBytes = bytes;
    _pickedFileName = file.name;
    _pickedMimeType = _mimeTypeFor(file.extension ?? '');
    notifyListeners();
  }

  void clearPicked() {
    if (_disposed) return;
    _pickGeneration++;
    _pickedBytes = null;
    _pickedFileName = null;
    _pickedMimeType = null;
    notifyListeners();
  }

  @override
  void dispose() {
    _pickGeneration++;
    super.dispose();
  }

  void sendMultimodal(String questionText) async {
    if (_pickedBytes == null) return;
    if (_isLoading || _disposed) return;
    beginRun();

    final bytes = _pickedBytes!;
    final mime = _pickedMimeType ?? 'application/octet-stream';
    final fileName = _pickedFileName ?? 'file';
    final b64 = base64Encode(bytes);
    final source = DataSource(value: b64, mimeType: mime);

    final List<InputContent> parts;
    switch (endpoint.path) {
      case 'vision':
        parts = [
          ImageInputContent(source: source),
          if (questionText.trim().isNotEmpty)
            TextInputContent(questionText.trim()),
        ];
      case 'audio':
        parts = [AudioInputContent(source: source)];
      case 'document':
        parts = [
          DocumentInputContent(source: source),
          if (questionText.trim().isNotEmpty)
            TextInputContent(questionText.trim()),
        ];
      default:
        parts = [ImageInputContent(source: source)];
    }

    switch (endpoint.path) {
      case 'vision':
        messages.add(
          ChatMessage(
            id: 'attachment_${DateTime.now().millisecondsSinceEpoch}',
            type: ChatMessageType.imageAttachment,
            content: fileName,
            imageBytes: bytes,
            timestamp: DateTime.now(),
          ),
        );
      case 'audio':
        messages.add(
          ChatMessage(
            id: 'attachment_${DateTime.now().millisecondsSinceEpoch}',
            type: ChatMessageType.audioAttachment,
            content: fileName,
            fileName: fileName,
            timestamp: DateTime.now(),
          ),
        );
      case 'document':
        messages.add(
          ChatMessage(
            id: 'attachment_${DateTime.now().millisecondsSinceEpoch}',
            type: ChatMessageType.documentAttachment,
            content: fileName,
            fileName: fileName,
            timestamp: DateTime.now(),
          ),
        );
    }

    _isLoading = true;
    clearPicked();
    notifyListeners();

    try {
      await for (final event in _service.sendMultimodalMessage(
        endpoint.path,
        parts,
      )) {
        if (_disposed) break;
        _handleEvent(event);
        if (runIsTerminal) break;
      }
      if (!runIsTerminal && !_disposed) {
        finishStreaming();
        messages.add(
          ChatMessage(
            id: uid('interrupted'),
            type: ChatMessageType.system,
            content: 'Connection closed before the run finished.',
            timestamp: DateTime.now(),
          ),
        );
      }
    } catch (e) {
      if (!_disposed) {
        messages.add(
          ChatMessage(
            id: 'error_${DateTime.now().millisecondsSinceEpoch}',
            type: ChatMessageType.system,
            content: 'Error: ${e.toString()}',
            timestamp: DateTime.now(),
          ),
        );
      }
    } finally {
      if (!_disposed) {
        _isLoading = false;
        finishStreaming();
        notifyListeners();
      }
    }
  }

  static String _mimeTypeFor(String ext) =>
      const {
        'jpg': 'image/jpeg',
        'jpeg': 'image/jpeg',
        'png': 'image/png',
        'gif': 'image/gif',
        'webp': 'image/webp',
        'mp3': 'audio/mpeg',
        'wav': 'audio/wav',
        'm4a': 'audio/mp4',
        'ogg': 'audio/ogg',
        'webm': 'audio/webm',
        'pdf': 'application/pdf',
      }[ext.toLowerCase()] ??
      'application/octet-stream';
}
