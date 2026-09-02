# Quickstart

## Prerequisites

### macOS

- Homebrew installed and available in PATH.
- `git` available.
- If Homebrew requires trust, run `brew trust --formula gentleman-programming/tap/gentle-ai` once for Gentle AI only.
  - To install several tools from this tap, use `brew trust gentleman-programming/tap` instead. It trusts all current and future formulas, casks, and external commands published in the tap.

### Ubuntu/Debian (and derivatives like Linux Mint, Pop!\_OS)

- `apt-get` available (standard on these distros).
- `sudo` access for package installs.
- `git` available.
- If Node.js is missing, `gentle-ai install` prints this install hint: NodeSource LTS setup + `apt-get install -y nodejs` (npm comes bundled).
- If using Homebrew on Linux, Bubblewrap may require unprivileged user namespaces; see `docs/usage.md#homebrew-upgrade-troubleshooting`.

### Arch Linux (and derivatives like Manjaro, EndeavourOS)

- `pacman` available (standard on these distros).
- `sudo` access for package installs.
- `git` available.
- If Node.js is missing, `gentle-ai install` prints this install hint: `pacman -S --noconfirm nodejs npm`.

### Fedora / RHEL family (Fedora, CentOS Stream, Rocky Linux, AlmaLinux)

- `dnf` available (standard on these distros).
- `sudo` access for package installs.
- `git` available.
- If Node.js is missing, `gentle-ai install` prints this install hint: NodeSource LTS setup + `dnf install -y nodejs` (npm comes bundled).

### All platforms

- Git 2.38+.
- Go 1.25.10+ (for building from source).
- Node.js 18+ and npm: `gentle-ai install` checks these as required prerequisites on every platform and prints a warning with a distro-specific install hint (see above) if either is missing — regardless of which agents/components you select. It does not install them for you, and it does not install agent runtimes either: if a selected agent isn't detected, `gentle-ai install` refuses and prints the exact `npm install -g` (or equivalent) command for you to run yourself. Node.js/npm are required when CodeGraph is selected as a first-class component; Gentle AI installs it via `npm install -g`.
- Pi installed and available as `pi` on `PATH` if you select the Pi agent.

### Windows

- Go 1.25.10+ is required to build the fork checkout.
- The repository keeps the upstream module path for source compatibility, so `go install github.com/Alexma03/...` is not supported.
- Build `./cmd/gentle-ai` in the `Alexma03/gentle-ai` `custom/main` checkout, then replace the inactive executable. The built-in updater deliberately skips Gentle AI itself.

## Personal Fork Update Policy

Install and update from the fork checkout:

```bash
git clone --branch custom/main https://github.com/Alexma03/gentle-ai.git
cd gentle-ai
./scripts/install-personal.sh
```

The installer proves which `gentle-ai` command is active. Run only the exact absolute `…/gentle-ai sync` invocation it prints. If another installation shadows the fork, activation fails closed but the same absolute invocation remains safe. For later updates, run `git pull --ff-only origin custom/main` in that checkout and rerun `./scripts/install-personal.sh`. Do not run the upstream installer or upstream `go install` command: either would replace the personal fork binary. `gentle-ai upgrade` skips self-upgrade with a manual hint and still manages the remaining registered tools.

## Run

```bash
go run ./cmd/gentle-ai install --dry-run
```

Use `--dry-run` first to validate selections and execution plan without applying changes. The dry-run output includes a `Platform decision` line showing the detected OS, distro, package manager, and support status.

## First real install

```bash
go run ./cmd/gentle-ai install
```

The installer detects your platform automatically — no flags needed to select macOS vs Linux. Install commands are resolved through the appropriate package manager (brew, apt, pacman, or dnf) based on detection.

After completion, verify that agent configs and selected components were installed to their expected paths.

The agents you select during install become the default scope for future `gentle-ai sync` runs. Gentle AI records that selection in `~/.gentle-ai/state.json` and does not automatically sync every agent config directory that exists on your machine. To check what will be updated after an upgrade, run:

```bash
gentle-ai sync --dry-run
```

To update a different set explicitly, pass every target agent:

```bash
```

## Verification outcome

When checks pass, installer reports:

If something looks wrong after install, run `gentle-ai doctor` for a read-only health check. It verifies tool binaries, `state.json` validity, Engram MCP reachability, and disk space — each check reports pass/warn/fail with a remedy hint.

## Hardening recommendations for users

For broader protection across npm packages you install yourself, set these once on your machine:

- `npm config set ignore-scripts true` — blocks postinstall scripts globally; the primary supply-chain attack vector.
- `npm config set min-release-age 3` — skip packages published in the last 3 days; catches malicious typosquats before you install them.
- `npm config set allow-git none` — block git: dependencies, which can be moving targets.

Optional wrapper tools for extra defense:

- [`npq`](https://github.com/lirantal/npq) — audits a package against several heuristics before it installs.
- [`sfw`](https://socket.dev/) (Socket Firewall) — runtime guard that intercepts suspicious behavior at install/run time.

## Unsupported platforms

If you run the installer on an unsupported OS or Linux distro, it exits immediately with an error:

- `unsupported operating system: only macOS, Linux, and Windows are supported (detected <os>)`
- `unsupported linux distro: Linux support is limited to Ubuntu/Debian, Arch, and Fedora/RHEL family (detected <distro>)`
