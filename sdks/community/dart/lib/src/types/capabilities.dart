/// Typed capability declarations shared by AG-UI implementations.
library;

import 'base.dart';
import 'copy_utils.dart';
import 'tool.dart';

Object? _value(Map<String, dynamic> json, String camel, [String? snake]) =>
    json.containsKey(camel) ? json[camel] : json[snake ?? camel];

AGUIValidationError _invalid(String field, Object? value) =>
    AGUIValidationError(
      message: 'Capability field has an invalid type',
      field: field,
      value: value?.runtimeType.toString(),
    );

Map<String, dynamic> _typedMap(Object? value, String field) {
  if (value is! Map) {
    throw _invalid(field, value);
  }
  try {
    return Map<String, dynamic>.from(value);
  } on Object catch (_) {
    throw _invalid(field, value);
  }
}

String _childField(String field, String child) =>
    field.isEmpty ? child : '$field.$child';

Object? _jsonValue(Object? value, String field) {
  if (value == null || value is String || value is bool) {
    return value;
  }
  if (value is num) {
    if (!value.isFinite) {
      throw _invalid(field, value);
    }
    return value;
  }
  if (value is List) {
    return [
      for (var index = 0; index < value.length; index++)
        _jsonValue(value[index], '$field[$index]'),
    ];
  }
  if (value is Map) {
    final result = <String, dynamic>{};
    for (final entry in value.entries) {
      if (entry.key is! String) {
        throw _invalid(field, entry.key);
      }
      final key = entry.key as String;
      result[key] = _jsonValue(entry.value, _childField(field, key));
    }
    return result;
  }
  throw _invalid(field, value);
}

Map<String, dynamic> _jsonMapValue(Object? value, String field) {
  final typed = _typedMap(value, field);
  return _jsonValue(typed, field)! as Map<String, dynamic>;
}

T? _scalar<T>(Map<String, dynamic> json, String camel, [String? snake]) {
  final value = _value(json, camel, snake);
  if (value == null) {
    return null;
  }
  if (value is! T) {
    throw _invalid(camel, value);
  }
  return value as T;
}

T? _model<T>(
  Map<String, dynamic> json,
  String camel,
  T Function(Map<String, dynamic>) decode, [
  String? snake,
]) {
  final value = _value(json, camel, snake);
  if (value == null) {
    return null;
  }
  final typed = _typedMap(value, camel);
  try {
    return decode(typed);
  } on AGUIValidationError catch (error) {
    throw AGUIValidationError(
      message: error.message,
      field: error.field == null ? camel : '$camel.${error.field}',
      value: error.value,
    );
  }
}

List<T>? _list<T>(
  Map<String, dynamic> json,
  String camel,
  T Function(Object?, int) decode, [
  String? snake,
]) {
  final value = _value(json, camel, snake);
  if (value == null) {
    return null;
  }
  if (value is! List) {
    throw _invalid(camel, value);
  }
  final decoded = <T>[];
  for (var index = 0; index < value.length; index++) {
    try {
      decoded.add(decode(value[index], index));
    } on AGUIValidationError catch (error) {
      throw AGUIValidationError(
        message: error.message,
        field: error.field == null || error.field!.isEmpty
            ? '$camel[$index]'
            : '$camel[$index].${error.field}',
        value: error.value,
      );
    }
  }
  return decoded;
}

Map<String, dynamic>? _map(Map<String, dynamic> json, String field) {
  final value = json[field];
  if (value == null) {
    return null;
  }
  return _jsonMapValue(value, field);
}

final _minimumIntegralLimit = BigInt.parse('-9223372036854775808');
final _maximumIntegralLimit = BigInt.parse('9223372036854775807');

int? _integralLimit(Map<String, dynamic> json, String camel, String snake) {
  return _integralValue(_value(json, camel, snake), camel);
}

int? _integralValue(Object? value, String field) {
  if (value == null) {
    return null;
  }
  if (value is! num || !value.isFinite) {
    throw _invalid(field, value);
  }
  if (value is int) {
    final integer = BigInt.from(value);
    if (integer < _minimumIntegralLimit || integer > _maximumIntegralLimit) {
      throw _invalid(field, value);
    }
    return value;
  }
  if (value is! double ||
      value >= 9223372036854775808.0 ||
      value < -9223372036854775808.0 ||
      value.truncateToDouble() != value) {
    throw _invalid(field, value);
  }
  final integer = value.toInt();
  final exact = BigInt.from(integer);
  if (exact < _minimumIntegralLimit || exact > _maximumIntegralLimit) {
    throw _invalid(field, value);
  }
  return integer;
}

