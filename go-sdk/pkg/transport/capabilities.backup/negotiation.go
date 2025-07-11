package capabilities

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Negotiator handles capability negotiation between client and server
type Negotiator interface {
	// NegotiateCapabilities performs capability negotiation
	NegotiateCapabilities(ctx context.Context, localCaps Capabilities, remoteCaps Capabilities) (NegotiationResult, error)

	// GetSupportedCapabilities returns the capabilities supported by this negotiator
	GetSupportedCapabilities() Capabilities
}

// NegotiationResult contains the result of capability negotiation
type NegotiationResult struct {
	// Agreed contains the capabilities both sides agreed upon
	Agreed Capabilities

	// LocalOnly contains capabilities only available locally
	LocalOnly Capabilities

	// RemoteOnly contains capabilities only available remotely
	RemoteOnly Capabilities

	// Conflicts contains capabilities that couldn't be resolved
	Conflicts []CapabilityConflict

	// FallbackRequired indicates if fallback to a different transport is needed
	FallbackRequired bool

	// FallbackReason explains why fallback is required
	FallbackReason string
}

// CapabilityConflict represents a conflict between local and remote capabilities
type CapabilityConflict struct {
	// Capability is the name of the conflicting capability
	Capability string

	// LocalValue is the local capability value
	LocalValue interface{}

	// RemoteValue is the remote capability value
	RemoteValue interface{}

	// Reason explains why the conflict occurred
	Reason string
}

// DefaultNegotiator provides default capability negotiation logic
type DefaultNegotiator struct {
	supportedCapabilities transport.Capabilities
	negotiationRules      []NegotiationRule
}

// NegotiationRule defines how to handle capability negotiation
type NegotiationRule interface {
	// CanHandle returns true if this rule can handle the given capability
	CanHandle(capability string) bool

	// Negotiate performs negotiation for a specific capability
	Negotiate(ctx context.Context, capability string, localValue, remoteValue interface{}) (interface{}, error)
}

// NewDefaultNegotiator creates a new default negotiator
func NewDefaultNegotiator(supportedCaps transport.Capabilities) *DefaultNegotiator {
	negotiator := &DefaultNegotiator{
		supportedCapabilities: supportedCaps,
		negotiationRules:      []NegotiationRule{},
	}

	// Add default negotiation rules
	negotiator.AddRule(&BooleanCapabilityRule{})
	negotiator.AddRule(&CompressionRule{})
	negotiator.AddRule(&SecurityRule{})
	negotiator.AddRule(&MessageSizeRule{})
	negotiator.AddRule(&ProtocolVersionRule{})

	return negotiator
}

// AddRule adds a negotiation rule
func (n *DefaultNegotiator) AddRule(rule NegotiationRule) {
	n.negotiationRules = append(n.negotiationRules, rule)
}

// GetSupportedCapabilities returns supported capabilities
func (n *DefaultNegotiator) GetSupportedCapabilities() transport.Capabilities {
	return n.supportedCapabilities
}

// NegotiateCapabilities performs capability negotiation
func (n *DefaultNegotiator) NegotiateCapabilities(ctx context.Context, localCaps, remoteCaps transport.Capabilities) (NegotiationResult, error) {
	result := NegotiationResult{
		Agreed:    transport.Capabilities{},
		LocalOnly: transport.Capabilities{},
		RemoteOnly: transport.Capabilities{},
		Conflicts: []CapabilityConflict{},
	}

	// Negotiate streaming capability
	if err := n.negotiateStreaming(localCaps, remoteCaps, &result); err != nil {
		return result, err
	}

	// Negotiate bidirectional capability
	if err := n.negotiateBidirectional(localCaps, remoteCaps, &result); err != nil {
		return result, err
	}

	// Negotiate compression
	if err := n.negotiateCompression(localCaps, remoteCaps, &result); err != nil {
		return result, err
	}

	// Negotiate multiplexing
	if err := n.negotiateMultiplexing(localCaps, remoteCaps, &result); err != nil {
		return result, err
	}

	// Negotiate reconnection
	if err := n.negotiateReconnection(localCaps, remoteCaps, &result); err != nil {
		return result, err
	}

	// Negotiate message size limits
	if err := n.negotiateMessageSize(localCaps, remoteCaps, &result); err != nil {
		return result, err
	}

	// Negotiate security features
	if err := n.negotiateSecurity(localCaps, remoteCaps, &result); err != nil {
		return result, err
	}

	// Negotiate protocol version
	if err := n.negotiateProtocolVersion(localCaps, remoteCaps, &result); err != nil {
		return result, err
	}

	// Negotiate custom features
	if err := n.negotiateFeatures(localCaps, remoteCaps, &result); err != nil {
		return result, err
	}

	return result, nil
}

