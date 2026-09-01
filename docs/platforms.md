# Supported Platforms

← [Back to README](../README.md)

---

| Platform | Package Manager | Status |
|----------|----------------|--------|
| macOS (Apple Silicon + Intel) | Homebrew | Supported |
| Linux (Ubuntu/Debian) | apt | Supported |
| Linux (Arch) | pacman | Supported |
| Linux (Fedora/RHEL family) | dnf | Supported |
| Windows 10/11 | `go install` (Go toolchain) | Supported (binary distribution held) |

Derivatives are detected via `ID_LIKE` in `/etc/os-release` (Linux Mint, Pop!_OS, Manjaro, EndeavourOS, CentOS Stream, Rocky Linux, AlmaLinux, etc.).

Release archives are currently produced for macOS and Linux only. Windows source compatibility remains supported, but official Windows executable/archive assets and Scoop publication are temporarily unavailable pending the [Authenticode restoration gate](release-signing.md#windows-distribution-restoration-gate).

## Windows Notes

- **Install from source** with Go 1.25.10+:
  `go install github.com/gentleman-programming/gentle-ai/v2/cmd/gentle-ai@latest`.
- **`gentle-ai upgrade` updates itself automatically on release channels when Go 1.25.10+ is on `PATH`.** It runs `go install …/cmd/gentle-ai@vX.Y.Z` pinned to the exact release tag. The module is verified against the Go checksum database (`sum.golang.org`) — a different trust anchor than the minisign signature used for the Linux/macOS release binaries, not a missing one.
  Because `go install` writes to `GOBIN` (or `GOPATH\bin`), which is not necessarily the directory your shell resolves, the upgrade checks the destination afterwards and warns — naming both full paths — if a different `gentle-ai.exe` earlier on `PATH` would keep running.
   On the beta/development channel, `$env:GENTLE_AI_CHANNEL="beta"; gentle-ai upgrade` advances the binary from `main` and refreshes managed tools. If a manual source install sees stale `main` commits, run `GOPROXY=direct go install github.com/gentleman-programming/gentle-ai/v2/cmd/gentle-ai@main` (PowerShell: `$env:GOPROXY="direct"; go install github.com/gentleman-programming/gentle-ai/v2/cmd/gentle-ai@main`).
   Re-running either installer defaults to stable, so preserve beta explicitly: `curl -fsSL https://raw.githubusercontent.com/Gentleman-Programming/gentle-ai/main/scripts/install.sh | bash -s -- --channel beta` on macOS/Linux, or `$env:GENTLE_AI_CHANNEL="beta"; irm https://raw.githubusercontent.com/Gentleman-Programming/gentle-ai/main/scripts/install.ps1 | iex` in PowerShell.
- **Without Go on `PATH`, the upgrader fails closed.** It downloads and executes nothing, and prints the runnable `go install` command instead.
- **Scoop and official Windows binaries are still temporarily unavailable.** No unsigned artifact is ever downloaded and `gentle-ai upgrade` never executes a remote update script.
- **npm global installs** do not require `sudo` on Windows (user-writable by default).
- **curl** is pre-installed on Windows 10+ and does not require separate installation.
- **PowerShell** is the default shell when `$SHELL` is not set.
- **PowerShell source-installer output** is forced to UTF-8 and installs through Go's configured `GOBIN`/`GOPATH`.

---

## Windows Config Paths

| Agent | Windows Config Path |
|-------|-------------------|
| Claude Code | `%USERPROFILE%\.claude\` |
| Cursor | `%USERPROFILE%\.cursor\` |
| Codex | `%USERPROFILE%\.codex\` |
| Antigravity | `%USERPROFILE%\.gemini\antigravity\` |
| Pi | `%USERPROFILE%\.pi\` (Pi config, project agents/chains, Gentle AI support assets) |
