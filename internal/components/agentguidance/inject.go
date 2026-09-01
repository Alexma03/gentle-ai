package agentguidance

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/gentleman-programming/gentle-ai/v2/internal/agents"
	"github.com/gentleman-programming/gentle-ai/v2/internal/components/filemerge"
	"github.com/gentleman-programming/gentle-ai/v2/internal/model"
)

// RoutingSectionID is the managed marker section that owns routing guidance.
// It is deliberately independent of the SDD sections so that installing or
// removing optional SDD assets can never add or drop routing guidance.
const RoutingSectionID = "agent-routing"

// ErrInvalidTarget rejects an unusable installation root before any write, so
// a caller that lost its resolved home directory fails loudly instead of
// writing guidance into an unexpected location.
var ErrInvalidTarget = errors.New("invalid routing guidance target directory")

// ErrUnreadableSettings fails closed when an adapter's settings document cannot
// be decoded. Routing guidance is worth less than a user's configuration, so a
// document we cannot understand is never rewritten from an empty base.
var ErrUnreadableSettings = errors.New("unreadable agent settings document")

// ErrUnloadableGuidance fails closed when guidance would be written to a place
// the agent does not actually load. Silently installing unread guidance is the
// exact failure this component exists to prevent.
var ErrUnloadableGuidance = errors.New("routing guidance target is not loaded by the agent")

// Result mirrors the shape returned by the SDD injector so both installers can
// be aggregated by the same caller.
type Result struct {
	Changed bool
	Files   []string
}

// InjectRouting installs the organic routing guidance for one supported agent
// under targetDir, which is the installation root the adapter resolves its
// configuration paths from.
//
// Delivery is strategy-aware: writing one markdown file for every adapter would
// land the guidance in a scope the agent never loads, or inside a template its
// own installer rewrites from an embedded asset on the next sync.
//
// Only the marked section is owned by Gentle AI: everything a user wrote around
// it is preserved verbatim, and a second identical injection is a no-op.
func InjectRouting(targetDir string, agent model.AgentID) (Result, error) {
	// Render before resolving the delivery so an unsupported agent is rejected
	// without having touched the filesystem.
	rendered, err := RenderRouting(agent)
	if err != nil {
		return Result{}, err
	}

	delivery, err := resolveRoutingDelivery(targetDir, agent)
	if err != nil {
		return Result{}, err
	}

	return injectPromptSection(delivery, rendered)
}

// RoutingPaths reports the exact filesystem paths InjectRouting would write for
// one supported agent under targetDir, without creating or touching anything.
//
// Install and sync must snapshot every file they are about to rewrite. Routing
// guidance is delivered outside the component loop, so a selection whose
// components do not happen to cover the same file would otherwise be rewritten
// without a backup and could never be rolled back. Both answers come from the
// same delivery resolution, so the backup contract cannot drift away from what
// the injector actually writes.
func RoutingPaths(targetDir string, agent model.AgentID) ([]string, error) {
	delivery, err := resolveRoutingDelivery(targetDir, agent)
	if err != nil {
		return nil, err
	}
	return delivery.paths, nil
}

// routingDeliveryKind names the three scopes an agent actually loads guidance
// from. Writing one markdown file for every adapter would land the guidance
// where the agent never reads it, or inside a template its own installer
// rewrites on the next sync.
type routingDeliveryKind int

const (
	deliveryPromptSection routingDeliveryKind = iota
)

// routingDelivery is the single resolution of how and where guidance reaches
// one agent. It is intentionally pure so the path reporter and the injector can
// share it without the reporter gaining write side effects.
type routingDelivery struct {
	kind    routingDeliveryKind
	adapter agents.Adapter
	paths   []string
}

// resolveRoutingDelivery selects the delivery strategy and its target paths.
//
// It fails closed on anything that would make the guidance unreachable, because
// an unreachable target is exactly the failure this component exists to
// prevent — and reporting a path the injector cannot write is the same defect
// seen from the backup side.
func resolveRoutingDelivery(targetDir string, agent model.AgentID) (routingDelivery, error) {
	if strings.TrimSpace(targetDir) == "" {
		return routingDelivery{}, fmt.Errorf("%w: %q", ErrInvalidTarget, targetDir)
	}

	adapter, err := agents.NewAdapter(agent)
	if err != nil {
		return routingDelivery{}, fmt.Errorf("resolve routing guidance adapter for %q: %w", agent, err)
	}

	promptPath := adapter.SystemPromptFile(targetDir)
	if strings.TrimSpace(promptPath) == "" {
		return routingDelivery{}, fmt.Errorf("%w: adapter %q exposes no system prompt file", ErrInvalidTarget, agent)
	}
	return routingDelivery{kind: deliveryPromptSection, adapter: adapter, paths: []string{promptPath}}, nil
}

// injectPromptSection is the default delivery: a managed marker section inside
// the adapter's own system prompt file.
func injectPromptSection(delivery routingDelivery, rendered string) (Result, error) {
	promptPath := delivery.paths[0]

	existing, err := readFileOrEmpty(promptPath)
	if err != nil {
		return Result{}, err
	}

	updated := filemerge.InjectMarkdownSection(existing, RoutingSectionID, rendered)

	writeResult, err := filemerge.WriteFileAtomic(promptPath, []byte(updated), 0o644)
	if err != nil {
		return Result{}, err
	}

	return Result{Changed: writeResult.Changed, Files: []string{promptPath}}, nil
}

// readFileOrEmpty treats a missing prompt file as empty content: the first
// install legitimately has nothing to merge into.
func readFileOrEmpty(path string) (string, error) {
	data, err := readBytesOrEmpty(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func readBytesOrEmpty(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read file %q: %w", path, err)
	}
	return data, nil
}