// negotiateStreaming negotiates streaming capability
func (n *DefaultNegotiator) negotiateStreaming(local, remote transport.Capabilities, result *NegotiationResult) error {
	if local.Streaming && remote.Streaming {
		result.Agreed.Streaming = true
	} else if local.Streaming && !remote.Streaming {
		result.LocalOnly.Streaming = true
	} else if !local.Streaming && remote.Streaming {
		result.RemoteOnly.Streaming = true
	}
	return nil
}

// negotiateBidirectional negotiates bidirectional capability
func (n *DefaultNegotiator) negotiateBidirectional(local, remote transport.Capabilities, result *NegotiationResult) error {
	if local.Bidirectional && remote.Bidirectional {
		result.Agreed.Bidirectional = true
	} else if local.Bidirectional && !remote.Bidirectional {
		result.LocalOnly.Bidirectional = true
	} else if !local.Bidirectional && remote.Bidirectional {
		result.RemoteOnly.Bidirectional = true
	}
	return nil
}

// negotiateCompression negotiates compression capabilities
func (n *DefaultNegotiator) negotiateCompression(local, remote transport.Capabilities, result *NegotiationResult) error {
	localCompressionSet := make(map[transport.CompressionType]bool)
	for _, comp := range local.Compression {
		localCompressionSet[comp] = true
	}

	var agreed []transport.CompressionType
	var remoteOnly []transport.CompressionType

	for _, remoteComp := range remote.Compression {
		if localCompressionSet[remoteComp] {
			agreed = append(agreed, remoteComp)
		} else {
			remoteOnly = append(remoteOnly, remoteComp)
		}
	}

	var localOnly []transport.CompressionType
	for _, localComp := range local.Compression {
		found := false
		for _, agreedComp := range agreed {
			if localComp == agreedComp {
				found = true
				break
			}
		}
		if !found {
			localOnly = append(localOnly, localComp)
		}
	}

	result.Agreed.Compression = agreed
	result.LocalOnly.Compression = localOnly
	result.RemoteOnly.Compression = remoteOnly

	return nil
}

// negotiateMultiplexing negotiates multiplexing capability
func (n *DefaultNegotiator) negotiateMultiplexing(local, remote transport.Capabilities, result *NegotiationResult) error {
	if local.Multiplexing && remote.Multiplexing {
		result.Agreed.Multiplexing = true
	} else if local.Multiplexing && !remote.Multiplexing {
		result.LocalOnly.Multiplexing = true
	} else if !local.Multiplexing && remote.Multiplexing {
		result.RemoteOnly.Multiplexing = true
	}
	return nil
}

// negotiateReconnection negotiates reconnection capability
func (n *DefaultNegotiator) negotiateReconnection(local, remote transport.Capabilities, result *NegotiationResult) error {
	if local.Reconnection && remote.Reconnection {
		result.Agreed.Reconnection = true
	} else if local.Reconnection && !remote.Reconnection {
		result.LocalOnly.Reconnection = true
	} else if !local.Reconnection && remote.Reconnection {
		result.RemoteOnly.Reconnection = true
	}
	return nil
}

