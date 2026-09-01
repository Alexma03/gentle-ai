#!/usr/bin/env bash
# e2e_test.sh — End-to-end tests for gentle-ai installer
#
# Test tiers (controlled by environment variables):
#   (default)            Tier 1: binary existence + dry-run tests (fast, no side-effects)
#   RUN_FULL_E2E=1       Tier 2: full install tests (writes to filesystem)
#   RUN_BACKUP_TESTS=1   Tier 3: backup/restore tests
#
# Usage inside Docker:
#   ./e2e_test.sh                         # Tier 1 only
#   RUN_FULL_E2E=1 ./e2e_test.sh          # Tier 1 + 2
#   RUN_BACKUP_TESTS=1 ./e2e_test.sh      # Tier 1 + 3
#   RUN_FULL_E2E=1 RUN_BACKUP_TESTS=1 ./e2e_test.sh  # All tiers
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$SCRIPT_DIR/lib.sh"

# ---------------------------------------------------------------------------
# Resolve binary
# ---------------------------------------------------------------------------
BINARY="$(resolve_binary)"
if [ -z "$BINARY" ]; then
    echo "ERROR: gentle-ai binary not found. Build it first."
    exit 1
fi
log_info "Using binary: $BINARY"

# Side-effect E2E exercises install/injection behavior. Keep it deterministic by
# satisfying the installer's "engram already exists on PATH" branch unless a
# maintainer explicitly opts into the live GitHub release download path.
if [ "${RUN_FULL_E2E:-0}" = "1" ] || [ "${RUN_BACKUP_TESTS:-0}" = "1" ]; then
    setup_fake_engram_binary
fi

# ===========================================================================
# TIER 1 — Basic binary & dry-run tests (always run)
# ===========================================================================

# --- Category 1a: Binary basics ---

test_binary_exists() {
    log_test "Binary exists and is executable"

    if [ -x "$(command -v "$BINARY")" ] || [ -x "$BINARY" ]; then
        log_pass "Binary is executable"
    else
        log_fail "Binary not found or not executable"
    fi
}

test_binary_runs() {
    log_test "Binary runs without panic"

    if output=$($BINARY install --dry-run 2>&1); then
        log_pass "Binary exited cleanly with --dry-run"
    else
        if echo "$output" | grep -qi "panic"; then
            log_fail "Binary panicked: $output"
        else
            log_pass "Binary exited with non-zero (no panic)"
        fi
    fi
}

test_version_command() {
    log_test "Version command works"

    output=$($BINARY version 2>&1) || true

    if echo "$output" | grep -q "gentle-ai"; then
        log_pass "Version command returns binary name"
    else
        log_fail "Version command failed: $output"
    fi
}

# --- Category 1b: Dry-run output format ---

test_dry_run_output_format() {
    log_test "Dry-run output contains expected sections"

    output=$($BINARY install --dry-run 2>&1) || true

    assert_output_contains "$output" "dry-run" "Output contains 'dry-run' marker"
    assert_output_contains "$output" "Agents:" "Output contains 'Agents:' header"
    assert_output_contains "$output" "Persona:" "Output contains 'Persona:' header"
    assert_output_contains "$output" "Preset:" "Output contains 'Preset:' header"
    assert_output_contains "$output" "Components order:" "Output contains 'Components order:' header"
    assert_output_contains "$output" "Platform decision:" "Output contains 'Platform decision:' header"
}

test_dry_run_platform_detection() {
    log_test "Dry-run shows platform decision"

    output=$($BINARY install --dry-run 2>&1) || true

    assert_output_contains "$output" "Platform decision" "Platform decision present in dry-run"
}

test_dry_run_detects_linux() {
    log_test "Dry-run detects Linux OS"

    # This test is only meaningful inside the Docker container (Linux).
    # Skip gracefully on macOS/other to avoid killing the test run.
    if [[ "$(uname -s)" != "Linux" ]]; then
        log_skip "Not running on Linux — platform detection test skipped"
        return 0
    fi

    output=$($BINARY install --dry-run 2>&1) || true

    assert_output_contains "$output" "os=linux" "Platform detected as Linux"
}

# --- Category 1c: Agent flag ---

test_dry_run_agent_claude_code() {
    log_test "Dry-run with --agent claude-code"

    output=$($BINARY install --agent claude-code --dry-run 2>&1) || true

    assert_output_contains "$output" "claude-code" "Dry-run output shows claude-code agent"
}




# --- Category 1d: Preset flags ---

test_dry_run_preset_minimal() {
    log_test "Dry-run with --preset minimal"

    output=$($BINARY install --preset minimal --dry-run 2>&1) || true

    assert_output_contains "$output" "Preset: minimal" "Shows minimal preset"
}

test_dry_run_preset_ecosystem() {
    log_test "Dry-run with --preset ecosystem-only"

    output=$($BINARY install --preset ecosystem-only --dry-run 2>&1) || true

    assert_output_contains "$output" "Preset: ecosystem-only" "Shows ecosystem-only preset"
}

test_dry_run_preset_full() {
    log_test "Dry-run with --preset full-gentleman"

    output=$($BINARY install --preset full-gentleman --dry-run 2>&1) || true

    assert_output_contains "$output" "Preset: full-gentleman" "Shows full-gentleman preset"
}

test_dry_run_preset_custom() {
    log_test "Dry-run with --preset custom"

    output=$($BINARY install --preset custom --dry-run 2>&1) || true

    assert_output_contains "$output" "Preset: custom" "Shows custom preset"
}

# --- Category 1e: Preset component order validation ---

test_preset_minimal_components() {
    log_test "Preset minimal with persona=custom produces only engram component"

    # Use persona=custom to test the preset alone, since persona is now
    # driven by Selection.Persona (decoupled from preset).
    output=$($BINARY install --preset minimal --persona custom --agent claude-code --dry-run 2>&1) || true

    # The component list should contain engram
    assert_output_contains "$output" "engram" "Minimal preset includes engram"
    # Should NOT contain sdd, skills, persona, etc.
    assert_output_not_contains "$output" "Components order:.*sdd" "Minimal preset excludes sdd"
    assert_output_not_contains "$output" "Components order:.*persona" "Minimal + persona=custom excludes persona"
}

test_preset_minimal_with_default_persona_includes_persona() {
    log_test "Preset minimal with default persona (gentleman) includes persona"

    # Persona is now decoupled from preset — default Gentleman persona is
    # installed regardless of which preset the user picks.
    output=$($BINARY install --preset minimal --agent claude-code --dry-run 2>&1) || true

    local components_line
    components_line=$(echo "$output" | grep "Components order:")

    assert_output_contains "$components_line" "engram" "Minimal includes engram"
    assert_output_contains "$components_line" "persona" "Minimal + default Gentleman persona includes persona"
}


