package types

import (
	"encoding/json"
	"fmt"

	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/internal/jsonnumber"
)

// Capability declarations describe an agent's advertised behavior. They do
// not themselves implement transports, persistence, execution, or delegation.

// SubAgentInfo describes a sub-agent that a parent agent can invoke.
type SubAgentInfo struct {
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
}

// IdentityCapabilities contains agent metadata used for discovery and display.
type IdentityCapabilities struct {
	Name             *string         `json:"name,omitempty"`
	Type             *string         `json:"type,omitempty"`
	Description      *string         `json:"description,omitempty"`
	Version          *string         `json:"version,omitempty"`
	Provider         *string         `json:"provider,omitempty"`
	DocumentationURL *string         `json:"documentationUrl,omitempty"`
	Metadata         *map[string]any `json:"metadata,omitempty"`
}

// TransportCapabilities describes supported connection mechanisms.
type TransportCapabilities struct {
	Streaming         *bool `json:"streaming,omitempty"`
	Websocket         *bool `json:"websocket,omitempty"`
	HTTPBinary        *bool `json:"httpBinary,omitempty"`
	PushNotifications *bool `json:"pushNotifications,omitempty"`
	Resumable         *bool `json:"resumable,omitempty"`
}

// ToolsCapabilities describes tool calling and agent-provided tools.
type ToolsCapabilities struct {
	Supported      *bool   `json:"supported,omitempty"`
	Items          *[]Tool `json:"items,omitempty"`
	ParallelCalls  *bool   `json:"parallelCalls,omitempty"`
	ClientProvided *bool   `json:"clientProvided,omitempty"`
}

// OutputCapabilities describes supported output formats.
type OutputCapabilities struct {
	StructuredOutput   *bool     `json:"structuredOutput,omitempty"`
	SupportedMimeTypes *[]string `json:"supportedMimeTypes,omitempty"`
}

// StateCapabilities describes state and memory support.
type StateCapabilities struct {
	Snapshots       *bool `json:"snapshots,omitempty"`
	Deltas          *bool `json:"deltas,omitempty"`
	Memory          *bool `json:"memory,omitempty"`
	PersistentState *bool `json:"persistentState,omitempty"`
}

// MultiAgentCapabilities describes coordination with other agents.
type MultiAgentCapabilities struct {
	Supported  *bool           `json:"supported,omitempty"`
	Delegation *bool           `json:"delegation,omitempty"`
	Handoffs   *bool           `json:"handoffs,omitempty"`
	SubAgents  *[]SubAgentInfo `json:"subAgents,omitempty"`
}

// ReasoningCapabilities describes visible reasoning support.
type ReasoningCapabilities struct {
	Supported *bool `json:"supported,omitempty"`
	Streaming *bool `json:"streaming,omitempty"`
	Encrypted *bool `json:"encrypted,omitempty"`
}

// MultimodalInputCapabilities describes accepted input modalities.
type MultimodalInputCapabilities struct {
	Image *bool `json:"image,omitempty"`
	Audio *bool `json:"audio,omitempty"`
	Video *bool `json:"video,omitempty"`
	PDF   *bool `json:"pdf,omitempty"`
	File  *bool `json:"file,omitempty"`
}

// MultimodalOutputCapabilities describes produced output modalities.
type MultimodalOutputCapabilities struct {
	Image *bool `json:"image,omitempty"`
	Audio *bool `json:"audio,omitempty"`
}

// MultimodalCapabilities groups input and output modality declarations.
type MultimodalCapabilities struct {
	Input  *MultimodalInputCapabilities  `json:"input,omitempty"`
	Output *MultimodalOutputCapabilities `json:"output,omitempty"`
}

// ExecutionCapabilities describes execution support and integral limits.
//
// The limits use int64 to represent integral values shared with Python and
// TypeScript. Exact TypeScript integer round trips are limited to 2^53-1;
// fractional TypeScript values are outside the shared integral domain and are
// rejected by encoding/json when decoded into this type.
type ExecutionCapabilities struct {
	CodeExecution    *bool  `json:"codeExecution,omitempty"`
	Sandboxed        *bool  `json:"sandboxed,omitempty"`
	MaxIterations    *int64 `json:"maxIterations,omitempty"`
	MaxExecutionTime *int64 `json:"maxExecutionTime,omitempty"`
}

// UnmarshalJSON accepts all exact integral JSON number spellings for execution
// limits, including decimal and exponent notation. Unknown fields retain the
// package's normal permissive encoding/json behavior.
func (c *ExecutionCapabilities) UnmarshalJSON(data []byte) error {
	type executionCapabilitiesAlias ExecutionCapabilities
	var wire struct {
		executionCapabilitiesAlias
		MaxIterations    json.RawMessage `json:"maxIterations"`
		MaxExecutionTime json.RawMessage `json:"maxExecutionTime"`
	}
	wire.executionCapabilitiesAlias = executionCapabilitiesAlias(*c)
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	decoded := ExecutionCapabilities(wire.executionCapabilitiesAlias)

	var err error
	if len(wire.MaxIterations) != 0 {
		decoded.MaxIterations, err = decodeIntegralLimit(wire.MaxIterations)
		if err != nil {
			return fmt.Errorf("maxIterations: %w", err)
		}
	}
	if len(wire.MaxExecutionTime) != 0 {
		decoded.MaxExecutionTime, err = decodeIntegralLimit(wire.MaxExecutionTime)
		if err != nil {
			return fmt.Errorf("maxExecutionTime: %w", err)
		}
	}
	*c = decoded
	return nil
}

func decodeIntegralLimit(raw json.RawMessage) (*int64, error) {
	if string(raw) == "null" {
		return nil, nil
	}
	value, err := jsonnumber.Int64(raw)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

// HumanInTheLoopCapabilities describes supported human interactions.
type HumanInTheLoopCapabilities struct {
	Supported        *bool `json:"supported,omitempty"`
	Approvals        *bool `json:"approvals,omitempty"`
	Interventions    *bool `json:"interventions,omitempty"`
	Feedback         *bool `json:"feedback,omitempty"`
	Interrupts       *bool `json:"interrupts,omitempty"`
	ApproveWithEdits *bool `json:"approveWithEdits,omitempty"`
}

// AgentCapabilities is a categorized, descriptive snapshot of an agent's
// capabilities. An omitted field means unknown, rather than false.
type AgentCapabilities struct {
	Identity       *IdentityCapabilities       `json:"identity,omitempty"`
	Transport      *TransportCapabilities      `json:"transport,omitempty"`
	Tools          *ToolsCapabilities          `json:"tools,omitempty"`
	Output         *OutputCapabilities         `json:"output,omitempty"`
	State          *StateCapabilities          `json:"state,omitempty"`
	MultiAgent     *MultiAgentCapabilities     `json:"multiAgent,omitempty"`
	Reasoning      *ReasoningCapabilities      `json:"reasoning,omitempty"`
	Multimodal     *MultimodalCapabilities     `json:"multimodal,omitempty"`
	Execution      *ExecutionCapabilities      `json:"execution,omitempty"`
	HumanInTheLoop *HumanInTheLoopCapabilities `json:"humanInTheLoop,omitempty"`
	Custom         *map[string]any             `json:"custom,omitempty"`
}
