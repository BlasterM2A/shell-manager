#!/usr/bin/env bash
# SessionStart: keep mise from parsing the shm tool manifest
# internal/shellmanager/registry/tools/mise.toml as if it were a mise config.
#
# Without this, any command run with cwd inside that directory makes every mise
# shim (jq, task, golangci-lint, python3, ...) fail with "Config files in ...
# are not trusted". mise only accepts ABSOLUTE paths in
# MISE_IGNORED_CONFIG_PATHS (no relative paths, no globs), settings.json `env`
# values are literal (no $CLAUDE_PROJECT_DIR expansion) and a [settings] entry in
# the root mise.toml is read too late (the bad file is parsed first), so the
# absolute path is computed here and exported to the Bash tool through
# CLAUDE_ENV_FILE. Nothing machine-specific is committed. Both the checkout this
# script lives in and the git toplevel of the session cwd are covered, because
# in a worktree CLAUDE_PROJECT_DIR points to the original checkout while
# commands run in the worktree.
sub=internal/shellmanager/registry/tools
[ -n "${CLAUDE_ENV_FILE:-}" ] || exit 0

paths=()
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
paths+=("$root/$sub")
cwd=$(jq -r '.cwd // empty' 2>/dev/null)
if [ -n "$cwd" ] && top=$(git -C "$cwd" rev-parse --show-toplevel 2>/dev/null) && [ "$top" != "$root" ]; then
	paths+=("$top/$sub")
fi

list=$(IFS=:; echo "${paths[*]}")
# Keep whatever the user already ignores.
[ -n "${MISE_IGNORED_CONFIG_PATHS:-}" ] && list="$MISE_IGNORED_CONFIG_PATHS:$list"
printf 'export MISE_IGNORED_CONFIG_PATHS=%q\n' "$list" >>"$CLAUDE_ENV_FILE"
exit 0