test_preset_full_with_custom_persona_excludes_persona() {
    log_test "Preset full-gentleman with persona=custom excludes persona"

    # Persona is decoupled — picking persona=custom skips persona install
    # even when the preset is full-gentleman.
    output=$($BINARY install --preset full-gentleman --persona custom --agent claude-code --dry-run 2>&1) || true

    local components_line
    components_line=$(echo "$output" | grep "Components order:")

    assert_output_contains "$components_line" "engram" "Full + persona=custom keeps engram"
    assert_output_contains "$components_line" "permissions" "Full + persona=custom keeps permissions"
    assert_output_not_contains "$components_line" "persona" "Full + persona=custom excludes persona"
}




test_preset_custom_no_components() {
    log_test "Preset custom with no --component produces empty component list"

    output=$($BINARY install --preset custom --agent claude-code --dry-run 2>&1) || true

    # Custom preset without explicit components = empty
    local components_line
    components_line=$(echo "$output" | grep "Components order:")
    assert_output_not_contains "$components_line" "engram" "Custom preset without components excludes engram"
    assert_output_not_contains "$components_line" "sdd" "Custom preset without components excludes sdd"
    assert_output_not_contains "$components_line" "skills" "Custom preset without components excludes skills"
}

test_preset_custom_explicit_components() {
    log_test "Preset custom with explicit --component flags"

    output=$($BINARY install --preset custom --agent claude-code --component engram --component sdd --component skills --dry-run 2>&1) || true

    local components_line
    components_line=$(echo "$output" | grep "Components order:")
    assert_output_contains "$components_line" "engram" "Custom + explicit components includes engram"
    assert_output_contains "$components_line" "sdd" "Custom + explicit components includes sdd"
    assert_output_contains "$components_line" "skills" "Custom + explicit components includes skills"
    assert_output_not_contains "$components_line" "persona" "Custom + explicit components excludes persona"
    assert_output_not_contains "$components_line" "context7" "Custom + explicit components excludes context7"
}

# --- Category 1f: Individual component flags ---

test_dry_run_component_engram() {
    log_test "Dry-run with --component engram"
    output=$($BINARY install --agent claude-code --component engram --dry-run 2>&1) || true
    assert_output_contains "$output" "engram" "Shows engram component"
}

test_dry_run_component_sdd() {
    log_test "Dry-run with --component sdd"
    output=$($BINARY install --agent claude-code --component sdd --dry-run 2>&1) || true
    assert_output_contains "$output" "sdd" "Shows sdd component"
}

test_dry_run_component_skills() {
    log_test "Dry-run with --component skills"
    output=$($BINARY install --agent claude-code --component skills --dry-run 2>&1) || true
    assert_output_contains "$output" "skills" "Shows skills component"
}

test_dry_run_component_context7() {
    log_test "Dry-run with --component context7"
    output=$($BINARY install --agent claude-code --component context7 --dry-run 2>&1) || true
    assert_output_contains "$output" "context7" "Shows context7 component"
}

test_dry_run_component_persona() {
    log_test "Dry-run with --component persona"
    output=$($BINARY install --agent claude-code --component persona --dry-run 2>&1) || true
    assert_output_contains "$output" "persona" "Shows persona component"
}




# --- Category 1f2: SDD mode flag ---




# --- Category 1g: Invalid input rejection ---

test_invalid_persona_rejected() {
    log_test "Invalid persona is rejected"

    if $BINARY install --persona nonexistent --dry-run 2>&1; then
        log_fail "Invalid persona should have been rejected"
    else
        log_pass "Invalid persona correctly rejected"
    fi
}

test_invalid_component_rejected() {
    log_test "Invalid component is rejected"

    if $BINARY install --component fakecomp --dry-run 2>&1; then
        log_fail "Invalid component should have been rejected"
    else
        log_pass "Invalid component correctly rejected"
    fi
}

test_invalid_preset_rejected() {
    log_test "Invalid preset is rejected"

    if $BINARY install --preset nonexistent --dry-run 2>&1; then
        log_fail "Invalid preset should have been rejected"
    else
        log_pass "Invalid preset correctly rejected"
    fi
}

test_unknown_command_rejected() {
    log_test "Unknown command is rejected"

    if $BINARY foobar 2>&1; then
        log_fail "Unknown command should have been rejected"
    else
        log_pass "Unknown command correctly rejected"
    fi
}

# ===========================================================================
# TIER 2 — Full install tests (require RUN_FULL_E2E=1)
# ===========================================================================

# --- Category 2: Claude Code component injection ---

test_cc_engram_injection() {
    log_test "Claude Code: engram injection (MCP + CLAUDE.md)"
    cleanup_test_env

    if $BINARY install --agent claude-code --component engram --persona neutral 2>&1; then
        # User-scope MCP registry
        local registry="$HOME/.claude.json"
        assert_file_exists "$registry" "Claude user MCP registry"
        assert_file_contains "$registry" '"mcpServers"' "Registry has mcpServers"
        assert_file_contains "$registry" '"engram"' "Registry has Engram server"
        assert_valid_json "$registry" "Claude user MCP registry is valid JSON"
        assert_file_not_exists "$HOME/.claude/mcp/engram.json" "legacy Engram MCP file is not written"

        # CLAUDE.md section
        assert_file_exists "$HOME/.claude/CLAUDE.md" "CLAUDE.md exists"
        assert_file_contains "$HOME/.claude/CLAUDE.md" "gentle-ai:engram-protocol" "CLAUDE.md has engram-protocol section marker"
        assert_file_contains "$HOME/.claude/CLAUDE.md" "mem_save" "CLAUDE.md has real Engram content (mem_save)"
        assert_file_size_min "$HOME/.claude/CLAUDE.md" 500 "CLAUDE.md has substantial content"
    else
        log_fail "engram install command failed"
    fi
}