/// Describes a sub-agent that a parent agent can invoke.
final class SubAgentInfo extends AGUIModel {
  const SubAgentInfo({required this.name, this.description});

  factory SubAgentInfo.fromJson(Map<String, dynamic> json) => SubAgentInfo(
        name: JsonDecoder.requireField<String>(json, 'name'),
        description: _scalar<String>(json, 'description'),
      );

  final String name;
  final String? description;

  @override
  Map<String, dynamic> toJson() => {
        'name': name,
        if (description != null) 'description': description,
      };

  @override
  SubAgentInfo copyWith({String? name, Object? description = kUnsetSentinel}) =>
      SubAgentInfo(
        name: name ?? this.name,
        description: resolveNullableCopy(description, this.description),
      );
}

/// Basic agent metadata used by discovery and display UIs.
final class IdentityCapabilities extends AGUIModel {
  const IdentityCapabilities({
    this.name,
    this.type,
    this.description,
    this.version,
    this.provider,
    this.documentationUrl,
    this.metadata,
  });

  factory IdentityCapabilities.fromJson(Map<String, dynamic> json) =>
      IdentityCapabilities(
        name: _scalar<String>(json, 'name'),
        type: _scalar<String>(json, 'type'),
        description: _scalar<String>(json, 'description'),
        version: _scalar<String>(json, 'version'),
        provider: _scalar<String>(json, 'provider'),
        documentationUrl: _scalar<String>(
          json,
          'documentationUrl',
          'documentation_url',
        ),
        metadata: _map(json, 'metadata'),
      );

  final String? name;
  final String? type;
  final String? description;
  final String? version;
  final String? provider;
  final String? documentationUrl;
  final Map<String, dynamic>? metadata;

  @override
  Map<String, dynamic> toJson() => {
        if (name != null) 'name': name,
        if (type != null) 'type': type,
        if (description != null) 'description': description,
        if (version != null) 'version': version,
        if (provider != null) 'provider': provider,
        if (documentationUrl != null) 'documentationUrl': documentationUrl,
        if (metadata != null) 'metadata': _jsonMapValue(metadata, 'metadata'),
      };

  @override
  IdentityCapabilities copyWith({
    Object? name = kUnsetSentinel,
    Object? type = kUnsetSentinel,
    Object? description = kUnsetSentinel,
    Object? version = kUnsetSentinel,
    Object? provider = kUnsetSentinel,
    Object? documentationUrl = kUnsetSentinel,
    Object? metadata = kUnsetSentinel,
  }) =>
      IdentityCapabilities(
        name: resolveNullableCopy(name, this.name),
        type: resolveNullableCopy(type, this.type),
        description: resolveNullableCopy(description, this.description),
        version: resolveNullableCopy(version, this.version),
        provider: resolveNullableCopy(provider, this.provider),
        documentationUrl: resolveNullableCopy(
          documentationUrl,
          this.documentationUrl,
        ),
        metadata: resolveNullableCopy(metadata, this.metadata),
      );
}

/// Supported connection mechanisms.
final class TransportCapabilities extends AGUIModel {
  const TransportCapabilities({
    this.streaming,
    this.websocket,
    this.httpBinary,
    this.pushNotifications,
    this.resumable,
  });

  factory TransportCapabilities.fromJson(Map<String, dynamic> json) =>
      TransportCapabilities(
        streaming: _scalar<bool>(json, 'streaming'),
        websocket: _scalar<bool>(json, 'websocket'),
        httpBinary: _scalar<bool>(json, 'httpBinary', 'http_binary'),
        pushNotifications: _scalar<bool>(
          json,
          'pushNotifications',
          'push_notifications',
        ),
        resumable: _scalar<bool>(json, 'resumable'),
      );

  final bool? streaming;
  final bool? websocket;
  final bool? httpBinary;
  final bool? pushNotifications;
  final bool? resumable;

