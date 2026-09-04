package pi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	piSubagentsRPCProbeMarker  = "gentle-ai.pi-subagents-rpc-ready"
	piSubagentsRPCProbeTimeout = 10 * time.Second
	piSubagentsRPCProbeMaxLine = 1 << 20
)

// piRPCCommandContext is a test seam for the outer Pi RPC process. The
// process itself remains Pi's documented --mode rpc protocol; the extension
// below uses only Pi's documented in-process event bus and RPC UI transport.
var piRPCCommandContext = exec.CommandContext

// piSubagentsRPCProbeExtension is loaded explicitly into a real Pi process.
// It asks the in-process Nicobailon bridge for its canonical ping projection,
// then carries that projection over Pi's documented RPC extension-ui notify
// event. No ad-hoc stdout protocol or alternate provider is introduced.
const piSubagentsRPCProbeExtension = `import type { ExtensionAPI, ExtensionContext } from "@earendil-works/pi-coding-agent";

const requestEvent = "subagents:rpc:v1:request";
const replyEvent = "subagents:rpc:v1:reply:gentle-ai-pi-subagents-rpc-probe";
const requestId = "gentle-ai-pi-subagents-rpc-probe";
const readyMarker = "gentle-ai.pi-subagents-rpc-ready";

export default function (pi: ExtensionAPI) {
  let currentContext: ExtensionContext | undefined;

  pi.events.on(replyEvent, (raw) => {
    const reply = raw as {
      requestId?: unknown;
      success?: unknown;
      data?: unknown;
    };
    if (reply.requestId !== requestId || reply.success !== true || !currentContext) return;
    currentContext.ui.notify(JSON.stringify({ marker: readyMarker, ready: reply.data }), "info");
  });

  pi.on("session_start", async (_event, context) => {
    currentContext = context;
    setTimeout(() => {
      pi.events.emit(requestEvent, { version: 1, requestId, method: "ping" });
    }, 0);
  });
}
`

// ProbeSubagentsRPC starts Pi through its documented outer RPC mode and asks
// the installed in-process provider for its canonical v1 ping/readiness
// projection. The response is returned even when process startup or exit
// fails, so the caller can run the same package-and-payload acceptance
// boundary and fail closed for an absent response.
func (a *Adapter) ProbeSubagentsRPC(ctx context.Context, homeDir, workspaceDir string) (PiSubagentsRPCProviderResponse, error) {
	response, err := a.installedSubagentsRPCProvider(homeDir)
	if err != nil {
		return response, err
	}

	probePath, err := writePiSubagentsRPCProbeExtension()
	if err != nil {
		return response, err
	}
	defer os.Remove(probePath)

	commandPath, err := a.lookPath("pi")
	if err != nil {
		return response, fmt.Errorf("resolve Pi runtime for subagents RPC probe: %w", err)
	}

	if ctx == nil {
		ctx = context.Background()
	}
	probeContext, cancel := context.WithTimeout(ctx, piSubagentsRPCProbeTimeout)
	defer cancel()

	command := piRPCCommandContext(
		probeContext,
		commandPath,
		"--mode", "rpc",
		"--no-session",
		"--offline",
		"--no-tools",
		"--no-context-files",
		"--extension", probePath,
	)
	if workspaceDir != "" {
		command.Dir = workspaceDir
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return response, fmt.Errorf("open Pi subagents RPC probe stdout: %w", err)
	}
	stdin, err := command.StdinPipe()
	if err != nil {
		return response, fmt.Errorf("open Pi subagents RPC probe stdin: %w", err)
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		return response, fmt.Errorf("start Pi subagents RPC probe: %w", err)
	}

	var ready bool
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 4096), piSubagentsRPCProbeMaxLine)
	for scanner.Scan() {
		payload, ok := parsePiSubagentsRPCReadyNotification(scanner.Bytes())
		if !ok {
			continue
		}
		response.Ready = payload
		ready = true
		break
	}
	scanErr := scanner.Err()
	// Pi's RPC process exits after its stdin reaches EOF. Closing stdin after
	// the first bound notification keeps the probe finite without sending an
	// invented command into Pi's protocol.
	closeErr := stdin.Close()
	waitErr := command.Wait()
	if scanErr != nil {
		return response, fmt.Errorf("read Pi subagents RPC probe output: %w", scanErr)
	}
	if closeErr != nil && waitErr == nil {
		return response, fmt.Errorf("close Pi subagents RPC probe stdin: %w", closeErr)
	}
	if waitErr != nil {
		return response, piSubagentsRPCProcessError(waitErr, stderr.String())
	}
	if !ready {
		// A clean process with no bound notification is an absent readiness
		// response; the production caller sends it through AcceptSubagentsRPCResponse.
		return response, nil
	}
	return response, nil
}