test_cc_sdd_injection() {
    log_test "Claude Code: SDD injection (CLAUDE.md + native sub-agents)"
    cleanup_test_env

    if $BINARY install --agent claude-code --component sdd --persona neutral 2>&1; then
        assert_file_exists "$HOME/.claude/CLAUDE.md" "CLAUDE.md exists"
        assert_file_contains "$HOME/.claude/CLAUDE.md" "gentle-ai:sdd-orchestrator" "CLAUDE.md has SDD section marker"
        assert_file_contains "$HOME/.claude/CLAUDE.md" "sub-agent\|dependency\|orchestrator" "CLAUDE.md has real SDD content"
        assert_file_size_min "$HOME/.claude/CLAUDE.md" 500 "CLAUDE.md SDD section is substantial"

        for phase in sdd-init sdd-explore sdd-research sdd-propose sdd-spec sdd-design sdd-tasks sdd-apply sdd-verify sdd-archive sdd-onboard; do
            assert_file_exists "$HOME/.claude/agents/${phase}.md" "Claude native sub-agent exists: ${phase}"
            assert_file_size_min "$HOME/.claude/agents/${phase}.md" 200 "Claude native sub-agent is substantial: ${phase}"
        done

        assert_file_contains "$HOME/.claude/agents/sdd-design.md" "model: opus" "Claude design sub-agent uses balanced Opus assignment"
        assert_file_contains "$HOME/.claude/agents/sdd-spec.md" "model: sonnet" "Claude spec sub-agent uses balanced Sonnet assignment"
        assert_file_contains "$HOME/.claude/agents/sdd-archive.md" "model: haiku" "Claude archive sub-agent uses balanced Haiku assignment"

        assert_file_contains "$HOME/.claude/agents/sdd-explore.md" "tools:" "Claude explore sub-agent declares tool scope"
        assert_file_contains "$HOME/.claude/agents/sdd-explore.md" "WebFetch" "Claude explore sub-agent includes WebFetch"
        assert_file_contains "$HOME/.claude/agents/sdd-explore.md" "WebSearch" "Claude explore sub-agent includes WebSearch"
        assert_file_contains "$HOME/.claude/agents/sdd-explore.md" "mcp__plugin_engram_engram__mem_save" "Claude explore sub-agent includes Engram save"

        assert_file_contains "$HOME/.claude/agents/sdd-apply.md" "tools:" "Claude apply sub-agent declares tool scope"
        assert_file_contains "$HOME/.claude/agents/sdd-apply.md" "Read" "Claude apply sub-agent includes Read"
        assert_file_contains "$HOME/.claude/agents/sdd-apply.md" "Edit" "Claude apply sub-agent includes Edit"
        assert_file_contains "$HOME/.claude/agents/sdd-apply.md" "Write" "Claude apply sub-agent includes Write"
        assert_file_contains "$HOME/.claude/agents/sdd-apply.md" "Bash" "Claude apply sub-agent includes Bash"
        assert_file_contains "$HOME/.claude/agents/sdd-apply.md" "mcp__plugin_engram_engram__mem_search" "Claude apply sub-agent includes Engram search"
        assert_file_contains "$HOME/.claude/agents/sdd-apply.md" "mcp__plugin_engram_engram__mem_get_observation" "Claude apply sub-agent includes Engram read"
        assert_file_contains "$HOME/.claude/agents/sdd-apply.md" "mcp__plugin_engram_engram__mem_save" "Claude apply sub-agent includes Engram save"
        assert_file_contains "$HOME/.claude/agents/sdd-apply.md" "mcp__plugin_engram_engram__mem_update" "Claude apply sub-agent includes Engram update"

        assert_file_contains "$HOME/.claude/agents/sdd-verify.md" "tools:" "Claude verify sub-agent declares tool scope"
        assert_file_contains "$HOME/.claude/agents/sdd-verify.md" "Read" "Claude verify sub-agent includes Read"
        assert_file_contains "$HOME/.claude/agents/sdd-verify.md" "Bash" "Claude verify sub-agent includes Bash"
        assert_file_contains "$HOME/.claude/agents/sdd-verify.md" "mcp__plugin_engram_engram__mem_search" "Claude verify sub-agent includes Engram search"
        assert_file_contains "$HOME/.claude/agents/sdd-verify.md" "mcp__plugin_engram_engram__mem_get_observation" "Claude verify sub-agent includes Engram read"
        assert_file_contains "$HOME/.claude/agents/sdd-verify.md" "mcp__plugin_engram_engram__mem_save" "Claude verify sub-agent includes Engram save"
    else
        log_fail "SDD install command failed"
    fi
}

test_cc_persona_gentleman() {
    log_test "Claude Code: persona injection (gentleman)"
    cleanup_test_env

    if $BINARY install --agent claude-code --component persona --persona gentleman 2>&1; then
        assert_file_exists "$HOME/.claude/CLAUDE.md" "CLAUDE.md exists"
        assert_file_contains "$HOME/.claude/CLAUDE.md" "gentle-ai:persona" "CLAUDE.md has persona section marker"
        # Claude has an active output-style channel — the CLAUDE.md persona
        # section is now a residual (tooling directives + pointer only); tone
        # content lives exclusively in the output style (design.md Decision 1).
        assert_file_contains "$HOME/.claude/CLAUDE.md" "Persona Voice" "CLAUDE.md persona residual points to the output style"
        assert_file_not_contains "$HOME/.claude/CLAUDE.md" "Senior Architect" "CLAUDE.md persona residual carries no tone content"
        assert_file_size_min "$HOME/.claude/CLAUDE.md" 200 "Persona section is substantial"
        # Output-style file — canonical tone channel
        assert_file_exists "$HOME/.claude/output-styles/gentleman.md" "Output-style file exists"
        assert_file_contains "$HOME/.claude/output-styles/gentleman.md" "name: Gentleman" "Output-style has YAML frontmatter"
        assert_file_contains "$HOME/.claude/output-styles/gentleman.md" "keep-coding-instructions: true" "Output-style keeps coding instructions"
        assert_file_contains "$HOME/.claude/output-styles/gentleman.md" "Senior Architect" "Output-style carries the Gentleman tone content"
        # settings.json outputStyle key
        assert_file_exists "$HOME/.claude/settings.json" "settings.json exists"
        assert_file_contains "$HOME/.claude/settings.json" "outputStyle" "settings.json has outputStyle key"
        assert_file_contains "$HOME/.claude/settings.json" "Gentleman" "settings.json outputStyle is Gentleman"
    else
        log_fail "persona (gentleman) install command failed"
    fi
}

