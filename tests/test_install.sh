#!/usr/bin/env bash
# Tests for the install.sh bootstrap. No network: curl and uname are stubbed
# and a fake release (tarball + checksums.txt) is served from a temp dir.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_SH="$SCRIPT_DIR/../install.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

FAKE_BIN="$TMP/fakebin"
export FAKE_RELEASE_DIR="$TMP/release"
mkdir -p "$FAKE_BIN" "$FAKE_RELEASE_DIR" "$TMP/build"

# Fake release: a tarball per arch containing a fake shell-manager binary
# that echoes its arguments.
printf '#!/bin/sh\necho fake-shell-manager "$@"\n' > "$TMP/build/shell-manager"
chmod +x "$TMP/build/shell-manager"
for arch in amd64 arm64; do
    tar -czf "$FAKE_RELEASE_DIR/shell-manager_linux_${arch}.tar.gz" -C "$TMP/build" shell-manager
done
(cd "$FAKE_RELEASE_DIR" && sha256sum shell-manager_linux_*.tar.gz > checksums.txt)

# Fake curl: copies $FAKE_RELEASE_DIR/<basename of URL> to the -o target.
cat > "$FAKE_BIN/curl" <<'EOS'
#!/usr/bin/env bash
out="" url="" prev=""
for arg in "$@"; do
    if [ "$prev" = "-o" ]; then out="$arg"; elif [[ "$arg" != -* ]]; then url="$arg"; fi
    prev="$arg"
done
cp "$FAKE_RELEASE_DIR/$(basename "$url")" "$out"
EOS
cat > "$FAKE_BIN/uname" <<'EOS'
#!/usr/bin/env bash
echo "${FAKE_ARCH:-x86_64}"
EOS
chmod +x "$FAKE_BIN/curl" "$FAKE_BIN/uname"
export PATH="$FAKE_BIN:$PATH"

export SHELL_MANAGER_BIN_DIR="$TMP/bin"
export SHELL_MANAGER_STATE_DIR="$TMP/state"
export SHELL_MANAGER_RELEASE_BASE="https://example.invalid/download"
export SHELL_MANAGER_OS_RELEASE="$TMP/os-release"
export SHELL_MANAGER_TTY="$TMP/no-tty" # default: no terminal

source "$INSTALL_SH"
set +e # install.sh enables -e; assertions must keep running

FAILURES=0

assert_eq() {
    local expected="$1" actual="$2" desc="$3"
    if [ "$expected" = "$actual" ]; then
        echo "PASS: $desc"
    else
        echo "FAIL: $desc (expected '$expected', got '$actual')"
        FAILURES=$((FAILURES + 1))
    fi
}

assert_true() {
    local desc="$1"
    shift
    if "$@"; then echo "PASS: $desc"; else echo "FAIL: $desc"; FAILURES=$((FAILURES + 1)); fi
}

assert_false() {
    local desc="$1"
    shift
    if "$@"; then echo "FAIL: $desc"; FAILURES=$((FAILURES + 1)); else echo "PASS: $desc"; fi
}

# --- detect_arch ---
assert_eq "amd64" "$(export FAKE_ARCH=x86_64; detect_arch)" "x86_64 maps to amd64"
assert_eq "arm64" "$(export FAKE_ARCH=aarch64; detect_arch)" "aarch64 maps to arm64"
assert_false "unsupported arch fails" bash -c "export FAKE_ARCH=mips64; source '$INSTALL_SH'; detect_arch" 2>/dev/null

# --- check_os ---
printf 'ID=ubuntu\nID_LIKE=debian\n' > "$SHELL_MANAGER_OS_RELEASE"
assert_true "ubuntu is supported" check_os
printf 'ID=fedora\n' > "$SHELL_MANAGER_OS_RELEASE"
assert_false "fedora is rejected" bash -c "source '$INSTALL_SH'; check_os" 2>/dev/null
printf 'ID=ubuntu\nID_LIKE=debian\n' > "$SHELL_MANAGER_OS_RELEASE"

