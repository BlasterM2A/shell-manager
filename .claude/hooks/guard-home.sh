#!/usr/bin/env bash
# PreToolUse(Bash): block `shm setup`/`update` unless HOME points to a temp dir.
cmd=$(jq -r '.tool_input.command // empty')

if grep -Eq '(^|[[:space:];&|/])(shm|shell-manager|go run \.|task run --)[^;&|]*[[:space:]](setup|update)([[:space:]]|$)' <<<"$cmd" &&
	! grep -Eq 'HOME=("?\$\(mktemp|"?/tmp/)' <<<"$cmd"; then
	echo "Blocked: 'shm setup/update' against the real \$HOME (see CLAUDE.md). Use HOME=\$(mktemp -d)." >&2
	exit 2
fi
exit 0