test_cc_persona_neutral() {
    log_test "Claude Code: persona injection (neutral)"
    cleanup_test_env

    if $BINARY install --agent claude-code --component persona --persona neutral 2>&1; then
        assert_file_exists "$HOME/.claude/CLAUDE.md" "CLAUDE.md exists"
        assert_file_contains "$HOME/.claude/CLAUDE.md" "gentle-ai:persona" "CLAUDE.md has persona section marker"
        # Claude has an active output-style channel — the CLAUDE.md persona
        # section is now a residual; the mentor identity lives in the output
        # style (design.md Decision 1).
        assert_file_contains "$HOME/.claude/CLAUDE.md" "Persona Voice" "CLAUDE.md persona residual points to the output style"
        assert_file_not_contains "$HOME/.claude/CLAUDE.md" "Senior Architect" "CLAUDE.md persona residual carries no tone content"
        assert_file_exists "$HOME/.claude/output-styles/neutral.md" "Neutral output-style file exists"
        assert_file_contains "$HOME/.claude/output-styles/neutral.md" "Push back when user asks for code without context or understanding" "Neutral output-style carries the mentor tone content"
        assert_file_not_contains "$HOME/.claude/output-styles/neutral.md" "Rioplatense\|voseo\|loco\|ponete las pilas" "Neutral output-style excludes regional language"
    else
        log_fail "persona (neutral) install command failed"
    fi
}

test_cc_persona_custom_does_nothing() {
    log_test "Claude Code: persona custom does nothing (user keeps own personality)"
    cleanup_test_env

    if $BINARY install --agent claude-code --component persona --persona custom 2>&1; then
        # CLAUDE.md may exist: routing guidance is scheduled per agent and
        # deliberately outside the component loop, so every configured agent
        # receives it. What `--persona custom` promises is that no personality
        # is imposed, so assert that instead of the file's absence.
        assert_file_not_contains "$HOME/.claude/CLAUDE.md" "Senior Architect\|Rioplatense\|voseo" "CLAUDE.md carries no tone content under custom"
        # No output-style file either.
        assert_file_not_exists "$HOME/.claude/output-styles/gentleman.md" "No output-style for custom"
    else
        log_fail "Custom persona install command failed"
    fi
}


test_cc_skills_minimal() {
    log_test "Claude Code: skills injection (minimal preset = SDD skills only)"
    cleanup_test_env

    if $BINARY install --agent claude-code --component skills --preset minimal --persona custom 2>&1; then
        local skills_dir="$HOME/.claude/skills"
        assert_dir_exists "$skills_dir" "Claude skills directory"

        # Minimal preset = 12 files: 11 SDD phases + judgment-day. _shared is support-only.
        assert_file_count "$skills_dir" "SKILL.md" 12 "Minimal preset: 12 skill files"

        # Verify specific SDD skills exist
        assert_file_exists "$skills_dir/sdd-init/SKILL.md" "sdd-init SKILL.md"
        assert_file_exists "$skills_dir/sdd-apply/SKILL.md" "sdd-apply SKILL.md"
        assert_file_exists "$skills_dir/sdd-verify/SKILL.md" "sdd-verify SKILL.md"
        assert_file_exists "$skills_dir/sdd-archive/SKILL.md" "sdd-archive SKILL.md"

        # Each skill should have substantial content
        assert_file_size_min "$skills_dir/sdd-init/SKILL.md" 100 "sdd-init SKILL.md has real content"

        # No framework skills in minimal
        if [ -f "$skills_dir/typescript/SKILL.md" ]; then
            log_fail "Minimal preset should NOT include typescript skill"
        else
            log_pass "Minimal preset correctly excludes framework skills"
        fi
    else
        log_fail "skills (minimal) install command failed"
    fi
}

test_cc_skills_full() {
    log_test "Claude Code: skills injection (full-gentleman = 13 foundation skills)"
    cleanup_test_env

    if $BINARY install --agent claude-code --component skills --preset full-gentleman --persona neutral 2>&1; then
        local skills_dir="$HOME/.claude/skills"
        assert_dir_exists "$skills_dir" "Claude skills directory"

        # Full preset = 25 files: 11 SDD phases + judgment-day + 13 foundation. _shared is support-only.
        assert_file_count "$skills_dir" "SKILL.md" 25 "Full preset: 25 skill files"

        # Verify foundation skills exist
        assert_file_exists "$skills_dir/go-testing/SKILL.md" "go-testing SKILL.md"
        assert_file_exists "$skills_dir/skill-creator/SKILL.md" "skill-creator SKILL.md"
        assert_file_exists "$skills_dir/branch-pr/SKILL.md" "branch-pr SKILL.md"
        assert_file_exists "$skills_dir/issue-creation/SKILL.md" "issue-creation SKILL.md"
        assert_file_exists "$skills_dir/skill-registry/SKILL.md" "skill-registry SKILL.md"

        # Real content check
        assert_file_size_min "$skills_dir/go-testing/SKILL.md" 200 "go-testing skill has real content"
        assert_file_size_min "$skills_dir/skill-creator/SKILL.md" 200 "skill-creator skill has real content"
        assert_file_size_min "$skills_dir/branch-pr/SKILL.md" 200 "branch-pr skill has real content"
        assert_file_size_min "$skills_dir/issue-creation/SKILL.md" 200 "issue-creation skill has real content"
        assert_file_size_min "$skills_dir/skill-registry/SKILL.md" 200 "skill-registry skill has real content"
    else
        log_fail "skills (full) install command failed"
    fi
}

test_cc_skills_ecosystem() {
    log_test "Claude Code: skills injection (ecosystem-only = 13 foundation skills)"
    cleanup_test_env

    if $BINARY install --agent claude-code --component skills --preset ecosystem-only --persona neutral 2>&1; then
        local skills_dir="$HOME/.claude/skills"
        assert_dir_exists "$skills_dir" "Claude skills directory"

        # ecosystem-only = 25 files: 11 SDD phases + judgment-day + 13 foundation. _shared is support-only.
        assert_file_count "$skills_dir" "SKILL.md" 25 "Ecosystem preset: 25 skill files"

        # SDD skills present
        assert_file_exists "$skills_dir/sdd-init/SKILL.md" "SDD skills present"
        # Foundation skills present
        assert_file_exists "$skills_dir/go-testing/SKILL.md" "Foundation skills present"
        assert_file_exists "$skills_dir/skill-creator/SKILL.md" "skill-creator present"
        assert_file_exists "$skills_dir/branch-pr/SKILL.md" "branch-pr present in ecosystem"
        assert_file_exists "$skills_dir/issue-creation/SKILL.md" "issue-creation present in ecosystem"
        # Stack-specific skills NOT present
        if [ -f "$skills_dir/react-19/SKILL.md" ]; then
            log_fail "Ecosystem preset should NOT include react-19"
        else
            log_pass "Ecosystem preset correctly excludes stack-specific skills"
        fi
    else
        log_fail "skills (ecosystem) install command failed"
    fi
}

