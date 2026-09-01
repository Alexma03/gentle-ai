package pi

import (
	"slices"
	"strings"
	"testing"
)

func TestPiSubagentsRPCReadyAcceptsVersionOneCapabilities(t *testing.T) {
	ready := `{
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

	capabilities, err := ValidatePiSubagentsRPCReady([]byte(ready))
	if err != nil {
		t.Fatalf("ValidatePiSubagentsRPCReady() error = %v", err)
	}
	if capabilities.Version != PiSubagentsRPCVersion {
		t.Fatalf("capabilities.Version = %d, want %d", capabilities.Version, PiSubagentsRPCVersion)
	}
	if !slices.Contains(capabilities.Methods, "spawn") {
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
			json: `{"methods":["ping","status","manage","spawn","steer","interrupt","stop","resume"],"capabilities":{"status":true,"asyncSpawn":true,"steer":true,"interrupt":true,"stop":true,"resume":true},"events":{"ready":"subagents:rpc:v1:ready","request":"subagents:rpc:v1:request","replyPrefix":"subagents:rpc:v1:reply:","asyncComplete":"subagent:async-complete"}}`,
			want: "version",
		},
		{
			name: "unknown version",
			json: `{"version":2,"methods":["ping","status","manage","spawn","steer","interrupt","stop","resume"],"capabilities":{"status":true,"asyncSpawn":true,"steer":true,"interrupt":true,"stop":true,"resume":true},"events":{"ready":"subagents:rpc:v1:ready","request":"subagents:rpc:v1:request","replyPrefix":"subagents:rpc:v1:reply:","asyncComplete":"subagent:async-complete"}}`,
			want: "version",
		},
		{
			name: "missing required method",
			json: `{"version":1,"methods":["ping","status","manage","steer","interrupt","stop","resume"],"capabilities":{"status":true,"asyncSpawn":true,"steer":true,"interrupt":true,"stop":true,"resume":true},"events":{"ready":"subagents:rpc:v1:ready","request":"subagents:rpc:v1:request","replyPrefix":"subagents:rpc:v1:reply:","asyncComplete":"subagent:async-complete"}}`,
			want: "spawn",
		},
		{
			name: "legacy event",
			json: `{"event":"pi-subagents:ready","version":1,"methods":["ping","status","manage","spawn","steer","interrupt","stop","resume"],"capabilities":{"status":true,"asyncSpawn":true,"steer":true,"interrupt":true,"stop":true,"resume":true},"events":{"ready":"subagents:rpc:v1:ready","request":"subagents:rpc:v1:request","replyPrefix":"subagents:rpc:v1:reply:","asyncComplete":"subagent:async-complete"}}`,
			want: "event",
		},
		{
			name: "trailing JSON",
			json: `{"version":1,"methods":["ping","status","manage","spawn","steer","interrupt","stop","resume"],"capabilities":{"status":true,"asyncSpawn":true,"steer":true,"interrupt":true,"stop":true,"resume":true},"events":{"ready":"subagents:rpc:v1:ready","request":"subagents:rpc:v1:request","replyPrefix":"subagents:rpc:v1:reply:","asyncComplete":"subagent:async-complete"}} {}`,
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