  @override
  Map<String, dynamic> toJson() => {
        if (streaming != null) 'streaming': streaming,
        if (websocket != null) 'websocket': websocket,
        if (httpBinary != null) 'httpBinary': httpBinary,
        if (pushNotifications != null) 'pushNotifications': pushNotifications,
        if (resumable != null) 'resumable': resumable,
      };

  @override
  TransportCapabilities copyWith({
    Object? streaming = kUnsetSentinel,
    Object? websocket = kUnsetSentinel,
    Object? httpBinary = kUnsetSentinel,
    Object? pushNotifications = kUnsetSentinel,
    Object? resumable = kUnsetSentinel,
  }) =>
      TransportCapabilities(
        streaming: resolveNullableCopy(streaming, this.streaming),
        websocket: resolveNullableCopy(websocket, this.websocket),
        httpBinary: resolveNullableCopy(httpBinary, this.httpBinary),
        pushNotifications: resolveNullableCopy(
          pushNotifications,
          this.pushNotifications,
        ),
        resumable: resolveNullableCopy(resumable, this.resumable),
      );
}

/// Tool calling and agent-provided tool declarations.
final class ToolsCapabilities extends AGUIModel {
  const ToolsCapabilities({
    this.supported,
    this.items,
    this.parallelCalls,
    this.clientProvided,
  });

  factory ToolsCapabilities.fromJson(Map<String, dynamic> json) =>
      ToolsCapabilities(
        supported: _scalar<bool>(json, 'supported'),
        items: _list<Tool>(json, 'items', (value, index) {
          final tool = Tool.fromJson(_typedMap(value, ''));
          _jsonMapValue(tool.toJson(), '');
          return tool;
        }),
        parallelCalls: _scalar<bool>(json, 'parallelCalls', 'parallel_calls'),
        clientProvided: _scalar<bool>(
          json,
          'clientProvided',
          'client_provided',
        ),
      );

  final bool? supported;
  final List<Tool>? items;
  final bool? parallelCalls;
  final bool? clientProvided;

  @override
  Map<String, dynamic> toJson() => {
        if (supported != null) 'supported': supported,
        if (items != null)
          'items': [
            for (var index = 0; index < items!.length; index++)
              _jsonMapValue(items![index].toJson(), 'items[$index]'),
          ],
        if (parallelCalls != null) 'parallelCalls': parallelCalls,
        if (clientProvided != null) 'clientProvided': clientProvided,
      };

  @override
  ToolsCapabilities copyWith({
    Object? supported = kUnsetSentinel,
    Object? items = kUnsetSentinel,
    Object? parallelCalls = kUnsetSentinel,
    Object? clientProvided = kUnsetSentinel,
  }) =>
      ToolsCapabilities(
        supported: resolveNullableCopy(supported, this.supported),
        items: resolveNullableCopy(items, this.items),
        parallelCalls: resolveNullableCopy(parallelCalls, this.parallelCalls),
        clientProvided:
            resolveNullableCopy(clientProvided, this.clientProvided),
      );
}

/// Supported output formats.
final class OutputCapabilities extends AGUIModel {
  const OutputCapabilities({this.structuredOutput, this.supportedMimeTypes});

  factory OutputCapabilities.fromJson(Map<String, dynamic> json) =>
      OutputCapabilities(
        structuredOutput: _scalar<bool>(
          json,
          'structuredOutput',
          'structured_output',
        ),
        supportedMimeTypes: _list<String>(
          json,
          'supportedMimeTypes',
          (value, index) {
            if (value is! String) {
              throw _invalid('', value);
            }
            return value;
          },
          'supported_mime_types',
        ),
      );

  final bool? structuredOutput;
  final List<String>? supportedMimeTypes;

  @override
  Map<String, dynamic> toJson() => {
        if (structuredOutput != null) 'structuredOutput': structuredOutput,
        if (supportedMimeTypes != null)
          'supportedMimeTypes': supportedMimeTypes,
      };

  @override
  OutputCapabilities copyWith({
    Object? structuredOutput = kUnsetSentinel,
    Object? supportedMimeTypes = kUnsetSentinel,
  }) =>
      OutputCapabilities(
        structuredOutput: resolveNullableCopy(
          structuredOutput,
          this.structuredOutput,
        ),
        supportedMimeTypes: resolveNullableCopy(
          supportedMimeTypes,
          this.supportedMimeTypes,
        ),
      );
}