test_cc_custom_skills_with_flag() {
    log_test "Claude Code: custom preset + explicit --skills flag installs specified skills"
    cleanup_test_env

    if $BINARY install --agent claude-code --preset custom --component skills --skills go-testing,branch-pr --persona neutral 2>&1; then
        local skills_dir="$HOME/.claude/skills"
        assert_dir_exists "$skills_dir" "Claude skills directory"

        # The explicitly requested skills must be present
        assert_file_exists "$skills_dir/go-testing/SKILL.md" "go-testing SKILL.md"
        assert_file_exists "$skills_dir/branch-pr/SKILL.md" "branch-pr SKILL.md"

        # Note: --component skills auto-resolves sdd (graph dep), which installs 12 SDD/orchestration skills.
        # Total = 12 SDD/orchestration skills + 2 explicit skills = 14 SKILL.md files.
        assert_file_count "$skills_dir" "SKILL.md" 14 "Custom + explicit skills: 12 SDD/orchestration + 2 explicit = 14 files"

        # SDD skills ARE present (from the sdd dependency)
        assert_file_exists "$skills_dir/sdd-init/SKILL.md" "sdd-init SKILL.md (from sdd dep)"
    else
        log_fail "custom + skills flag install command failed"
    fi
}

test_cc_custom_no_skills_flag_installs_nothing() {
    log_test "Claude Code: custom preset + skills component without --skills flag installs only SDD skills (from dep)"
    cleanup_test_env

    if $BINARY install --agent claude-code --preset custom --component skills --persona neutral 2>&1; then
        local skills_dir="$HOME/.claude/skills"
        # --component skills auto-resolves sdd as a hard dependency (graph: skills → sdd → engram).
        # The SDD component always installs its 12 SDD/orchestration skills.
        # The skills component itself is a no-op (SkillsForPreset(custom) returns nil, no --skills flag).
        # Result: exactly 12 SKILL.md files from the SDD dependency. _shared is support-only.
        assert_dir_exists "$skills_dir" "Skills directory created by sdd dependency"
        assert_file_count "$skills_dir" "SKILL.md" 12 "12 skill files from the SDD dependency"
        assert_file_exists "$skills_dir/sdd-init/SKILL.md" "sdd-init installed by sdd dependency"
    else
        log_fail "custom + skills component (no flag) install command failed"
    fi
}

test_cc_custom_sdd_plus_skills() {
    log_test "Claude Code: custom preset + SDD + skills with explicit --skills flag"
    cleanup_test_env

    if $BINARY install --agent claude-code --preset custom --component engram --component sdd --component skills --skills go-testing,branch-pr --persona neutral 2>&1; then
        local skills_dir="$HOME/.claude/skills"
        assert_dir_exists "$skills_dir" "Claude skills directory"

        # SDD component installs its own skills (sdd-init, sdd-explore, etc.)
        assert_file_exists "$skills_dir/sdd-init/SKILL.md" "sdd-init SKILL.md (from SDD component)"

        # Skills component installs only the explicitly requested ones
        assert_file_exists "$skills_dir/go-testing/SKILL.md" "go-testing SKILL.md (from --skills flag)"
        assert_file_exists "$skills_dir/branch-pr/SKILL.md" "branch-pr SKILL.md (from --skills flag)"

        # Total: 12 SDD/orchestration skills + 2 explicit skills = 14.
        assert_file_count "$skills_dir" "SKILL.md" 14 "SDD + explicit skills: 14 skill files total"
    else
        log_fail "custom + SDD + skills install command failed"
    fi
}

test_cc_context7_injection() {
    log_test "Claude Code: context7 injection (~/.claude.json user MCP registry)"
    cleanup_test_env

    if $BINARY install --agent claude-code --component context7 --persona neutral 2>&1; then
        # Claude Code only reads user-scope MCP servers from ~/.claude.json;
        # the settings.json mcpServers block earlier versions wrote is inert
        # and no longer written (issue #1868, PR #1909).
        local registry="$HOME/.claude.json"
        assert_file_exists "$registry" "Claude user MCP registry (~/.claude.json)"
        assert_file_contains "$registry" '"mcpServers"' "user registry has mcpServers key"
        assert_file_contains "$registry" '"context7"' "user registry has context7 server"
        assert_file_contains "$registry" 'context7-mcp' "user registry points to context7-mcp"
        assert_valid_json "$registry" "user registry is valid JSON"
        assert_file_not_exists "$HOME/.claude/mcp/context7.json" "legacy context7 MCP file is not written"
    else
        log_fail "context7 install command failed"
    fi
}

test_cc_permissions_injection() {
    log_test "Claude Code: permissions injection"
    cleanup_test_env

    if $BINARY install --agent claude-code --component permissions --persona neutral 2>&1; then
        local settings="$HOME/.claude/settings.json"
        assert_file_exists "$settings" "Claude settings.json"
        assert_file_contains "$settings" '"permissions"' "Has permissions key"
        assert_file_contains "$settings" '"deny"' "Has deny list"
        assert_valid_json "$settings" "settings.json is valid JSON"
    else
        log_fail "permissions install command failed"
    fi
}















# --- Category 4: Full preset integration ---




test_minimal_preset_claude_only_engram() {
    log_test "Minimal preset: Claude Code (only engram, nothing else)"
    cleanup_test_env

    if $BINARY install --agent claude-code --preset minimal --persona custom 2>&1; then
        # Engram should be installed (MCP + CLAUDE.md)
        assert_file_exists "$HOME/.claude/CLAUDE.md" "CLAUDE.md exists"
        assert_file_contains "$HOME/.claude/CLAUDE.md" "gentle-ai:engram-protocol" "Engram protocol section"

        # SDD should NOT be in CLAUDE.md
        assert_file_not_contains "$HOME/.claude/CLAUDE.md" "gentle-ai:sdd-orchestrator" "No SDD in minimal"
        # Persona should NOT be in CLAUDE.md
        assert_file_not_contains "$HOME/.claude/CLAUDE.md" "gentle-ai:persona" "No persona in minimal"
        # No permissions settings.json
        if [ -f "$HOME/.claude/settings.json" ]; then
            assert_file_not_contains "$HOME/.claude/settings.json" '"permissions"' "No permissions in minimal"
        else
            log_pass "No settings.json in minimal (correct)"
        fi
        # No skills directory (or empty)
        if [ -d "$HOME/.claude/skills" ]; then
            log_fail "Minimal preset should not create skills directory (skills component not in minimal preset)"
        else
            log_pass "No skills directory in minimal (correct)"
        fi
    else
        log_fail "Minimal preset (Claude Code) install command failed"
    fi
}