// negotiateMessageSize negotiates message size limits
func (n *DefaultNegotiator) negotiateMessageSize(local, remote transport.Capabilities, result *NegotiationResult) error {
	if local.MaxMessageSize > 0 && remote.MaxMessageSize > 0 {
		// Use the smaller of the two limits
		if local.MaxMessageSize < remote.MaxMessageSize {
			result.Agreed.MaxMessageSize = local.MaxMessageSize
		} else {
			result.Agreed.MaxMessageSize = remote.MaxMessageSize
		}
	} else if local.MaxMessageSize > 0 {
		result.LocalOnly.MaxMessageSize = local.MaxMessageSize
	} else if remote.MaxMessageSize > 0 {
		result.RemoteOnly.MaxMessageSize = remote.MaxMessageSize
	}
	return nil
}

// negotiateSecurity negotiates security features
func (n *DefaultNegotiator) negotiateSecurity(local, remote transport.Capabilities, result *NegotiationResult) error {
	localSecuritySet := make(map[transport.SecurityFeature]bool)
	for _, feature := range local.Security {
		localSecuritySet[feature] = true
	}

	var agreed []transport.SecurityFeature
	var remoteOnly []transport.SecurityFeature

	for _, remoteFeature := range remote.Security {
		if localSecuritySet[remoteFeature] {
			agreed = append(agreed, remoteFeature)
		} else {
			remoteOnly = append(remoteOnly, remoteFeature)
		}
	}

	var localOnly []transport.SecurityFeature
	for _, localFeature := range local.Security {
		found := false
		for _, agreedFeature := range agreed {
			if localFeature == agreedFeature {
				found = true
				break
			}
		}
		if !found {
			localOnly = append(localOnly, localFeature)
		}
	}

	result.Agreed.Security = agreed
	result.LocalOnly.Security = localOnly
	result.RemoteOnly.Security = remoteOnly

	return nil
}

// negotiateProtocolVersion negotiates protocol version
func (n *DefaultNegotiator) negotiateProtocolVersion(local, remote transport.Capabilities, result *NegotiationResult) error {
	if local.ProtocolVersion == remote.ProtocolVersion {
		result.Agreed.ProtocolVersion = local.ProtocolVersion
	} else {
		result.Conflicts = append(result.Conflicts, CapabilityConflict{
			Capability:  "protocol_version",
			LocalValue:  local.ProtocolVersion,
			RemoteValue: remote.ProtocolVersion,
			Reason:      "Protocol version mismatch",
		})
		result.FallbackRequired = true
		result.FallbackReason = "Protocol version incompatibility"
	}
	return nil
}

// negotiateFeatures negotiates custom features
func (n *DefaultNegotiator) negotiateFeatures(local, remote transport.Capabilities, result *NegotiationResult) error {
	result.Agreed.Features = make(map[string]interface{})
	result.LocalOnly.Features = make(map[string]interface{})
	result.RemoteOnly.Features = make(map[string]interface{})

	// Find common features
	for key, localValue := range local.Features {
		if remoteValue, exists := remote.Features[key]; exists {
			// Try to negotiate the feature using rules
			negotiated := false
			for _, rule := range n.negotiationRules {
				if rule.CanHandle(key) {
					if agreedValue, err := rule.Negotiate(context.Background(), key, localValue, remoteValue); err == nil {
						result.Agreed.Features[key] = agreedValue
						negotiated = true
						break
					}
				}
			}

			if !negotiated {
				// If no rule can handle it, compare values
				if fmt.Sprintf("%v", localValue) == fmt.Sprintf("%v", remoteValue) {
					result.Agreed.Features[key] = localValue
				} else {
					result.Conflicts = append(result.Conflicts, CapabilityConflict{
						Capability:  key,
						LocalValue:  localValue,
						RemoteValue: remoteValue,
						Reason:      "Feature value mismatch",
					})
				}
			}
		} else {
			result.LocalOnly.Features[key] = localValue
		}
	}

	// Find remote-only features
	for key, remoteValue := range remote.Features {
		if _, exists := local.Features[key]; !exists {
			result.RemoteOnly.Features[key] = remoteValue
		}
	}

	return nil
}

// BooleanCapabilityRule handles boolean capability negotiation
type BooleanCapabilityRule struct{}

