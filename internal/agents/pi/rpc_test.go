package pi

import (
	"strings"
	"testing"
)

func TestPiSubagentsRPCReadyAcceptsVersionOneCapabilities(t *testing.T) {
	ready := `{
  "event": "subagents:rpc:v1:ready",
  "version": 1,
  "capabilities": {
    "methods": ["ping", "status", "spawn", "interrupt", "stop"],
    "events": {"asyncComplete": true}
  }
}`

	capabilities, err := ValidatePiSubagentsRPCReady([]byte(ready))
	if err != nil {
		t.Fatalf("ValidatePiSubagentsRPCReady() error = %v", err)
	}
	if capabilities.Version != PiSubagentsRPCVersion {
		t.Fatalf("capabilities.Version = %d, want %d", capabilities.Version, PiSubagentsRPCVersion)
	}
	if !capabilities.Supports("spawn") {
		t.Fatal("version-one capabilities did not retain spawn")
	}
}

func TestPiSubagentsRPCReadyRejectsUnversionedOrUnknownCapabilities(t *testing.T) {
	tests := []struct {
		name string
		json string
		want string
	}{
		{
			name: "missing version",
			json: `{"event":"subagents:rpc:v1:ready","capabilities":{"methods":["ping","status","spawn","interrupt","stop"],"events":{"asyncComplete":true}}}`,
			want: "version",
		},
		{
			name: "unknown version",
			json: `{"event":"subagents:rpc:v1:ready","version":2,"capabilities":{"methods":["ping","status","spawn","interrupt","stop"],"events":{"asyncComplete":true}}}`,
			want: "version",
		},
		{
			name: "missing required method",
			json: `{"event":"subagents:rpc:v1:ready","version":1,"capabilities":{"methods":["ping","status"],"events":{"asyncComplete":true}}}`,
			want: "spawn",
		},
		{
			name: "legacy event",
			json: `{"event":"pi-subagents:ready","version":1,"capabilities":{"methods":["ping","status","spawn","interrupt","stop"],"events":{"asyncComplete":true}}}`,
			want: "event",
		},
		{
			name: "trailing JSON",
			json: `{"event":"subagents:rpc:v1:ready","version":1,"capabilities":{"methods":["ping","status","spawn","interrupt","stop"],"events":{"asyncComplete":true}}} {}`,
			want: "trailing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ValidatePiSubagentsRPCReady([]byte(tt.json)); err == nil || !strings.Contains(strings.ToLower(err.Error()), tt.want) {
				t.Fatalf("ValidatePiSubagentsRPCReady() error = %v, want substring %q", err, tt.want)
			}
		})
	}
}