/// State and memory support.
final class StateCapabilities extends AGUIModel {
  const StateCapabilities({
    this.snapshots,
    this.deltas,
    this.memory,
    this.persistentState,
  });

  factory StateCapabilities.fromJson(Map<String, dynamic> json) =>
      StateCapabilities(
        snapshots: _scalar<bool>(json, 'snapshots'),
        deltas: _scalar<bool>(json, 'deltas'),
        memory: _scalar<bool>(json, 'memory'),
        persistentState: _scalar<bool>(
          json,
          'persistentState',
          'persistent_state',
        ),
      );

  final bool? snapshots;
  final bool? deltas;
  final bool? memory;
  final bool? persistentState;

  @override
  Map<String, dynamic> toJson() => {
        if (snapshots != null) 'snapshots': snapshots,
        if (deltas != null) 'deltas': deltas,
        if (memory != null) 'memory': memory,
        if (persistentState != null) 'persistentState': persistentState,
      };

  @override
  StateCapabilities copyWith({
    Object? snapshots = kUnsetSentinel,
    Object? deltas = kUnsetSentinel,
    Object? memory = kUnsetSentinel,
    Object? persistentState = kUnsetSentinel,
  }) =>
      StateCapabilities(
        snapshots: resolveNullableCopy(snapshots, this.snapshots),
        deltas: resolveNullableCopy(deltas, this.deltas),
        memory: resolveNullableCopy(memory, this.memory),
        persistentState:
            resolveNullableCopy(persistentState, this.persistentState),
      );
}

/// Coordination with other agents.
final class MultiAgentCapabilities extends AGUIModel {
  const MultiAgentCapabilities({
    this.supported,
    this.delegation,
    this.handoffs,
    this.subAgents,
  });

  factory MultiAgentCapabilities.fromJson(Map<String, dynamic> json) =>
      MultiAgentCapabilities(
        supported: _scalar<bool>(json, 'supported'),
        delegation: _scalar<bool>(json, 'delegation'),
        handoffs: _scalar<bool>(json, 'handoffs'),
        subAgents: _list<SubAgentInfo>(
          json,
          'subAgents',
          (value, index) {
            return SubAgentInfo.fromJson(_typedMap(value, ''));
          },
          'sub_agents',
        ),
      );

  final bool? supported;
  final bool? delegation;
  final bool? handoffs;
  final List<SubAgentInfo>? subAgents;

  @override
  Map<String, dynamic> toJson() => {
        if (supported != null) 'supported': supported,
        if (delegation != null) 'delegation': delegation,
        if (handoffs != null) 'handoffs': handoffs,
        if (subAgents != null)
          'subAgents': subAgents!.map((agent) => agent.toJson()).toList(),
      };

  @override
  MultiAgentCapabilities copyWith({
    Object? supported = kUnsetSentinel,
    Object? delegation = kUnsetSentinel,
    Object? handoffs = kUnsetSentinel,
    Object? subAgents = kUnsetSentinel,
  }) =>
      MultiAgentCapabilities(
        supported: resolveNullableCopy(supported, this.supported),
        delegation: resolveNullableCopy(delegation, this.delegation),
        handoffs: resolveNullableCopy(handoffs, this.handoffs),
        subAgents: resolveNullableCopy(subAgents, this.subAgents),
      );
}

/// Visible or encrypted reasoning support.
final class ReasoningCapabilities extends AGUIModel {
  const ReasoningCapabilities({this.supported, this.streaming, this.encrypted});

  factory ReasoningCapabilities.fromJson(Map<String, dynamic> json) =>
      ReasoningCapabilities(
        supported: _scalar<bool>(json, 'supported'),
        streaming: _scalar<bool>(json, 'streaming'),
        encrypted: _scalar<bool>(json, 'encrypted'),
      );

  final bool? supported;
  final bool? streaming;
  final bool? encrypted;

  @override
  Map<String, dynamic> toJson() => {
        if (supported != null) 'supported': supported,
        if (streaming != null) 'streaming': streaming,
        if (encrypted != null) 'encrypted': encrypted,
      };

