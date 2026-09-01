package pi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// PiSubagentsRPCVersion is the in-process RPC contract version exported by
// Nicobailon's pi-subagents package.
const PiSubagentsRPCVersion = 1

const (
	// PiSubagentsRPCReadyEvent is emitted when the package can accept requests.
	PiSubagentsRPCReadyEvent = "subagents:rpc:v1:ready"
	// PiSubagentsRPCRequestEvent is the request event consumed by the package.
	PiSubagentsRPCRequestEvent = "subagents:rpc:v1:request"
	// PiSubagentsRPCReplyEventPrefix prefixes request-specific reply events.
	PiSubagentsRPCReplyEventPrefix = "subagents:rpc:v1:reply:"
)

var requiredPiSubagentsRPCMethods = [...]string{
	"ping",
	"status",
	"spawn",
	"interrupt",
	"stop",
}

// PiSubagentsRPCCapabilities is the validated, versioned readiness
// advertisement used by Pi integrations. Methods are copied from the input
// so callers cannot mutate the parser's internal state.
type PiSubagentsRPCCapabilities struct {
	Version       int
	Event         string
	Methods       []string
	AsyncComplete bool
}

// Supports reports whether the validated package advertises method.
func (c PiSubagentsRPCCapabilities) Supports(method string) bool {
	for _, advertised := range c.Methods {
		if advertised == method {
			return true
		}
	}
	return false
}

// ValidatePiSubagentsRPCReady validates a ready-event payload. The event name
// is normally supplied by Pi's event bus rather than repeated in the payload;
// when present, it is checked against the v1 event name. Missing, malformed,
// or unsupported versions fail closed.
func ValidatePiSubagentsRPCReady(payload []byte) (PiSubagentsRPCCapabilities, error) {
	var raw struct {
		Event        string          `json:"event"`
		Version      json.RawMessage `json:"version"`
		Capabilities json.RawMessage `json:"capabilities"`
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	if err := decoder.Decode(&raw); err != nil {
		return PiSubagentsRPCCapabilities{}, fmt.Errorf("pi-subagents RPC ready payload is invalid JSON: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return PiSubagentsRPCCapabilities{}, fmt.Errorf("pi-subagents RPC ready payload contains trailing JSON")
		}
		return PiSubagentsRPCCapabilities{}, fmt.Errorf("pi-subagents RPC ready payload has trailing data: %w", err)
	}
	if raw.Event != "" && raw.Event != PiSubagentsRPCReadyEvent {
		return PiSubagentsRPCCapabilities{}, fmt.Errorf("pi-subagents RPC ready event %q is unsupported", raw.Event)
	}
	if len(raw.Version) == 0 || bytes.Equal(bytes.TrimSpace(raw.Version), []byte("null")) {
		return PiSubagentsRPCCapabilities{}, fmt.Errorf("pi-subagents RPC ready payload is missing version")
	}
	var version int
	if err := json.Unmarshal(raw.Version, &version); err != nil || version != PiSubagentsRPCVersion {
		return PiSubagentsRPCCapabilities{}, fmt.Errorf("pi-subagents RPC ready version must be %d", PiSubagentsRPCVersion)
	}
	if len(raw.Capabilities) == 0 || bytes.Equal(bytes.TrimSpace(raw.Capabilities), []byte("null")) {
		return PiSubagentsRPCCapabilities{}, fmt.Errorf("pi-subagents RPC ready payload is missing capabilities")
	}

	var capabilities struct {
		Methods []string `json:"methods"`
		Events  struct {
			AsyncComplete *bool `json:"asyncComplete"`
		} `json:"events"`
	}
	if err := json.Unmarshal(raw.Capabilities, &capabilities); err != nil {
		return PiSubagentsRPCCapabilities{}, fmt.Errorf("pi-subagents RPC capabilities are invalid: %w", err)
	}
	if len(capabilities.Methods) == 0 {
		return PiSubagentsRPCCapabilities{}, fmt.Errorf("pi-subagents RPC capabilities are missing methods")
	}
	seen := make(map[string]struct{}, len(capabilities.Methods))
	for _, method := range capabilities.Methods {
		if method == "" {
			return PiSubagentsRPCCapabilities{}, fmt.Errorf("pi-subagents RPC capabilities contain an empty method")
		}
		if _, duplicate := seen[method]; duplicate {
			return PiSubagentsRPCCapabilities{}, fmt.Errorf("pi-subagents RPC capabilities duplicate method %q", method)
		}
		seen[method] = struct{}{}
	}
	for _, required := range requiredPiSubagentsRPCMethods {
		if _, ok := seen[required]; !ok {
			return PiSubagentsRPCCapabilities{}, fmt.Errorf("pi-subagents RPC capabilities are missing required method %q", required)
		}
	}
	if capabilities.Events.AsyncComplete == nil || !*capabilities.Events.AsyncComplete {
		return PiSubagentsRPCCapabilities{}, fmt.Errorf("pi-subagents RPC capabilities must advertise events.asyncComplete")
	}

	return PiSubagentsRPCCapabilities{
		Version:       version,
		Event:         raw.Event,
		Methods:       append([]string(nil), capabilities.Methods...),
		AsyncComplete: true,
	}, nil
}

// ValidatePiSubagentsRPC is an alias for callers that only care about the
// package's versioned readiness contract rather than the event name.
func ValidatePiSubagentsRPC(payload []byte) (PiSubagentsRPCCapabilities, error) {
	return ValidatePiSubagentsRPCReady(payload)
}
