#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
install_dir="${GOBIN:-${HOME}/.local/bin}"
mkdir -p "${install_dir}"

tmp="$(mktemp "${install_dir}/.gentle-ai.XXXXXX")"
trap 'rm -f "${tmp}"' EXIT

(
  cd "${repo_root}"
  CGO_ENABLED=0 go build -trimpath -o "${tmp}" ./cmd/gentle-ai
)
chmod 0755 "${tmp}"
mv -f "${tmp}" "${install_dir}/gentle-ai"
trap - EXIT

printf 'Installed personal Gentle AI fork to %s\n' "${install_dir}/gentle-ai"
printf 'Run gentle-ai sync to refresh managed assets.\n'
