#!/usr/bin/env bash
# Tests for .claude/hooks/guard-home.sh. The hook JSON is fed on stdin exactly
# as Claude Code does; the exit code is 2 when blocked and 0 when allowed.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOOK="$SCRIPT_DIR/../.claude/hooks/guard-home.sh"

FAILURES=0

# run_hook COMMAND: print the hook's exit code for a Bash tool call.
run_hook() {
    jq -n --arg c "$1" '{tool_name: "Bash", tool_input: {command: $c}}' | bash "$HOOK" 2>/dev/null
    echo $?
}

assert_blocked() {
    local cmd="$1" got
    got="$(run_hook "$cmd")"
    if [ "$got" = "2" ]; then
        echo "PASS: blocked: $cmd"
    else
        echo "FAIL: should be blocked (exit 2, got $got): $cmd"
        FAILURES=$((FAILURES + 1))
    fi
}

assert_allowed() {
    local cmd="$1" got
    got="$(run_hook "$cmd")"
    if [ "$got" = "0" ]; then
        echo "PASS: allowed: $cmd"
    else
        echo "FAIL: should be allowed (exit 0, got $got): $cmd"
        FAILURES=$((FAILURES + 1))
    fi
}

# Direct invocations against the real $HOME are blocked.
assert_blocked 'shm setup'
assert_blocked 'shm update'
assert_blocked 'shell-manager update --force'
assert_blocked './bin/shell-manager setup'
assert_blocked '~/.local/bin/shm setup --yes'
assert_blocked 'go run . setup'
assert_blocked 'task run -- update'
assert_blocked 'cd /tmp && shm setup'
assert_blocked 'sudo shm setup'
assert_blocked 'env FOO=1 shm setup'
assert_blocked 'bash -c "shm setup"'
assert_blocked 'echo $(shm setup)'
assert_blocked $'echo hi\nshm update'
assert_blocked 'shm --config x setup'

# The HOME assignment must be in the same segment as the invocation.
assert_blocked 'HOME=/tmp/x true; shm setup'
assert_blocked 'HOME=/tmp/x true && shm setup'
assert_blocked 'export HOME=/tmp/x; shm setup'
assert_blocked 'HOME=/tmp/x echo hi | shm setup'

# Not a temp dir (or not provably one).
assert_blocked 'HOME=/home/someone shm setup'
assert_blocked 'HOME=/tmp shm setup'
assert_blocked 'HOME=/tmp/../root shm setup'
assert_blocked 'HOME=$H shm setup'
assert_blocked 'HOME=$HOME shm setup'
assert_blocked 'HOME=/tmpfoo/x shm setup'

# self-update ignores $HOME, so a temp HOME does not help.
assert_blocked 'shm self-update'
assert_blocked 'HOME=$(mktemp -d) shm self-update'

# Temp HOME in the same segment is allowed.
assert_allowed 'HOME=$(mktemp -d) shm setup'
assert_allowed 'HOME="$(mktemp -d)" shm update'
assert_allowed 'HOME=/tmp/x shm setup'
assert_allowed 'HOME=/var/tmp/x shell-manager update'
assert_allowed 'HOME=${TMPDIR}/x shm setup'
assert_allowed 'HOME=`mktemp -d` shm setup'
assert_allowed 'HOME=$(mktemp -d) go run . setup'
assert_allowed 'HOME=$(mktemp -d) ./bin/shell-manager doctor && shm doctor'
assert_allowed 'env HOME=$(mktemp -d) shm setup'
assert_allowed 'true; HOME=$(mktemp -d) shm setup'
assert_allowed 'HOME=$(mktemp -d) bash -c "shm setup"'

# Mentions of the commands in text are not invocations.
assert_allowed "grep -n 'shm update' README.md"
assert_allowed 'grep -n run\ shm\ update README.md'
assert_allowed 'grep -rn "shm setup" docs'
assert_allowed 'echo shm setup'
assert_allowed "git commit -m 'document shm setup; and shm update'"
assert_allowed 'cat docs/shm-setup.md'
assert_allowed 'echo hi # shm setup'

# Heredoc bodies are data, not commands (the shell only runs substitutions in
# an unquoted one).
NL=$'\n'
assert_allowed "python3 - <<'EOF'${NL}hint = \"Run \`shm update --config-only\` now\"${NL}shm setup${NL}EOF"
assert_allowed "cat > notes.md <<\"EOF\"${NL}run shm update${NL}EOF"
assert_allowed "cat <<-EOF${NL}	shm setup${NL}	EOF"
assert_allowed "cat <<'A' <<'B'${NL}shm setup${NL}A${NL}shm update${NL}B"
assert_allowed "cat <<< 'shm setup'"
assert_blocked "cat <<EOF${NL}\$(shm setup)${NL}EOF"
assert_blocked "cat <<EOF${NL}\`shm update\`${NL}EOF"
assert_blocked "cat <<'EOF'${NL}text${NL}EOF${NL}shm setup"
assert_blocked "cat <<'EOF' && shm setup${NL}text${NL}EOF"
assert_allowed "cat <<'EOF'${NL}shm setup" # unterminated: bash still reads it as text
assert_blocked "cat <<EOF${NL}\$(shm setup)" # ...but substitutions in an unquoted one run
assert_allowed "cat <<EOF${NL}\$(HOME=\$(mktemp -d) shm setup)${NL}EOF"

# Other shm commands and unrelated tools are allowed.
assert_allowed 'shm doctor'
assert_allowed 'shm --version'
assert_allowed 'shm setup-guide'
assert_allowed 'shm doctor | grep update'
assert_allowed 'go run . doctor'
assert_allowed 'go test ./...'
assert_allowed 'task test'
assert_allowed 'task lint'
assert_allowed 'ls'
assert_allowed ''

echo ""
if [ "$FAILURES" -gt 0 ]; then
    echo "$FAILURES test(s) failed"
    exit 1
fi
echo "All tests passed"
