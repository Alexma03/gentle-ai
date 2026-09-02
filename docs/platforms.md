# Supported Platforms

← [Back to README](../README.md)

---

| Platform | Package Manager | Status |
|----------|----------------|--------|
| macOS (Apple Silicon + Intel) | Homebrew | Supported |
| Linux (Ubuntu/Debian) | apt | Supported |
| Linux (Arch) | pacman | Supported |
| Linux (Fedora/RHEL family) | dnf | Supported |
| Windows 10/11 | Build from the personal fork checkout | Supported |

Derivatives are detected via `ID_LIKE` in `/etc/os-release` (Linux Mint, Pop!_OS, Manjaro, EndeavourOS, CentOS Stream, Rocky Linux, AlmaLinux, etc.).

Release archives are currently produced for macOS and Linux only. Windows source compatibility remains supported, but official Windows executable/archive assets and Scoop publication are temporarily unavailable pending the [Authenticode restoration gate](release-signing.md#windows-distribution-restoration-gate).

## Windows Notes

- Build `./cmd/gentle-ai` with Go 1.25.10+ from the `Alexma03/gentle-ai` `custom/main` checkout.
- The repository retains the upstream Go module identity for compatibility, so `go install github.com/Alexma03/...` is not a valid install path.
- Replace the installed executable only after the running process exits, then run `gentle-ai sync`.
- `gentle-ai upgrade` deliberately skips Gentle AI itself and continues updating other managed tools; this prevents an upstream release from replacing the personal fork.
- npm global installs do not require `sudo` on Windows.

---

## Windows Config Paths

| Agent | Windows Config Path |
|-------|-------------------|
| Claude Code | `%USERPROFILE%\.claude\` |
| Cursor | `%USERPROFILE%\.cursor\` |
| Codex | `%USERPROFILE%\.codex\` |
| Antigravity | `%USERPROFILE%\.gemini\antigravity\` |
| Pi | `%USERPROFILE%\.pi\` (Pi config, project agents/chains, Gentle AI support assets) |
