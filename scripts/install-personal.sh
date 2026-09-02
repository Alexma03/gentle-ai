#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
install_dir="${GENTLE_AI_INSTALL_DIR:-}"
if [[ "${1:-}" == "--install-dir" ]]; then
  [[ $# -eq 2 && -n "${2:-}" ]] || {
    printf '%s\n' 'Usage: ./scripts/install-personal.sh [--install-dir ABSOLUTE_DIRECTORY]' >&2
    exit 2
  }
  install_dir="$2"
elif [[ $# -ne 0 ]]; then
  printf '%s\n' 'Usage: ./scripts/install-personal.sh [--install-dir ABSOLUTE_DIRECTORY]' >&2
  exit 2
fi
if [[ -z "$install_dir" ]]; then install_dir="${GOBIN:-}"; fi
if [[ -z "$install_dir" ]]; then install_dir="$(go env GOBIN)"; fi
if [[ -z "$install_dir" ]]; then
  go_path="$(go env GOPATH)"
  install_dir="${go_path%%:*}/bin"
fi
[[ -n "$install_dir" ]] || {
  printf '%s\n' 'Could not determine an install directory. Pass --install-dir explicitly.' >&2
  exit 2
}
mkdir -p "${install_dir}"
install_dir="$(cd "$install_dir" && pwd -P)"

tmp="$(mktemp "${install_dir}/.gentle-ai.XXXXXX")"
trap 'rm -f "${tmp}"' EXIT

(
  cd "${repo_root}"
  CGO_ENABLED=0 go build -trimpath -o "${tmp}" ./cmd/gentle-ai
)
chmod 0755 "${tmp}"
mv -f "${tmp}" "${install_dir}/gentle-ai"
trap - EXIT

installed="${install_dir}/gentle-ai"
printf 'Installed personal Gentle AI fork to %s\n' "$installed"
active="$(command -v gentle-ai 2>/dev/null || true)"
if [[ -z "$active" || ! "$active" -ef "$installed" ]]; then
  printf 'Activation check failed: command -v gentle-ai resolved %s instead of %s.\n' "${active:-nothing}" "$installed" >&2
  printf 'Run exactly: %q sync\n' "$installed" >&2
  exit 3
fi
printf 'Activation check passed: %s\n' "$active"
printf 'Run exactly: %q sync\n' "$installed"
