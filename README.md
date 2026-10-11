# shell-setup

A CLI/TUI that bootstraps and maintains a zsh environment on Ubuntu/Debian.
It installs and keeps up to date:

- **Built-in** (always installed; shell-setup needs them): zsh (as the login
  shell) and [mise](https://mise.jdx.dev/)
- [starship](https://starship.rs/), [zoxide](https://github.com/ajeetdsouza/zoxide)
  and [fzf](https://github.com/junegunn/fzf), installed through mise
- [antidote](https://github.com/mattmc3/antidote) with zsh-autosuggestions and
  zsh-syntax-highlighting
- JetBrainsMono Nerd Font
- AI CLIs (optional, a failure is only a warning): Claude Code, Junie,
  Antigravity CLI (`agy`)

## Install on a new machine

```bash
curl -fsSL https://raw.githubusercontent.com/BlasterM2A/shell-setup/master/install.sh | bash
```

This downloads the latest release binary to `~/.local/bin/shell-setup`
(checksum verified) and runs `shell-setup init`. Expect a **sudo** prompt
(apt packages) and possibly a **chsh** password prompt (login shell).

After the first `init`:

1. Run `exec zsh`, or open a new terminal. If a new terminal still opens
   your old shell, log out and back in (some terminals only pick up a new
   login shell in a new session).
2. Close **all** terminal windows and reopen the terminal (a running terminal
   keeps the font list it loaded at start), then select **JetBrainsMono Nerd
   Font** in its font settings.

## Migrating from the old install.sh

Run the one-liner above once. Then:

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
  `shs doctor` lists the leftover and `shs update` retries it.

## Commands

| Command | What it does |
|---|---|
| `shell-setup init` | Install what is missing, write the shell config, make zsh the login shell |
| `shell-setup update` | Same as init, plus update every installed tool. `--force` overwrites managed files you edited (a backup is kept) |
| `shell-setup doctor` | Check tools, managed files and the login shell; exits non-zero if something required is missing |
| `shell-setup self-update` | Replace the binary with the latest release; then run `shell-setup update` |

Global flags: `--plain` (no interactive UI), `--verbose`, `--quiet`.

`shs` is a short alias for `shell-setup` (`shs update`, `shs doctor`…): a
link `~/.local/bin/shs → shell-setup` created by the installer and kept by
`init`/`update`. A file of your own named `~/.local/bin/shs` is never replaced.

## Where things live

| Path | Owner |
|---|---|
| `~/.config/zsh/{env,functions,aliases,custom}.d/*.zsh` | **You.** Auto-loaded, never touched by shell-setup |
| `~/.zshrc` | shell-setup (a one-line stub) |
| `~/.local/bin/shell-setup`, `~/.local/bin/shs` | shell-setup (binary and alias link) |
| `~/.config/shell-setup/zshrc`, `zsh.d/`, `plugins.txt` | shell-setup (generated) |
| `~/.config/starship.toml` | shell-setup |
| `~/.config/shell-setup/config.toml` | You (optional: `log_level`) |
| `~/.local/state/shell-setup/` | shell-setup (state, log) |

If you edit a file shell-setup manages, `update` leaves it alone and `doctor`
reports it as modified; `update --force` replaces it after a backup
(`<file>.bak.<timestamp>`). Put personal config in `~/.config/zsh/*.d` instead.

## Development

```bash
mise install          # Go, Task, goreleaser, golangci-lint
task test             # Go tests + bootstrap script tests
task lint             # golangci-lint, including dependency rules (depguard)
task run -- doctor    # run the CLI from source
task snapshot         # local release build into ./dist
```

Tools are modules in `internal/shellsetup/registry/tools/<id>.toml` (config
files they ship go under `registry/files/`). Which modules are installed is
decided only by the list in `registry/catalog.toml`: add or remove an id
there. A module left out stays in the codebase and is still validated by the
tests. No Go code is needed if its backend exists.

Release: `git tag vX.Y.Z && git push --tags` (GitHub Actions runs goreleaser).