  @override
  ReasoningCapabilities copyWith({
    Object? supported = kUnsetSentinel,
    Object? streaming = kUnsetSentinel,
    Object? encrypted = kUnsetSentinel,
  }) =>
      ReasoningCapabilities(
        supported: resolveNullableCopy(supported, this.supported),
        streaming: resolveNullableCopy(streaming, this.streaming),
        encrypted: resolveNullableCopy(encrypted, this.encrypted),
      );
}

/// Modalities accepted as agent input.
final class MultimodalInputCapabilities extends AGUIModel {
  const MultimodalInputCapabilities({
    this.image,
    this.audio,
    this.video,
    this.pdf,
    this.file,
  });

  factory MultimodalInputCapabilities.fromJson(Map<String, dynamic> json) =>
      MultimodalInputCapabilities(
        image: _scalar<bool>(json, 'image'),
        audio: _scalar<bool>(json, 'audio'),
        video: _scalar<bool>(json, 'video'),
        pdf: _scalar<bool>(json, 'pdf'),
        file: _scalar<bool>(json, 'file'),
      );

  final bool? image;
  final bool? audio;
  final bool? video;
  final bool? pdf;
  final bool? file;

  @override
  Map<String, dynamic> toJson() => {
        if (image != null) 'image': image,
        if (audio != null) 'audio': audio,
        if (video != null) 'video': video,
        if (pdf != null) 'pdf': pdf,
        if (file != null) 'file': file,
      };

  @override
  MultimodalInputCapabilities copyWith({
    Object? image = kUnsetSentinel,
    Object? audio = kUnsetSentinel,
    Object? video = kUnsetSentinel,
    Object? pdf = kUnsetSentinel,
    Object? file = kUnsetSentinel,
  }) =>
      MultimodalInputCapabilities(
        image: resolveNullableCopy(image, this.image),
        audio: resolveNullableCopy(audio, this.audio),
        video: resolveNullableCopy(video, this.video),
        pdf: resolveNullableCopy(pdf, this.pdf),
        file: resolveNullableCopy(file, this.file),
      );
}

/// Modalities produced as agent output.
final class MultimodalOutputCapabilities extends AGUIModel {
  const MultimodalOutputCapabilities({this.image, this.audio});

  factory MultimodalOutputCapabilities.fromJson(Map<String, dynamic> json) =>
      MultimodalOutputCapabilities(
        image: _scalar<bool>(json, 'image'),
        audio: _scalar<bool>(json, 'audio'),
      );

  final bool? image;
  final bool? audio;

  @override
  Map<String, dynamic> toJson() => {
        if (image != null) 'image': image,
        if (audio != null) 'audio': audio,
      };

  @override
  MultimodalOutputCapabilities copyWith({
    Object? image = kUnsetSentinel,
    Object? audio = kUnsetSentinel,
  }) =>
      MultimodalOutputCapabilities(
        image: resolveNullableCopy(image, this.image),
        audio: resolveNullableCopy(audio, this.audio),
      );
}

/// Input and output modality declarations.
final class MultimodalCapabilities extends AGUIModel {
  const MultimodalCapabilities({this.input, this.output});

  factory MultimodalCapabilities.fromJson(Map<String, dynamic> json) =>
      MultimodalCapabilities(
        input: _model(json, 'input', MultimodalInputCapabilities.fromJson),
        output: _model(json, 'output', MultimodalOutputCapabilities.fromJson),
      );

  final MultimodalInputCapabilities? input;
  final MultimodalOutputCapabilities? output;

  @override
  Map<String, dynamic> toJson() => {
        if (input != null) 'input': input!.toJson(),
        if (output != null) 'output': output!.toJson(),
      };

  @override
  MultimodalCapabilities copyWith({
    Object? input = kUnsetSentinel,
    Object? output = kUnsetSentinel,
  }) =>
      MultimodalCapabilities(
        input: resolveNullableCopy(input, this.input),
        output: resolveNullableCopy(output, this.output),
      );
}

/// Code execution support and integral limits.
final class ExecutionCapabilities extends AGUIModel {
  factory ExecutionCapabilities({
    bool? codeExecution,
    bool? sandboxed,
    num? maxIterations,
    num? maxExecutionTime,
  }) =>
      ExecutionCapabilities._(
        codeExecution: codeExecution,
        sandboxed: sandboxed,
        maxIterations: _integralValue(maxIterations, 'maxIterations'),
        maxExecutionTime: _integralValue(maxExecutionTime, 'maxExecutionTime'),
      );

