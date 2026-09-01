package pi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
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
	"manage",
	"spawn",
	"steer",
	"interrupt",
	"stop",
	"resume",
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

// Supports reports whether the validated provider advertises a method.
func (c PiSubagentsRPCCapabilities) Supports(method string) bool {
	for _, advertised := range c.Methods {
		if advertised == method {
			return true
		}
	}
	return false
}

// PiSubagentsRPCProviderResponse is the opaque readiness response returned by
// a Pi host runtime. Package identity is checked before the payload is
// admitted so a retired provider cannot satisfy the same event shape.
type PiSubagentsRPCProviderResponse struct {
	Package string
	Ready   []byte
}

// ValidatePiSubagentsRPCReady validates Nicobailon's v1 ping/readiness
// projection. The event name is normally supplied by Pi's event bus rather
// than repeated in the payload; when present, it is checked against the v1
// event name. Missing, malformed, or unsupported versions fail closed.
func ValidatePiSubagentsRPCReady(payload []byte) (PiSubagentsRPCCapabilities, error) {
	var raw struct {
		Event        string          `json:"event"`
		Version      json.RawMessage `json:"version"`
		Methods      json.RawMessage `json:"methods"`
		Capabilities json.RawMessage `json:"capabilities"`
		Events       json.RawMessage `json:"events"`
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
	if len(raw.Methods) == 0 || bytes.Equal(bytes.TrimSpace(raw.Methods), []byte("null")) {
		return PiSubagentsRPCCapabilities{}, fmt.Errorf("pi-subagents RPC ready payload is missing methods")
	}
	var methods []string
	if err := json.Unmarshal(raw.Methods, &methods); err != nil {
		return PiSubagentsRPCCapabilities{}, fmt.Errorf("pi-subagents RPC methods are invalid: %w", err)
	}
	if len(methods) == 0 {
		return PiSubagentsRPCCapabilities{}, fmt.Errorf("pi-subagents RPC capabilities are missing methods")
	}
	seen := make(map[string]struct{}, len(methods))
	for _, method := range methods {
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
	if len(raw.Capabilities) == 0 || bytes.Equal(bytes.TrimSpace(raw.Capabilities), []byte("null")) {
		return PiSubagentsRPCCapabilities{}, fmt.Errorf("pi-subagents RPC ready payload is missing capabilities")
	}
	var capabilities map[string]json.RawMessage
	if err := json.Unmarshal(raw.Capabilities, &capabilities); err != nil {
		return PiSubagentsRPCCapabilities{}, fmt.Errorf("pi-subagents RPC capabilities are invalid: %w", err)
	}
	for _, name := range []string{"status", "asyncSpawn", "steer", "interrupt", "stop", "resume"} {
		value, ok := capabilities[name]
		if !ok {
			return PiSubagentsRPCCapabilities{}, fmt.Errorf("pi-subagents RPC capabilities are missing %q", name)
		}
		var enabled bool
		if err := json.Unmarshal(value, &enabled); err != nil || !enabled {
			return PiSubagentsRPCCapabilities{}, fmt.Errorf("pi-subagents RPC capability %q must be true", name)
		}
	}
	if len(raw.Events) == 0 || bytes.Equal(bytes.TrimSpace(raw.Events), []byte("null")) {
		return PiSubagentsRPCCapabilities{}, fmt.Errorf("pi-subagents RPC ready payload is missing events")
	}
	var events struct {
		Ready         string `json:"ready"`
		Request       string `json:"request"`
		ReplyPrefix   string `json:"replyPrefix"`
		AsyncComplete string `json:"asyncComplete"`
	}
	if err := json.Unmarshal(raw.Events, &events); err != nil {
		return PiSubagentsRPCCapabilities{}, fmt.Errorf("pi-subagents RPC events are invalid: %w", err)
	}
	if events.Ready != PiSubagentsRPCReadyEvent || events.Request != PiSubagentsRPCRequestEvent || events.ReplyPrefix != PiSubagentsRPCReplyEventPrefix {
		return PiSubagentsRPCCapabilities{}, fmt.Errorf("pi-subagents RPC events do not match the v1 event bus")
	}
	if events.AsyncComplete != "subagent:async-complete" {
		return PiSubagentsRPCCapabilities{}, fmt.Errorf("pi-subagents RPC capabilities must advertise events.asyncComplete")
	}

	return PiSubagentsRPCCapabilities{
		Version:       version,
		Event:         raw.Event,
		Methods:       append([]string(nil), methods...),
		AsyncComplete: true,
	}, nil
}

// AcceptSubagentsRPCResponse is the production/runtime acceptance boundary
// for Pi's subagents provider. It deliberately validates the package identity
// before handing the readiness bytes to the protocol validator and has no
// fallback provider or legacy protocol path.
func (a *Adapter) AcceptSubagentsRPCResponse(response PiSubagentsRPCProviderResponse) (PiSubagentsRPCCapabilities, error) {
	if strings.TrimSpace(response.Package) == "" {
		return PiSubagentsRPCCapabilities{}, fmt.Errorf("pi-subagents RPC provider response is missing package identity")
	}
	identity := piPackageIdentity(response.Package)
	if identity != piSubagentsPackageSpec {
		if isRetiredPiSubagentPackage(identity) {
			return PiSubagentsRPCCapabilities{}, fmt.Errorf("retired Pi subagents provider %q rejected; canonical %q is required", response.Package, piSubagentsPackageSpec)
		}
		return PiSubagentsRPCCapabilities{}, fmt.Errorf("Pi subagents provider %q rejected; canonical %q is required", response.Package, piSubagentsPackageSpec)
	}
	if len(bytes.TrimSpace(response.Ready)) == 0 {
		return PiSubagentsRPCCapabilities{}, fmt.Errorf("pi-subagents RPC ready payload is absent")
	}
	return ValidatePiSubagentsRPCReady(response.Ready)
}
