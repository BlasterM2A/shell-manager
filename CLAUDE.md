# shell-manager

Go CLI/TUI (`shell-manager`, alias `shm`) that bootstraps and maintains a zsh
environment on Ubuntu/Debian (zsh, mise, starship, zoxide, fzf, antidote,
Nerd Font, AI CLIs). Distributed as a GitHub Release binary; `install.sh`
downloads it, links `shm` and asks whether to run `shm setup`.
See `README.md` for usage. Design: base spec
`docs/superpowers/specs/2026-10-10-shell-setup-cli-design.md` (written under
the old name), `docs/superpowers/specs/2026-10-11-shell-manager-redesign-design.md`
(names, the `setup` command, the installer prompt) and
`docs/superpowers/specs/2026-10-11-shm-layout-design.md`, which overrides
paths (`~/.shm`, `~/.zsh/`, blocks in `~/.zshenv`/`~/.zshrc`) and removes all
migration.

## Repo layout

- `main.go` → `internal/cmd` (cobra). `cmd` is the only integration layer: it
  reads flags, builds UI components and binds domain events to them. No
  business logic, no styles.
- `internal/shellmanager` — the whole domain in one package (chezmoi-style):
  `Engine` (`Setup`, `Update`, `Doctor`), tool manifests + catalog, backends
  (`apt`, `mise`, `script`, `archive`, `font`), zsh config composition,
  managed files + state, shm blocks in user dotfiles. All I/O
  goes through the `System` interface.
- `internal/shellmanager/registry/` (embedded) — `tools/<id>.toml` is one
  module (manifest) per tool; `catalog.toml` lists the ids that are
  installed — the only place that decides inclusion. `files/` holds the
  config files modules ship and shm's zsh files (`files/zsh/`).
- `internal/ui` — components (`steplist`, `statustable`, `notice`, `model`).
  `internal/ui/styles` is the ONLY place that defines colors/icons.
- `internal/iostreams`, `internal/log`, `internal/config`, `internal/selfupdate`.
- `install.sh` + `tests/test_install.sh` — bootstrap and its tests.

## Conventions

- Dependency rules are enforced by depguard (`.golangci.yml`): `ui`,
  `iostreams` and `log` never import `shellmanager`/`cmd`; `shellmanager`,
  `selfupdate` and `config` never import `ui`/`iostreams`/`cmd`/`charm.land`.
- Every operation is idempotent. Managed files are never overwritten if the
  user edited them unless `--force`; anything replaced is backed up.
- The domain never prints: it sends `Event`s on a channel and returns a
  `Report`.
- Never run `shm setup`/`update` against your own `$HOME` while developing
  (installs packages, chsh). Verify with `task test` and `task lint`; manual
  runs only `doctor`/`--version` with `HOME=$(mktemp -d)`.
- No Docker- or shellcheck-based testing (explicit scope decision).
- The GitHub slug `BlasterM2A/shell-manager` lives in three places: the
  goreleaser ldflags (`.goreleaser.yml`), `REPO` in `install.sh`, and the
  `repoSlug` default in `internal/cmd/root.go`; update all three if the repo
  moves.

## Making changes

Design changes: update the spec first, then the code. Adding a tool is a new
module in `registry/tools/` plus its id in `registry/catalog.toml` (and a
backend in Go only if none fits); removing one is deleting its id from
`catalog.toml`. Tests never name or count tools: they validate whatever
`catalog.toml` lists, and that every module in `tools/` parses.