  const ExecutionCapabilities._({
    this.codeExecution,
    this.sandboxed,
    this.maxIterations,
    this.maxExecutionTime,
  });

  factory ExecutionCapabilities.fromJson(Map<String, dynamic> json) =>
      ExecutionCapabilities._(
        codeExecution: _scalar<bool>(json, 'codeExecution', 'code_execution'),
        sandboxed: _scalar<bool>(json, 'sandboxed'),
        maxIterations: _integralLimit(json, 'maxIterations', 'max_iterations'),
        maxExecutionTime: _integralLimit(
          json,
          'maxExecutionTime',
          'max_execution_time',
        ),
      );

  final bool? codeExecution;
  final bool? sandboxed;
  final int? maxIterations;
  final int? maxExecutionTime;

  @override
  Map<String, dynamic> toJson() => {
        if (codeExecution != null) 'codeExecution': codeExecution,
        if (sandboxed != null) 'sandboxed': sandboxed,
        if (maxIterations != null) 'maxIterations': maxIterations,
        if (maxExecutionTime != null) 'maxExecutionTime': maxExecutionTime,
      };

  @override
  ExecutionCapabilities copyWith({
    Object? codeExecution = kUnsetSentinel,
    Object? sandboxed = kUnsetSentinel,
    Object? maxIterations = kUnsetSentinel,
    Object? maxExecutionTime = kUnsetSentinel,
  }) =>
      ExecutionCapabilities(
        codeExecution: resolveNullableCopy(codeExecution, this.codeExecution),
        sandboxed: resolveNullableCopy(sandboxed, this.sandboxed),
        maxIterations: _integralValue(
          identical(maxIterations, kUnsetSentinel)
              ? this.maxIterations
              : maxIterations,
          'maxIterations',
        ),
        maxExecutionTime: _integralValue(
          identical(maxExecutionTime, kUnsetSentinel)
              ? this.maxExecutionTime
              : maxExecutionTime,
          'maxExecutionTime',
        ),
      );
}

/// Human approval, intervention, feedback, and interrupt support.
final class HumanInTheLoopCapabilities extends AGUIModel {
  const HumanInTheLoopCapabilities({
    this.supported,
    this.approvals,
    this.interventions,
    this.feedback,
    this.interrupts,
    this.approveWithEdits,
  });

  factory HumanInTheLoopCapabilities.fromJson(Map<String, dynamic> json) =>
      HumanInTheLoopCapabilities(
        supported: _scalar<bool>(json, 'supported'),
        approvals: _scalar<bool>(json, 'approvals'),
        interventions: _scalar<bool>(json, 'interventions'),
        feedback: _scalar<bool>(json, 'feedback'),
        interrupts: _scalar<bool>(json, 'interrupts'),
        approveWithEdits: _scalar<bool>(
          json,
          'approveWithEdits',
          'approve_with_edits',
        ),
      );

  final bool? supported;
  final bool? approvals;
  final bool? interventions;
  final bool? feedback;
  final bool? interrupts;
  final bool? approveWithEdits;

  @override
  Map<String, dynamic> toJson() => {
        if (supported != null) 'supported': supported,
        if (approvals != null) 'approvals': approvals,
        if (interventions != null) 'interventions': interventions,
        if (feedback != null) 'feedback': feedback,
        if (interrupts != null) 'interrupts': interrupts,
        if (approveWithEdits != null) 'approveWithEdits': approveWithEdits,
      };

  @override
  HumanInTheLoopCapabilities copyWith({
    Object? supported = kUnsetSentinel,
    Object? approvals = kUnsetSentinel,
    Object? interventions = kUnsetSentinel,
    Object? feedback = kUnsetSentinel,
    Object? interrupts = kUnsetSentinel,
    Object? approveWithEdits = kUnsetSentinel,
  }) =>
      HumanInTheLoopCapabilities(
        supported: resolveNullableCopy(supported, this.supported),
        approvals: resolveNullableCopy(approvals, this.approvals),
        interventions: resolveNullableCopy(interventions, this.interventions),
        feedback: resolveNullableCopy(feedback, this.feedback),
        interrupts: resolveNullableCopy(interrupts, this.interrupts),
        approveWithEdits: resolveNullableCopy(
          approveWithEdits,
          this.approveWithEdits,
        ),
      );
}