func (r *BooleanCapabilityRule) CanHandle(capability string) bool {
	return capability == "streaming" || capability == "bidirectional" || 
		   capability == "multiplexing" || capability == "reconnection"
}

func (r *BooleanCapabilityRule) Negotiate(ctx context.Context, capability string, localValue, remoteValue interface{}) (interface{}, error) {
	localBool, localOk := localValue.(bool)
	remoteBool, remoteOk := remoteValue.(bool)

	if !localOk || !remoteOk {
		return nil, fmt.Errorf("invalid boolean values for capability %s", capability)
	}

	// Both must support the capability for it to be agreed upon
	return localBool && remoteBool, nil
}

// CompressionRule handles compression capability negotiation
type CompressionRule struct{}

func (r *CompressionRule) CanHandle(capability string) bool {
	return capability == "compression"
}

func (r *CompressionRule) Negotiate(ctx context.Context, capability string, localValue, remoteValue interface{}) (interface{}, error) {
	localTypes, localOk := localValue.([]transport.CompressionType)
	remoteTypes, remoteOk := remoteValue.([]transport.CompressionType)

	if !localOk || !remoteOk {
		return nil, fmt.Errorf("invalid compression types for capability %s", capability)
	}

	// Find intersection of compression types
	localSet := make(map[transport.CompressionType]bool)
	for _, t := range localTypes {
		localSet[t] = true
	}

	var agreed []transport.CompressionType
	for _, t := range remoteTypes {
		if localSet[t] {
			agreed = append(agreed, t)
		}
	}

	return agreed, nil
}

// SecurityRule handles security feature negotiation
type SecurityRule struct{}

func (r *SecurityRule) CanHandle(capability string) bool {
	return capability == "security"
}

func (r *SecurityRule) Negotiate(ctx context.Context, capability string, localValue, remoteValue interface{}) (interface{}, error) {
	localFeatures, localOk := localValue.([]transport.SecurityFeature)
	remoteFeatures, remoteOk := remoteValue.([]transport.SecurityFeature)

	if !localOk || !remoteOk {
		return nil, fmt.Errorf("invalid security features for capability %s", capability)
	}

	// Find intersection of security features
	localSet := make(map[transport.SecurityFeature]bool)
	for _, f := range localFeatures {
		localSet[f] = true
	}

	var agreed []transport.SecurityFeature
	for _, f := range remoteFeatures {
		if localSet[f] {
			agreed = append(agreed, f)
		}
	}

	return agreed, nil
}

// MessageSizeRule handles message size limit negotiation
type MessageSizeRule struct{}

func (r *MessageSizeRule) CanHandle(capability string) bool {
	return capability == "max_message_size"
}

func (r *MessageSizeRule) Negotiate(ctx context.Context, capability string, localValue, remoteValue interface{}) (interface{}, error) {
	localSize, localOk := localValue.(int64)
	remoteSize, remoteOk := remoteValue.(int64)

	if !localOk || !remoteOk {
		return nil, fmt.Errorf("invalid message size values for capability %s", capability)
	}

	// Use the smaller of the two limits
	if localSize < remoteSize {
		return localSize, nil
	}
	return remoteSize, nil
}

// ProtocolVersionRule handles protocol version negotiation
type ProtocolVersionRule struct{}

func (r *ProtocolVersionRule) CanHandle(capability string) bool {
	return capability == "protocol_version"
}

func (r *ProtocolVersionRule) Negotiate(ctx context.Context, capability string, localValue, remoteValue interface{}) (interface{}, error) {
	localVersion, localOk := localValue.(string)
	remoteVersion, remoteOk := remoteValue.(string)

	if !localOk || !remoteOk {
		return nil, fmt.Errorf("invalid protocol version values for capability %s", capability)
	}

	// For now, versions must match exactly
	if localVersion == remoteVersion {
		return localVersion, nil
	}

	return nil, fmt.Errorf("protocol version mismatch: local=%s, remote=%s", localVersion, remoteVersion)
}