# --- Category 5: Content validation ---

test_content_claude_md_sections_substantial() {
    log_test "Content validation: CLAUDE.md sections are substantial"
    cleanup_test_env

    # Install SDD + persona + engram (all inject into CLAUDE.md)
    $BINARY install --agent claude-code --component sdd --component persona --persona gentleman 2>&1 || true
    $BINARY install --agent claude-code --component engram --persona gentleman 2>&1 || true

    local claude_md="$HOME/.claude/CLAUDE.md"
    if [ -f "$claude_md" ]; then
        assert_file_size_min "$claude_md" 1000 "CLAUDE.md with 3 sections >= 1000 bytes"
    else
        log_fail "CLAUDE.md not created"
    fi
}

test_content_skills_are_real() {
    log_test "Content validation: skill files contain real instructions"
    cleanup_test_env

    $BINARY install --agent claude-code --component skills --preset full-gentleman --persona neutral 2>&1 || true

    local skills_dir="$HOME/.claude/skills"
    if [ -d "$skills_dir" ]; then
        # Check every SKILL.md is at least 200 bytes (real content, not stubs)
        local all_ok=true
        while IFS= read -r skill_file; do
            local size
            size=$(wc -c < "$skill_file" | tr -d ' ')
            if [ "$size" -lt 200 ]; then
                log_fail "Skill file too small ($size bytes): $skill_file"
                all_ok=false
            fi
        done < <(find "$skills_dir" -name "SKILL.md" -type f)

        if $all_ok; then
            log_pass "All skill files have >= 200 bytes of real content"
        fi
    else
        log_fail "Skills directory not created"
    fi
}

test_content_mcp_json_valid() {
    log_test "Content validation: MCP JSON files are parseable"
    cleanup_test_env

    $BINARY install --agent claude-code --component context7 --persona neutral 2>&1 || true
    $BINARY install --agent claude-code --component engram --persona neutral 2>&1 || true

    # Claude Code reads user-scoped MCP servers from ~/.claude.json. The legacy
    # ~/.claude/mcp directory is intentionally no longer created.
    local registry="$HOME/.claude.json"
    assert_file_exists "$registry" "Claude user MCP registry"
    assert_valid_json "$registry" "Claude user MCP registry is valid JSON"
    assert_file_contains "$registry" '"context7"' "Registry has Context7 server"
    assert_file_contains "$registry" '"engram"' "Registry has Engram server"
    assert_file_not_exists "$HOME/.claude/mcp/context7.json" "legacy Context7 MCP file is not written"
    assert_file_not_exists "$HOME/.claude/mcp/engram.json" "legacy Engram MCP file is not written"
}


# --- Category 6: Idempotency ---


test_idempotent_sdd_claude() {
    log_test "Idempotency: SDD on Claude Code (no duplicate sections)"
    cleanup_test_env

    $BINARY install --agent claude-code --component sdd --persona neutral 2>&1 || true
    $BINARY install --agent claude-code --component sdd --persona neutral 2>&1 || true

    local claude_md="$HOME/.claude/CLAUDE.md"
    if [ -f "$claude_md" ]; then
        assert_no_duplicate_section "$claude_md" "sdd-orchestrator" "No duplicate SDD section after 2 runs"
    else
        log_fail "CLAUDE.md not found"
    fi
}

test_idempotent_persona_claude() {
    log_test "Idempotency: persona on Claude Code (no duplicate sections)"
    cleanup_test_env

    $BINARY install --agent claude-code --component persona --persona gentleman 2>&1 || true
    $BINARY install --agent claude-code --component persona --persona gentleman 2>&1 || true

    local claude_md="$HOME/.claude/CLAUDE.md"
    if [ -f "$claude_md" ]; then
        assert_no_duplicate_section "$claude_md" "persona" "No duplicate persona section after 2 runs"
    else
        log_fail "CLAUDE.md not found"
    fi
}

test_idempotent_engram_claude() {
    log_test "Idempotency: engram on Claude Code (no duplicate sections)"
    cleanup_test_env

    $BINARY install --agent claude-code --component engram --persona neutral 2>&1 || true
    $BINARY install --agent claude-code --component engram --persona neutral 2>&1 || true

    local claude_md="$HOME/.claude/CLAUDE.md"
    if [ -f "$claude_md" ]; then
        assert_no_duplicate_section "$claude_md" "engram-protocol" "No duplicate engram section after 2 runs"

        # Also check the user registry remains valid.
        local mcp_file="$HOME/.claude.json"
        if [ -f "$mcp_file" ]; then
            assert_valid_json "$mcp_file" "Claude user MCP registry still valid after 2 runs"
        fi
    else
        log_fail "CLAUDE.md not found"
    fi
}

# ─── Gemini parity tests ─────────────────────────────────────────────────────


# ─── Codex parity tests ───────────────────────────────────────────────────────

test_codex_engram_injection() {
    log_test "Codex: engram injection writes config.toml + instruction files"
    cleanup_test_env

    if $BINARY install --agent codex --component engram --persona neutral 2>&1; then
        local config_toml="$HOME/.codex/config.toml"
        local instructions="$HOME/.codex/engram-instructions.md"
        local compact="$HOME/.codex/engram-compact-prompt.md"

        assert_file_exists "$config_toml" "Codex config.toml"
        assert_file_contains "$config_toml" '[mcp_servers.engram]' "config.toml has [mcp_servers.engram]"
        assert_file_contains "$config_toml" 'command = ".*engram"' "config.toml has correct command"
        assert_file_contains "$config_toml" '"--tools=agent"' "config.toml has --tools=agent"
        assert_file_contains "$config_toml" 'model_instructions_file' "config.toml references instruction file"
        assert_file_contains "$config_toml" 'experimental_compact_prompt_file' "config.toml references compact prompt"

        assert_file_exists "$instructions" "engram-instructions.md"
        assert_file_contains "$instructions" 'mem_save' "Instructions have memory protocol content"

        assert_file_exists "$compact" "engram-compact-prompt.md"
        assert_file_contains "$compact" 'FIRST ACTION REQUIRED' "Compact prompt has required sentinel"
    else
        log_fail "Codex engram install command failed"
    fi
}