func (a *Adapter) installedSubagentsRPCProvider(homeDir string) (PiSubagentsRPCProviderResponse, error) {
	settingsPath := filepath.Join(ConfiguredAgentDir(homeDir), piSettingsFile)
	settings, err := readPiJSONObject(settingsPath)
	if err != nil {
		return PiSubagentsRPCProviderResponse{}, err
	}

	var firstIdentity string
	canonical := false
	for _, pkg := range piPackagesAsSlice(settings["packages"]) {
		identity := piPackageIdentity(pkg)
		if identity == "" {
			continue
		}
		if firstIdentity == "" {
			firstIdentity = identity
		}
		if isRetiredPiSubagentPackage(identity) {
			return PiSubagentsRPCProviderResponse{Package: identity}, nil
		}
		if identity == piSubagentsPackage {
			canonical = true
		}
	}
	if canonical {
		return PiSubagentsRPCProviderResponse{Package: piSubagentsPackage}, nil
	}
	return PiSubagentsRPCProviderResponse{Package: firstIdentity}, nil
}

func writePiSubagentsRPCProbeExtension() (string, error) {
	file, err := os.CreateTemp("", "gentle-ai-pi-subagents-rpc-probe-*.ts")
	if err != nil {
		return "", fmt.Errorf("create Pi subagents RPC probe extension: %w", err)
	}
	path := file.Name()
	cleanup := func(cause error) (string, error) {
		_ = file.Close()
		_ = os.Remove(path)
		return "", cause
	}
	if err := file.Chmod(0o600); err != nil {
		return cleanup(fmt.Errorf("restrict Pi subagents RPC probe extension: %w", err))
	}
	if _, err := io.WriteString(file, piSubagentsRPCProbeExtension); err != nil {
		return cleanup(fmt.Errorf("write Pi subagents RPC probe extension: %w", err))
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("close Pi subagents RPC probe extension: %w", err)
	}
	return path, nil
}

func parsePiSubagentsRPCReadyNotification(line []byte) ([]byte, bool) {
	var notification struct {
		Type    string `json:"type"`
		Method  string `json:"method"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(line, &notification); err != nil {
		return nil, false
	}
	if notification.Type != "extension_ui_request" || notification.Method != "notify" || notification.Message == "" {
		return nil, false
	}
	var envelope struct {
		Marker string          `json:"marker"`
		Ready  json.RawMessage `json:"ready"`
	}
	if err := json.Unmarshal([]byte(notification.Message), &envelope); err != nil || envelope.Marker != piSubagentsRPCProbeMarker {
		return nil, false
	}
	if len(envelope.Ready) == 0 || bytes.Equal(bytes.TrimSpace(envelope.Ready), []byte("null")) {
		return nil, false
	}
	return append([]byte(nil), envelope.Ready...), true
}

func piSubagentsRPCProcessError(processErr error, stderr string) error {
	detail := strings.TrimSpace(stderr)
	if len(detail) > 2048 {
		detail = detail[:2048] + "..."
	}
	if detail == "" {
		return fmt.Errorf("Pi subagents RPC probe failed: %w", processErr)
	}
	return fmt.Errorf("Pi subagents RPC probe failed: %w: %s", processErr, detail)
}
