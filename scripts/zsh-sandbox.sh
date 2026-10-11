#!/usr/bin/env bash
# Run a zsh snippet against the shm shell config in a throwaway HOME.
set -euo pipefail

usage() {
	cat >&2 <<'USAGE'
Usage: scripts/zsh-sandbox.sh '<zsh script>'     (or: task zsh -- '<script>')

Materializes internal/shellmanager/testdata/TestShellConfigGolden.golden into a
fresh temporary HOME (~/.zshenv, ~/.zshrc, ~/.zsh/...) and runs the snippet with
`zsh -c` under a clean environment (env -i, PATH=/usr/bin:/bin).

Environment:
  ZSH_FLAGS=-u         extra zsh flags, e.g. -u (nounset) or -i (interactive;
                       works without a pty)
  PATH_EXTRA=dirs      appended to the sandbox PATH. The tool fragments (mise
                       activate, starship init, antidote, fzf, zoxide) call real
                       binaries that are not in the sandbox PATH, so interactive
                       runs print "command not found" for them. That is expected.
                       -i is mostly useful with something like
                       PATH_EXTRA=$HOME/.local/bin:$HOME/.local/share/mise/shims
  TERM_OVERRIDE=term   TERM for the snippet (default: dumb)
  KEEP_HOME=1          print the temporary HOME to stderr and do not delete it
USAGE
}

if [[ $# -lt 1 || -z "$1" ]]; then
	usage
	exit 2
fi

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
golden="$root/internal/shellmanager/testdata/TestShellConfigGolden.golden"
[[ -r "$golden" ]] || { echo "golden file not found: $golden" >&2; exit 1; }

H=$(mktemp -d)
if [[ "${KEEP_HOME:-}" == 1 ]]; then
	echo "sandbox HOME: $H" >&2
else
	trap 'rm -rf "$H"' EXIT
fi

# Sections: "== /<path relative to HOME> ==" followed by the file content.
awk -v h="$H" '
	/^== .* ==$/ {
		f = h substr($0, 4, length($0) - 6)
		d = f; sub(/\/[^\/]*$/, "", d)
		system("mkdir -p \"" d "\"")
		printf "" > f
		next
	}
	{ print > f }
' "$golden"

mkdir -p "$H/.local/bin" "$H/.local/share/mise/shims"

sandbox_path=/usr/bin:/bin
[[ -n "${PATH_EXTRA:-}" ]] && sandbox_path="$sandbox_path:$PATH_EXTRA"

# ZSH_FLAGS is intentionally unquoted: it may hold several flags.
# shellcheck disable=SC2086
env -i HOME="$H" PATH="$sandbox_path" TERM="${TERM_OVERRIDE:-dumb}" \
	zsh ${ZSH_FLAGS:-} -c "$1"