test_codex_engram_idempotent() {
    log_test "Codex: engram injection is idempotent (no duplicate blocks)"
    cleanup_test_env

    $BINARY install --agent codex --component engram --persona neutral 2>&1 || true
    $BINARY install --agent codex --component engram --persona neutral 2>&1 || true

    local config_toml="$HOME/.codex/config.toml"
    if [ -f "$config_toml" ]; then
        local count
        count=$(grep -c '\[mcp_servers\.engram\]' "$config_toml" || true)
        if [ "$count" -ne 1 ]; then
            log_fail "config.toml has $count [mcp_servers.engram] blocks after 2 runs (want exactly 1)"
        else
            log_pass "config.toml has exactly 1 [mcp_servers.engram] block after 2 runs"
        fi
    else
        log_fail "config.toml not found after 2 runs"
    fi
}

test_idempotent_skills_claude() {
    log_test "Idempotency: skills injection produces same files"
    cleanup_test_env

    $BINARY install --agent claude-code --component skills --preset minimal --persona custom 2>&1 || true
    # Capture file hashes
    local first_hashes
    first_hashes=$(find "$HOME/.claude/skills" -name "SKILL.md" -exec md5sum {} \; 2>/dev/null | sort)

    $BINARY install --agent claude-code --component skills --preset minimal --persona custom 2>&1 || true
    local second_hashes
    second_hashes=$(find "$HOME/.claude/skills" -name "SKILL.md" -exec md5sum {} \; 2>/dev/null | sort)

    if [ "$first_hashes" = "$second_hashes" ] && [ -n "$first_hashes" ]; then
        log_pass "Idempotent: same skill files after two runs"
    else
        log_fail "Skill files changed between runs"
    fi
}



# --- Category 8: Edge cases ---



test_edge_persona_switch() {
    log_test "Edge case: switching persona from gentleman to neutral"
    cleanup_test_env

    # First install with gentleman. Claude has an active output-style channel,
    # so the teacher identity ("Senior Architect") lives in the output style —
    # the CLAUDE.md persona section is a residual pointer (design.md Decision 1).
    $BINARY install --agent claude-code --component persona --persona gentleman 2>&1 || true
    assert_file_contains "$HOME/.claude/CLAUDE.md" "Persona Voice" "First install: gentleman persona residual points to the output style"
    assert_file_contains "$HOME/.claude/output-styles/gentleman.md" "Senior Architect" "First install: gentleman output-style has the teacher identity"

    # Then install with neutral — should REPLACE persona section AND the
    # selected output style. Neutral is a distinct professional voice (same
    # mentor identity, no regional language, no "Senior Architect" bio), so
    # the CLAUDE.md residual stays tone-free and the neutral output style is
    # checked for its own mentor-teaching tone content instead.
    $BINARY install --agent claude-code --component persona --persona neutral 2>&1 || true
    assert_file_contains "$HOME/.claude/CLAUDE.md" "Persona Voice" "Second install: neutral persona residual still points to the output style"
    assert_file_not_contains "$HOME/.claude/CLAUDE.md" "Senior Architect" "Second install: CLAUDE.md residual carries no tone content"
    assert_file_contains "$HOME/.claude/output-styles/neutral.md" "Push back when user asks for code without context or understanding" "Second install: neutral output-style has the mentor identity"
    assert_file_not_contains "$HOME/.claude/output-styles/neutral.md" "Rioplatense\|voseo\|ponete las pilas" "Second install: regional language removed from output style"
    assert_no_duplicate_section "$HOME/.claude/CLAUDE.md" "persona" "No duplicate persona after switch"
}








# --- Category 10: Cursor agent files ---

test_cursor_sdd_subagents() {
    log_test "Cursor: SDD install writes 11 agent files to ~/.cursor/agents/"
    cleanup_test_env

    # Cursor is a desktop app — create the config dir to signal it's "installed"
    mkdir -p "$HOME/.cursor"

    if $BINARY install --agent cursor --component sdd --persona neutral 2>&1; then
        local agents_dir="$HOME/.cursor/agents"

        # Directory must exist
        assert_dir_exists "$agents_dir" "~/.cursor/agents/ directory"

        # All 11 SDD agent files must exist
        assert_file_exists "$agents_dir/sdd-init.md" "sdd-init.md agent file"
        assert_file_exists "$agents_dir/sdd-explore.md" "sdd-explore.md agent file"
        assert_file_exists "$agents_dir/sdd-research.md" "sdd-research.md agent file"
        assert_file_exists "$agents_dir/sdd-propose.md" "sdd-propose.md agent file"
        assert_file_exists "$agents_dir/sdd-spec.md" "sdd-spec.md agent file"
        assert_file_exists "$agents_dir/sdd-design.md" "sdd-design.md agent file"
        assert_file_exists "$agents_dir/sdd-tasks.md" "sdd-tasks.md agent file"
        assert_file_exists "$agents_dir/sdd-apply.md" "sdd-apply.md agent file"
        assert_file_exists "$agents_dir/sdd-verify.md" "sdd-verify.md agent file"
        assert_file_exists "$agents_dir/sdd-archive.md" "sdd-archive.md agent file"
        assert_file_exists "$agents_dir/sdd-onboard.md" "sdd-onboard.md agent file"

        # readonly flags: explore and verify are readonly: false (issue #156 — readonly: true
        # blocks MCP tools and terminal in Cursor, not just file writes)
        assert_file_contains "$agents_dir/sdd-explore.md" "readonly: false" "sdd-explore is not readonly"
        assert_file_contains "$agents_dir/sdd-verify.md" "readonly: false" "sdd-verify is not readonly"

        # apply must NOT be readonly (it writes code)
        assert_file_not_contains "$agents_dir/sdd-apply.md" "readonly: true" "sdd-apply is NOT readonly"

        # All agent files must have substantial content
        for phase in sdd-init sdd-explore sdd-research sdd-propose sdd-spec sdd-design sdd-tasks sdd-apply sdd-verify sdd-archive sdd-onboard; do
            assert_file_size_min "$agents_dir/$phase.md" 200 "$phase agent has real content"
        done
    else
        log_fail "Cursor SDD install command failed"
    fi
}



