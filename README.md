# shell-manager

A CLI/TUI that bootstraps and maintains a zsh environment on Ubuntu/Debian.
`shm` is its short alias. It installs and keeps up to date:

- **Built-in** (always installed; shell-manager needs them): zsh (as the login
  shell) and [mise](https://mise.jdx.dev/)
- [starship](https://starship.rs/), [zoxide](https://github.com/ajeetdsouza/zoxide)
  and [fzf](https://github.com/junegunn/fzf), installed through mise
- [antidote](https://github.com/mattmc3/antidote) with zsh-autosuggestions and
  zsh-syntax-highlighting
- JetBrainsMono Nerd Font
- AI CLIs (optional, a failure is only a warning): Claude Code, Junie,
  Antigravity CLI (`agy`)

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/BlasterM2A/shell-manager/master/install.sh | bash
```

This installs the latest release binary to `~/.local/bin/shell-manager`
(checksum verified) plus the `~/.local/bin/shm` alias, then asks
`Run 'shm setup' now? [Y/n]`. Without a terminal it doesn't ask; run
`shm setup` yourself. `setup` asks for your **sudo** password (apt packages)
and possibly your **chsh** password (login shell).

After the first `setup`:

1. Run `exec zsh`, or open a new terminal. If a new terminal still opens
   your old shell, log out and back in (some terminals only pick up a new
   login shell in a new session).
2. Close **all** terminal windows and reopen the terminal (a running terminal
   keeps the font list it loaded at start), then select **JetBrainsMono Nerd
   Font** in its font settings.

## Migrating

**From shell-setup (v0.1.x):** run the install one-liner once and answer
"Y". `shm setup` moves `~/.config/shell-setup` and `~/.local/state/shell-setup`
to their `shell-manager` names, rewrites `~/.zshrc` to point at them and removes
`~/.local/bin/shell-setup` and the `shs` alias. Your configuration and the
record of what you edited by hand are kept. If anything can't be moved,
`shm doctor` lists it under "shell-setup leftovers".

**From the original install.sh script:**

- Your old `~/.zshrc` is backed up to `~/.zshrc.bak.<timestamp>` and replaced
  by a one-line stub. Move your personal lines into `~/.config/zsh/*.d`
  (see [Where things live](#where-things-live)).
- starship, zoxide and fzf are reinstalled via mise, and the Nerd Font moves
  to `~/.local/share/fonts/JetBrainsMonoNerdFont`. Once each new copy is in
  place, the old one is removed automatically (`~/.local/bin/starship`,
  `~/.local/bin/zoxide`, apt's `fzf` — which asks for sudo — and the old
  `JetBrainsMono*NerdFont*.ttf` files in `~/.local/share/fonts`, after which
  the font cache is refreshed). Reopen your terminal afterwards: one that was
  already running can show the old fonts as empty boxes. If a removal fails,
  `shm doctor` lists the leftover and `shm update` retries it.

## Commands

| Command | What it does |
|---|---|
| `shm setup` | Install what is missing, write the shell config, make zsh the login shell |
| `shm update` | Same as setup, plus update every installed tool. `--force` overwrites managed files you edited (a backup is kept) |
| `shm doctor` | Check tools, managed files and the login shell; exits non-zero if something required is missing |
| `shm self-update` | Replace the binary with the latest release; then run `shm update` |

Global flags: `--plain` (no interactive UI), `--verbose`, `--quiet`.

`shm` is a link `~/.local/bin/shm → shell-manager` created by the installer
and kept by `setup`/`update`. A file of your own named `~/.local/bin/shm` is
never replaced.

## Where things live

| Path | Owner |
|---|---|
| `~/.config/zsh/{env,functions,aliases,custom}.d/*.zsh` | **You.** Auto-loaded, never touched by shell-manager |
| `~/.zshrc` | shell-manager (a one-line stub) |
| `~/.local/bin/shell-manager`, `~/.local/bin/shm` | shell-manager (binary and alias link) |
| `~/.config/shell-manager/zshrc`, `zsh.d/`, `plugins.txt` | shell-manager (generated) |
| `~/.config/starship.toml` | shell-manager |
| `~/.config/shell-manager/config.toml` | You (optional: `log_level`) |
| `~/.local/state/shell-manager/` | shell-manager (state, log) |

If you edit a file shell-manager manages, `update` leaves it alone and
`doctor` reports it as modified; `update --force` replaces it after a backup
(`<file>.bak.<timestamp>`). Put personal config in `~/.config/zsh/*.d` instead.

## Development

```bash
mise install          # Go, Task, goreleaser, golangci-lint
task test             # Go tests + bootstrap script tests
task lint             # golangci-lint, including dependency rules (depguard)
task run -- doctor    # run the CLI from source
task snapshot         # local release build into ./dist
```

Tools are modules in `internal/shellmanager/registry/tools/<id>.toml` (config
files they ship go under `registry/files/`). Which modules are installed is
decided only by the list in `registry/catalog.toml`: add or remove an id
there. A module left out stays in the codebase and is still validated by the
tests. No Go code is needed if its backend exists.

Release: `git tag vX.Y.Z && git push --tags` (GitHub Actions runs goreleaser).