# --- install_binary ---
assert_true "installs the binary" bash -c "source '$INSTALL_SH'; install_binary amd64" >/dev/null
assert_eq "fake-shell-manager --version" "$("$SHELL_MANAGER_BIN_DIR/shell-manager" --version)" "installed binary runs"
second="$(bash -c "source '$INSTALL_SH'; install_binary amd64")"
assert_true "re-run skips the download" grep -q "already up to date" <<<"$second"

# --- link_alias ---
assert_true "links the alias" bash -c "source '$INSTALL_SH'; link_alias" >/dev/null
assert_eq "shell-manager" "$(readlink "$SHELL_MANAGER_BIN_DIR/shm")" "alias is a relative link to shell-manager"
assert_eq "fake-shell-manager --version" "$("$SHELL_MANAGER_BIN_DIR/shm" --version)" "alias runs the binary"
assert_true "re-linking is idempotent" bash -c "source '$INSTALL_SH'; link_alias" >/dev/null
assert_eq "shell-manager" "$(readlink "$SHELL_MANAGER_BIN_DIR/shm")" "alias unchanged after re-link"
rm -f "$SHELL_MANAGER_BIN_DIR/shm"
printf '#!/bin/sh\necho mine\n' > "$SHELL_MANAGER_BIN_DIR/shm"
chmod +x "$SHELL_MANAGER_BIN_DIR/shm"
bash -c "source '$INSTALL_SH'; link_alias" >/dev/null 2>&1
assert_eq "mine" "$("$SHELL_MANAGER_BIN_DIR/shm")" "a real file named shm is left alone"
rm -f "$SHELL_MANAGER_BIN_DIR/shm"

# --- main: final question ---
run_main() { # $1: tty file contents, or "-" for no terminal
    if [ "$1" = "-" ]; then
        SHELL_MANAGER_TTY="$TMP/no-tty" bash -c "source '$INSTALL_SH'; main" 2>&1
    else
        printf '%s' "$1" > "$TMP/tty"
        SHELL_MANAGER_TTY="$TMP/tty" bash -c "source '$INSTALL_SH'; main" 2>&1
    fi
}
out="$(run_main $'\n')"
assert_true "empty answer runs setup" grep -q "fake-shell-manager setup" <<<"$out"
assert_true "the question is shown" grep -q "Run 'shm setup' now? \[Y/n\]" <<<"$out"
out="$(run_main $'y\n')"
assert_true "y runs setup" grep -q "fake-shell-manager setup" <<<"$out"
out="$(run_main $'n\n')"
assert_false "n does not run setup" grep -q "fake-shell-manager setup" <<<"$out"
assert_true "n prints the hint" grep -q "Run \`shm setup\` whenever you're ready" <<<"$out"
out="$(run_main -)"
assert_false "no terminal does not run setup" grep -q "fake-shell-manager setup" <<<"$out"
assert_false "no terminal does not ask" grep -q "\[Y/n\]" <<<"$out"
assert_true "no terminal prints the hint" grep -q "whenever you're ready" <<<"$out"

# A corrupted checksum must abort and leave no new binary behind.
rm -f "$SHELL_MANAGER_BIN_DIR/shell-manager" "$SHELL_MANAGER_STATE_DIR/bootstrap.sha256"
sed -i "s/^[0-9a-f]*/$(printf '0%.0s' $(seq 64))/" "$FAKE_RELEASE_DIR/checksums.txt"
assert_false "bad checksum aborts" bash -c "source '$INSTALL_SH'; install_binary amd64" 2>/dev/null
assert_false "no binary after a bad checksum" test -e "$SHELL_MANAGER_BIN_DIR/shell-manager"

echo ""
if [ "$FAILURES" -gt 0 ]; then
    echo "$FAILURES test(s) failed"
    exit 1
fi
echo "All tests passed"