test_antigravity_sdd_skills_path() {
    log_test "Antigravity: SDD skills install to ~/.gemini/antigravity-cli/skills/"
    cleanup_test_env

    # Antigravity is a desktop app — create the config dir to signal it's "installed"
    mkdir -p "$HOME/.gemini/antigravity"

    if $BINARY install --agent antigravity --component sdd --persona neutral 2>&1; then
        local skills_dir="$HOME/.gemini/antigravity-cli/skills"
        assert_dir_exists "$skills_dir" "Antigravity skills directory"
        assert_file_exists "$skills_dir/sdd-init/SKILL.md" "sdd-init skill"
        assert_file_exists "$skills_dir/sdd-apply/SKILL.md" "sdd-apply skill"
        assert_file_exists "$skills_dir/_shared/sdd-phase-common.md" "shared convention"
        assert_file_size_min "$skills_dir/sdd-init/SKILL.md" 100 "skill has real content"

        # Path regression guard: skills must NOT go to legacy Gemini paths.
        if [ -d "$HOME/.gemini/skills/sdd-init" ]; then
            log_fail "Skills went to ~/.gemini/skills/ instead of ~/.gemini/antigravity-cli/skills/"
        elif [ -d "$HOME/.gemini/antigravity/skills/sdd-init" ]; then
            log_fail "Skills went to legacy ~/.gemini/antigravity/skills/ instead of ~/.gemini/antigravity-cli/skills/"
        else
            log_pass "Skills correctly in ~/.gemini/antigravity-cli/skills/"
        fi
    else
        log_fail "Antigravity SDD install command failed"
    fi
}


# --- Category 12: Codex context7 TOML injection ---

test_codex_context7_in_toml() {
    log_test "Codex: context7 component writes [mcp_servers.context7] into config.toml (TOML strategy)"
    cleanup_test_env

    $BINARY install --agent codex --component context7 --persona neutral 2>&1 || true

    local config_toml="$HOME/.codex/config.toml"
    assert_file_exists "$config_toml" "Codex config.toml created by context7"
    assert_file_contains "$config_toml" "[mcp_servers.context7]" "Codex config.toml has [mcp_servers.context7] block"
    assert_file_contains "$config_toml" "https://mcp.context7.com/mcp" "Codex context7 block uses remote MCP URL"
    assert_file_not_contains "$config_toml" "context7-mcp" "Codex context7 block does not use local npx package"

    # Idempotent: re-running must not duplicate the block.
    $BINARY install --agent codex --component context7 --persona neutral 2>&1 || true
    local count
    # `grep -c` prints "0" AND exits 1 on zero matches, so `|| echo 0` would
    # yield the two-line string "0\n0" and break the numeric comparison below.
    count=$(grep -c "\[mcp_servers.context7\]" "$config_toml" 2>/dev/null || true)
    count=${count:-0}
    if [ "$count" -eq 1 ]; then
        log_pass "Codex context7 block is idempotent (exactly 1 entry)"
    else
        log_fail "Codex context7 block duplicated ($count entries)"
    fi
}

# --- Category 7: Injection integrity (guards against issue #4 regression) ---







# --- Category 9: SDD multi-mode tests ---




# ===========================================================================
# TIER 3 — Backup / restore tests (require RUN_BACKUP_TESTS=1)
# ===========================================================================






test_backup_claude_code_files() {
    log_test "Backup captures Claude Code files"
    cleanup_test_env
    setup_fake_configs

    if $BINARY install --agent claude-code --component permissions --persona neutral 2>&1; then
        local latest_backup
        latest_backup=$(find "$HOME/.gentle-ai/backups" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | sort | tail -1)
        if [ -n "$latest_backup" ] && [ -f "$latest_backup/manifest.json" ]; then
            log_pass "Claude Code backup snapshot with manifest created"
        else
            log_fail "No proper backup found for Claude Code install"
        fi
    else
        log_fail "Install for Claude backup test failed"
    fi
}

# ===========================================================================
# Test execution
# ===========================================================================

log_info "=== Tier 1: Basic binary & dry-run tests ==="

# Category 1a: Binary basics
test_binary_exists
test_binary_runs
test_version_command

# Category 1b: Dry-run output format
test_dry_run_output_format
test_dry_run_platform_detection
test_dry_run_detects_linux

# Category 1c: Agent flags
test_dry_run_agent_claude_code
# Category 1d: Preset flags
test_dry_run_preset_minimal
test_dry_run_preset_ecosystem
test_dry_run_preset_full
test_dry_run_preset_custom

# Category 1e: Preset component order validation
test_preset_minimal_components
test_preset_minimal_with_default_persona_includes_persona
test_preset_full_with_custom_persona_excludes_persona
test_preset_custom_no_components
test_preset_custom_explicit_components

# Category 1f: Individual component flags (all 8)
test_dry_run_component_engram
test_dry_run_component_sdd
test_dry_run_component_skills
test_dry_run_component_context7
test_dry_run_component_persona
# Category 1f2: SDD mode flag
# Category 1g: Invalid inputs
test_invalid_persona_rejected
test_invalid_component_rejected
test_invalid_preset_rejected
test_unknown_command_rejected

if [ "${RUN_FULL_E2E:-0}" = "1" ]; then
    log_info ""
    log_info "=== Tier 2: Component injection tests ==="

    # Category 2: Claude Code injection
    test_cc_engram_injection
    test_cc_sdd_injection
    test_cc_persona_gentleman
    test_cc_persona_neutral
    test_cc_persona_custom_does_nothing
    test_cc_skills_minimal
    test_cc_skills_full
    test_cc_skills_ecosystem
    test_cc_custom_skills_with_flag
    test_cc_custom_no_skills_flag_installs_nothing
    test_cc_custom_sdd_plus_skills
    test_cc_context7_injection
    test_cc_permissions_injection
    # Category 4: Full preset integration
    test_minimal_preset_claude_only_engram
    # Category 5: Content validation
    test_content_claude_md_sections_substantial
    test_content_skills_are_real
    test_content_mcp_json_valid
    # Category 6: Idempotency
    test_idempotent_sdd_claude
    test_idempotent_persona_claude
    test_idempotent_engram_claude
    test_idempotent_skills_claude
    test_codex_engram_injection
    test_codex_engram_idempotent

    # Category 8: Edge cases
    test_edge_persona_switch
    # Category 7: Injection integrity (issue #4 regression guard)
    # Category 9: SDD multi-mode
    # Category 10: Cursor native agent files
    test_cursor_sdd_subagents

    # Antigravity skills path
    test_antigravity_sdd_skills_path

    # Category 12: Codex context7 by-design skip
    test_codex_context7_in_toml

else
    log_skip "Tier 2 tests (set RUN_FULL_E2E=1 to enable)"
fi

if [ "${RUN_BACKUP_TESTS:-0}" = "1" ]; then
    log_info ""
    log_info "=== Tier 3: Backup/restore tests ==="
    test_backup_claude_code_files
else
    log_skip "Tier 3 tests (set RUN_BACKUP_TESTS=1 to enable)"
fi

# ---------------------------------------------------------------------------
# Summary & exit
# ---------------------------------------------------------------------------
print_summary
