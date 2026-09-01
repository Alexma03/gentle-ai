package pi

import (
	"slices"
	"strings"
	"testing"
)

const canonicalPiSubagentsRPCReady = `{
  "version": 1,
  "methods": ["ping", "status", "manage", "spawn", "steer", "interrupt", "stop", "resume"],
  "capabilities": {
    "status": true,
    "asyncSpawn": true,
    "steer": true,
    "interrupt": true,
    "stop": true,
    "resume": true
  },
  "events": {
    "ready": "subagents:rpc:v1:ready",
    "request": "subagents:rpc:v1:request",
    "replyPrefix": "subagents:rpc:v1:reply:",
    "asyncComplete": "subagent:async-complete"
  }
}`

func TestAdapterAcceptsCanonicalPiSubagentsProvider(t *testing.T) {
	capabilities, err := NewAdapter().AcceptSubagentsRPCResponse(PiSubagentsRPCProviderResponse{
		Package: piSubagentsPackageSpec,
		Ready:   []byte(canonicalPiSubagentsRPCReady),
	})
	if err != nil {
		t.Fatalf("AcceptSubagentsRPCResponse() error = %v", err)
	}
	if capabilities.Version != PiSubagentsRPCVersion || !slices.Contains(capabilities.Methods, "resume") {
		t.Fatalf("capabilities = %#v, want canonical Nicobailon v1", capabilities)
	}
}

func TestAdapterRejectsAbsentMalformedWrongVersionIncompleteAndRetiredPiSubagents(t *testing.T) {
	tests := []struct {
		name     string
		response PiSubagentsRPCProviderResponse
		want     string
	}{
		{name: "absent", response: PiSubagentsRPCProviderResponse{Package: piSubagentsPackageSpec}, want: "ready payload"},
		{name: "malformed", response: PiSubagentsRPCProviderResponse{Package: piSubagentsPackageSpec, Ready: []byte("{")}, want: "invalid JSON"},
		{name: "wrong version", response: PiSubagentsRPCProviderResponse{Package: piSubagentsPackageSpec, Ready: []byte(strings.Replace(canonicalPiSubagentsRPCReady, `"version": 1`, `"version": 2`, 1))}, want: "version"},
		{name: "incomplete", response: PiSubagentsRPCProviderResponse{Package: piSubagentsPackageSpec, Ready: []byte(strings.Replace(canonicalPiSubagentsRPCReady, `"resume"`, `"not-resume"`, 1))}, want: "resume"},
		{name: "retired provider", response: PiSubagentsRPCProviderResponse{Package: "npm:pi-subagents-j0k3r", Ready: []byte(canonicalPiSubagentsRPCReady)}, want: "canonical"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewAdapter().AcceptSubagentsRPCResponse(tt.response); err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.want)) {
				t.Fatalf("AcceptSubagentsRPCResponse() error = %v, want substring %q", err, tt.want)
			}
		})
	}
}