/// Categorized snapshot of an agent's declared capabilities.
final class AgentCapabilities extends AGUIModel {
  const AgentCapabilities({
    this.identity,
    this.transport,
    this.tools,
    this.output,
    this.state,
    this.multiAgent,
    this.reasoning,
    this.multimodal,
    this.execution,
    this.humanInTheLoop,
    this.custom,
  });

  factory AgentCapabilities.fromJson(Map<String, dynamic> json) =>
      AgentCapabilities(
        identity: _model(json, 'identity', IdentityCapabilities.fromJson),
        transport: _model(json, 'transport', TransportCapabilities.fromJson),
        tools: _model(json, 'tools', ToolsCapabilities.fromJson),
        output: _model(json, 'output', OutputCapabilities.fromJson),
        state: _model(json, 'state', StateCapabilities.fromJson),
        multiAgent: _model(
          json,
          'multiAgent',
          MultiAgentCapabilities.fromJson,
          'multi_agent',
        ),
        reasoning: _model(json, 'reasoning', ReasoningCapabilities.fromJson),
        multimodal: _model(json, 'multimodal', MultimodalCapabilities.fromJson),
        execution: _model(json, 'execution', ExecutionCapabilities.fromJson),
        humanInTheLoop: _model(
          json,
          'humanInTheLoop',
          HumanInTheLoopCapabilities.fromJson,
          'human_in_the_loop',
        ),
        custom: _map(json, 'custom'),
      );

  final IdentityCapabilities? identity;
  final TransportCapabilities? transport;
  final ToolsCapabilities? tools;
  final OutputCapabilities? output;
  final StateCapabilities? state;
  final MultiAgentCapabilities? multiAgent;
  final ReasoningCapabilities? reasoning;
  final MultimodalCapabilities? multimodal;
  final ExecutionCapabilities? execution;
  final HumanInTheLoopCapabilities? humanInTheLoop;
  final Map<String, dynamic>? custom;

  @override
  Map<String, dynamic> toJson() => {
        if (identity != null) 'identity': identity!.toJson(),
        if (transport != null) 'transport': transport!.toJson(),
        if (tools != null) 'tools': tools!.toJson(),
        if (output != null) 'output': output!.toJson(),
        if (state != null) 'state': state!.toJson(),
        if (multiAgent != null) 'multiAgent': multiAgent!.toJson(),
        if (reasoning != null) 'reasoning': reasoning!.toJson(),
        if (multimodal != null) 'multimodal': multimodal!.toJson(),
        if (execution != null) 'execution': execution!.toJson(),
        if (humanInTheLoop != null) 'humanInTheLoop': humanInTheLoop!.toJson(),
        if (custom != null) 'custom': _jsonMapValue(custom, 'custom'),
      };

  @override
  AgentCapabilities copyWith({
    Object? identity = kUnsetSentinel,
    Object? transport = kUnsetSentinel,
    Object? tools = kUnsetSentinel,
    Object? output = kUnsetSentinel,
    Object? state = kUnsetSentinel,
    Object? multiAgent = kUnsetSentinel,
    Object? reasoning = kUnsetSentinel,
    Object? multimodal = kUnsetSentinel,
    Object? execution = kUnsetSentinel,
    Object? humanInTheLoop = kUnsetSentinel,
    Object? custom = kUnsetSentinel,
  }) =>
      AgentCapabilities(
        identity: resolveNullableCopy(identity, this.identity),
        transport: resolveNullableCopy(transport, this.transport),
        tools: resolveNullableCopy(tools, this.tools),
        output: resolveNullableCopy(output, this.output),
        state: resolveNullableCopy(state, this.state),
        multiAgent: resolveNullableCopy(multiAgent, this.multiAgent),
        reasoning: resolveNullableCopy(reasoning, this.reasoning),
        multimodal: resolveNullableCopy(multimodal, this.multimodal),
        execution: resolveNullableCopy(execution, this.execution),
        humanInTheLoop:
            resolveNullableCopy(humanInTheLoop, this.humanInTheLoop),
        custom: resolveNullableCopy(custom, this.custom),
      );
}
