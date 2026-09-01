package pi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParsePiSubagentsRPCReadyNotification(t *testing.T) {
	message, err := json.Marshal(map[string]any{
		"marker": piSubagentsRPCProbeMarker,
		"ready":  json.RawMessage(canonicalPiSubagentsRPCReady),
	})
	if err != nil {
		t.Fatal(err)
	}
	line, err := json.Marshal(map[string]any{
		"type":    "extension_ui_request",
		"method":  "notify",
		"message": string(message),
	})
	if err != nil {
		t.Fatal(err)
	}

	got, ok := parsePiSubagentsRPCReadyNotification(line)
	if !ok {
		t.Fatal("parsePiSubagentsRPCReadyNotification() did not accept official notify event")
	}
	var wantBuffer bytes.Buffer
	if err := json.Compact(&wantBuffer, []byte(canonicalPiSubagentsRPCReady)); err != nil {
		t.Fatal(err)
	}
	if string(got) != wantBuffer.String() {
		t.Fatalf("ready payload = %s, want %s", got, wantBuffer.String())
	}

	for _, line := range [][]byte{
		[]byte("not json"),
		[]byte(`{"type":"response","method":"notify"}`),
		[]byte(`{"type":"extension_ui_request","method":"select","message":"{}"}`),
		[]byte(`{"type":"extension_ui_request","method":"notify","message":"{}"}`),
	} {
		if got, ok := parsePiSubagentsRPCReadyNotification(line); ok || got != nil {
			t.Fatalf("parsePiSubagentsRPCReadyNotification(%s) = (%s, %v), want no match", line, got, ok)
		}
	}
}

func TestInstalledSubagentsRPCProviderSelectsCanonicalAndRejectsRetired(t *testing.T) {
	agentDir := t.TempDir()
	t.Setenv(piCodingAgentDirEnv, agentDir)
	settingsPath := filepath.Join(agentDir, piSettingsFile)
	if err := os.WriteFile(settingsPath, []byte(`{"packages":["npm:pi-mcp-adapter","npm:pi-subagents@1.2.3"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	response, err := NewAdapter().installedSubagentsRPCProvider(t.TempDir())
	if err != nil {
		t.Fatalf("installedSubagentsRPCProvider() error = %v", err)
	}
	if response.Package != piSubagentsPackageSpec {
		t.Fatalf("package = %q, want %q", response.Package, piSubagentsPackageSpec)
	}

	if err := os.WriteFile(settingsPath, []byte(`{"packages":["npm:pi-subagents","npm:pi-subagents-j0k3r"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	response, err = NewAdapter().installedSubagentsRPCProvider(t.TempDir())
	if err != nil {
		t.Fatalf("installedSubagentsRPCProvider() retired error = %v", err)
	}
	if response.Package != "npm:pi-subagents-j0k3r" {
		t.Fatalf("retired package = %q, want retired identity", response.Package)
	}
}

func TestProbeSubagentsRPCCapturesOfficialRPCNotification(t *testing.T) {
	if os.Getenv("PI_SUBAGENTS_RPC_PROBE_HELPER") == "1" {
		message, err := json.Marshal(map[string]any{
			"marker": piSubagentsRPCProbeMarker,
			"ready":  json.RawMessage(canonicalPiSubagentsRPCReady),
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		line, err := json.Marshal(map[string]any{
			"type":    "extension_ui_request",
			"method":  "notify",
			"message": string(message),
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		fmt.Println(string(line))
		return
	}

	previousCommandContext := piRPCCommandContext
	piRPCCommandContext = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestProbeSubagentsRPCCapturesOfficialRPCNotification$", "--")
		command.Env = append(os.Environ(), "PI_SUBAGENTS_RPC_PROBE_HELPER=1")
		return command
	}
	t.Cleanup(func() { piRPCCommandContext = previousCommandContext })

	agentDir := t.TempDir()
	t.Setenv(piCodingAgentDirEnv, agentDir)
	settingsPath := filepath.Join(agentDir, piSettingsFile)
	if err := os.WriteFile(settingsPath, []byte(`{"packages":["npm:pi-subagents"]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	adapter := NewAdapter()
	adapter.lookPath = func(string) (string, error) { return os.Args[0], nil }
	response, err := adapter.ProbeSubagentsRPC(context.Background(), t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatalf("ProbeSubagentsRPC() error = %v", err)
	}
	var compactReadyBuffer bytes.Buffer
	if err := json.Compact(&compactReadyBuffer, []byte(canonicalPiSubagentsRPCReady)); err != nil {
		t.Fatal(err)
	}
	want := PiSubagentsRPCProviderResponse{Package: piSubagentsPackageSpec, Ready: compactReadyBuffer.Bytes()}
	if !reflect.DeepEqual(response, want) {
		t.Fatalf("ProbeSubagentsRPC() = %#v, want %#v", response, want)
	}
}
