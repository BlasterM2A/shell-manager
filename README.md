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

## Commands

| Command | What it does |
|---|---|
| `shm setup` | Install what is missing, write the shell config, make zsh the login shell |
| `shm update` | Same as setup, plus update every installed tool. `--force` overwrites managed files you edited (a backup is kept) |
| `shm doctor` | Check tools, managed files, the shm blocks in `~/.zshenv`/`~/.zshrc` and the login shell; exits non-zero if something required is missing |
| `shm self-update` | Replace the binary with the latest release, then (in a terminal) offer to run `shm update` with it to apply the new configuration |

Global flags: `--plain` (no interactive UI), `--verbose`, `--quiet`.

`shm` is a link `~/.local/bin/shm → shell-manager` created by the installer
and kept by `setup`/`update`. A file of your own named `~/.local/bin/shm` is
never replaced.

## Where things live

| Path | Owner |
|---|---|
| `~/.local/bin/shell-manager`, `~/.local/bin/shm` | shm (binary and alias link) |
| `~/.shm/` | shm: `state.toml`, `shm.log`, `bootstrap.sha256`, lock, tmp; `config.toml` is yours (optional: `log_level`) |
| `~/.zshenv`, `~/.zshrc` | **You.** shm only keeps its block between `# >>> shm >>>` and `# <<< shm <<<` at the top |
| `~/.zsh/` | Your zsh config, auto-discovered (below). shm owns only the files it wrote |
| `~/.zsh_plugins.txt` | antidote plugin list (shm) |
| `~/.config/starship.toml` | starship config (shm) |

`~/.zsh/` is loaded in this order; inside each directory, files load
alphabetically (use an `NN-` prefix to order them). Add or remove a file and
open a new shell; nothing else to edit.

| Directory | What goes there | Loaded by |
|---|---|---|
| `env/*.zsh` | Environment variables and `path` (`00-path.zsh` and tool env fragments, e.g. mise shims, are shm's) | every zsh, including scripts and AI agents (`~/.zshenv`); keep it fast and silent |
| `functions/` | One function per file, file name = function name (autoloaded) | interactive shells |
| `completions/` | Completion functions (`_mycmd`) | interactive shells (on `fpath` before `compinit`) |
| `tools/*.zsh` | Tool setup: one `NN-<tool>.zsh` per tool (shm) | interactive shells |
| `aliases/*.zsh` | Aliases (`00-base.zsh` is shm's) | interactive shells |
| `local/*.zsh` | Anything else; loads last of `~/.zsh/`, so it can override the rest (lines you put in `~/.zshrc` below the shm block load after it) | interactive shells |

If you edit a file or block shm manages, `update` leaves it alone and
`doctor` reports it as modified; `update --force` replaces it after a backup
(`<file>.bak.<timestamp>`). The first time shm adds its block to an existing
`~/.zshrc` or `~/.zshenv` it saves the same kind of backup. Put your own config
in new files instead.

## Development

```bash
mise install          # Go, Task, goreleaser, golangci-lint
task test             # Go tests + bootstrap script tests
task lint             # golangci-lint, including dependency rules (depguard)
task run -- doctor    # run the CLI from source
task test:one -- Re   # tests matching a regex; task golden regenerates the zsh golden;
                      # task zsh -- 'print $PATH' runs zsh in a sandbox HOME (ZSH_FLAGS=-u|-i)
task snapshot         # local release build into ./dist
```

Tools are modules in `internal/shellmanager/registry/tools/<id>.toml` (config
files they ship go under `registry/files/`). Which modules are installed is
decided only by the list in `registry/catalog.toml`: add or remove an id
there. A module left out stays in the codebase and is still validated by the
tests. No Go code is needed if its backend exists.

Release: `git tag vX.Y.Z && git push --tags` (GitHub Actions runs goreleaser).
