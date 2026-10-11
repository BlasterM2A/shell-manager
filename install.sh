#!/usr/bin/env bash
# shell-manager bootstrap: installs the shell-manager binary (and the shm
# alias) from the latest GitHub release, checksum verified, then offers to
# run `shm setup`.
#   curl -fsSL https://raw.githubusercontent.com/BlasterM2A/shell-manager/master/install.sh | bash
set -euo pipefail

REPO="BlasterM2A/shell-manager"
BIN_DIR="${SHELL_MANAGER_BIN_DIR:-$HOME/.local/bin}"
SHM_DIR="${SHM_DIR:-$HOME/.shm}"
OS_RELEASE="${SHELL_MANAGER_OS_RELEASE:-/etc/os-release}"
RELEASE_BASE="${SHELL_MANAGER_RELEASE_BASE:-https://github.com/$REPO/releases/latest/download}"
TTY="${SHELL_MANAGER_TTY:-/dev/tty}"
STAMP_FILE="$SHM_DIR/bootstrap.sha256"

log() { printf '\033[1;34m•\033[0m %s\n' "$1"; }
die() {
    printf '\033[1;31m✗\033[0m %s\n' "$1" >&2
    exit 1
}

check_os() {
    [ -f "$OS_RELEASE" ] || die "Cannot detect the OS: $OS_RELEASE not found. Ubuntu/Debian only."
    local ids
    ids="$(. "$OS_RELEASE" && echo "${ID:-} ${ID_LIKE:-}")"
    case " $ids " in
        *" debian "* | *" ubuntu "*) ;;
        *) die "Unsupported OS (${ids% }). shell-manager supports Ubuntu/Debian only." ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64 | amd64) echo amd64 ;;
        aarch64 | arm64) echo arm64 ;;
        *) die "Unsupported architecture: $(uname -m)" ;;
    esac
}

install_binary() {
    local arch="$1" asset tmp expected
    asset="shell-manager_linux_${arch}.tar.gz"
    tmp="$(mktemp -d)"
    curl -fsSL -o "$tmp/checksums.txt" "$RELEASE_BASE/checksums.txt" ||
        { rm -rf "$tmp"; die "Could not download checksums.txt"; }
    expected="$(grep " ${asset}\$" "$tmp/checksums.txt" || true)"
    [ -n "$expected" ] || { rm -rf "$tmp"; die "No checksum for $asset in the latest release"; }

    if [ -x "$BIN_DIR/shell-manager" ] && [ -f "$STAMP_FILE" ] && [ "$(cat "$STAMP_FILE")" = "$expected" ]; then
        rm -rf "$tmp"
        log "shell-manager is already up to date"
        return 0
    fi

    log "Downloading $asset..."
    curl -fsSL -o "$tmp/$asset" "$RELEASE_BASE/$asset" || { rm -rf "$tmp"; die "Could not download $asset"; }
    (cd "$tmp" && printf '%s\n' "$expected" | sha256sum -c --status) ||
        { rm -rf "$tmp"; die "Checksum verification failed for $asset"; }
    tar -xzf "$tmp/$asset" -C "$tmp" shell-manager
    mkdir -p "$BIN_DIR" "$SHM_DIR"
    install -m 0755 "$tmp/shell-manager" "$BIN_DIR/shell-manager"
    printf '%s\n' "$expected" > "$STAMP_FILE"
    rm -rf "$tmp"
    log "Installed $BIN_DIR/shell-manager"
}

# Short alias: shm → shell-manager. A real file named shm is never replaced.
link_alias() {
    local alias="$BIN_DIR/shm"
    if [ -e "$alias" ] && [ ! -L "$alias" ]; then
        log "$alias exists and is not a link; skipping the shm alias"
        return 0
    fi
    ln -sfn shell-manager "$alias"
}

# ask_setup asks on the terminal whether to run setup now (Enter = yes).
# Without a usable terminal (CI, a pipe with no tty) it doesn't ask: no.
ask_setup() {
    { : <"$TTY"; } 2>/dev/null || return 1
    local answer=""
    printf "Run 'shm setup' now? [Y/n] "
    IFS= read -r answer <"$TTY" || answer=""
    case "$answer" in
        "" | y | Y | yes | Yes | YES) return 0 ;;
        *) return 1 ;;
    esac
}

main() {
    check_os
    local arch
    arch="$(detect_arch)"
    install_binary "$arch"
    link_alias
    if ask_setup; then
        # With `curl | bash`, stdin is the pipe: give setup the real terminal.
        exec "$BIN_DIR/shell-manager" setup <"$TTY"
    fi
    log "Run \`shm setup\` whenever you're ready."
}

# Only run main when executed (./install.sh, bash install.sh, curl | bash),
# not when sourced by tests.
if ! (return 0 2>/dev/null); then
    main "$@"
fi
