# shm layout (`~/.shm`, `~/.zsh`, dotfile blocks) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move shm's own data to `~/.shm`, generate the zsh config under `~/.zsh/` loaded by minimal marker blocks in `~/.zshenv`/`~/.zshrc`, and delete all migration code.

**Architecture:** `Paths` gets the new locations. `fileWriter` (managed.go) learns `writeBlock`, which owns only the text between `# >>> shm >>>` and `# <<< shm <<<`, with the same never-overwrite-edits policy as whole files (hash in `State.Blocks`). `shellWriter` (shellrc.go) composes the two blocks, shm's own files in `~/.zsh/` (embedded under `registry/files/zsh/`) and one `env/` and/or `tools/` fragment per module (`ShellSpec.Env` / `ShellSpec.Snippet`). Doctor checks blocks like files.

**Tech Stack:** Go 1.x, testify, `charmbracelet/x/exp/golden` (golden files, `-update`), go-toml v2, bash (`install.sh`), zsh.

**Spec:** `docs/superpowers/specs/2026-10-11-shm-layout-design.md`

## Global Constraints

- shm's own data: `~/.shm/` flat: `config.toml`, `state.toml`, `shm.log`, `lock`, `bootstrap.sha256`, `tmp/`.
- Binary unchanged: `~/.local/bin/shell-manager` + link `~/.local/bin/shm`.
- Block markers, exactly: `# >>> shm >>>` and `# <<< shm <<<`; the block is inserted at the TOP of the file.
- `~/.zsh/` dirs: `env`, `functions`, `completions`, `tools`, `aliases`, `local` — all created by setup.
- antidote plugins at `~/.zsh_plugins.txt`; starship at `~/.config/starship.toml`.
- No migration of any kind: no code that reads `~/.config/shell-setup`, `~/.config/shell-manager`, `~/.local/state/*` or `~/.config/zsh`.
- `install.sh`: `SHM_DIR` (default `~/.shm`) replaces `SHELL_MANAGER_STATE_DIR`.
- Never run `shm setup`/`update` against the real `$HOME`; manual runs only with `HOME=$(mktemp -d)`. Verify with `task test` and `task lint`.
- Dependency rules (depguard): `shellmanager` and `config` never import `ui`/`iostreams`/`cmd`/`charm.land`.
- Tests never name or count catalog tools except through `catalog.toml`-driven helpers already used in `shellrc_test.go` (which pick real modules by id — keep that pattern).
- Zsh startup must print nothing.

## Review Focus

1. `~/.zshrc` is a symlink (stow/chezmoi/yadm dotfiles) → shm edits the link's target and the link stays a link. Test in Task 3.
2. Marker lines with CRLF endings or trailing spaces → still recognised as the shm block (not "broken", not duplicated). Test in Task 3.
3. An existing `~/.zshrc` without a trailing newline, or empty → original content preserved byte for byte below the block; an empty file gets no backup. Test in Task 3.
4. The user deleted the shm block after a setup (state still has its hash) → setup re-inserts it at the top with a backup, and doctor reports it `missing`. Tests in Task 3 and Task 6.
5. Non-interactive `zsh -c` (agents, scripts) gets `~/.local/bin` and mise shims on PATH, and still runs silently if `~/.zsh` was deleted. Test in Task 5.

---

### Task 1: Remove migration and legacy purge

**Files:**
- Delete: `internal/shellmanager/migrate.go`, `internal/shellmanager/migrate_test.go`, `internal/shellmanager/legacy.go`, `internal/shellmanager/legacy_test.go`
- Modify: `internal/shellmanager/shellmanager.go` (apply), `internal/shellmanager/doctor.go`, `internal/shellmanager/tool.go`, `internal/shellmanager/catalog.go` (loadTool), `internal/shellmanager/catalog_test.go` (case "empty after_purge command"), `internal/shellmanager/event.go`, `internal/cmd/bindings.go:50`, `internal/shellmanager/registry/tools/{fzf,nerdfont,starship,zoxide}.toml`

**Interfaces:**
- Consumes: nothing.
- Produces: `Tool` without `Legacy`; `Result` constants without `ResultRemoved`/`ResultMigrated`; `Paths.ConfigDir`/`StateDir` no longer used outside `paths.go`/`shellrc.go`.

- [ ] **Step 1: Delete the files**

```bash
git rm internal/shellmanager/migrate.go internal/shellmanager/migrate_test.go \
       internal/shellmanager/legacy.go internal/shellmanager/legacy_test.go
```

- [ ] **Step 2: Remove the calls in `Engine.apply` (`shellmanager.go`)**

Delete this block (right after `checkOS`):

```go
	migrated, err := e.migrateFromShellSetup()
	for _, r := range migrated {
		events <- FileFinished(r)
	}
	rep.Files = append(rep.Files, migrated...)
	if err != nil {
		return rep, fmt.Errorf("migrating from shell-setup: %w", err)
	}
```

Delete the purge loop after the tools loop:

```go
	for _, t := range okTools {
		for _, r := range e.purgeLegacy(ctx, t) {
			events <- FileFinished(r)
			rep.Files = append(rep.Files, r)
		}
	}
```

Delete the compat link removal after `w.apply`:

```go
	if err := e.removeCompatLink(); err != nil {
		return rep, fmt.Errorf("removing the shell-setup compatibility link: %w", err)
	}
```

- [ ] **Step 3: Remove leftover checks in `doctor.go`**

Delete the two loops at the end of `Doctor` (the `legacyLeftovers` loop and the `oldInstallLeftovers` block) and drop the now-unused `"strings"` import. `Doctor` ends with `rep.Checks = append(rep.Checks, sc, e.aliasCheck())` then `return rep, nil`.

- [ ] **Step 4: Remove `Legacy` from the manifest model**

`tool.go`: delete the `Legacy *LegacySpec \`toml:"legacy"\`` field with its comment, and the whole `LegacySpec` type. Move the `// FileSpec is a config file the tool ships, embedded under registry/.` comment back onto `type FileSpec struct`.

`catalog.go` (`loadTool`): delete the `if t.Legacy != nil { ... }` block.

`catalog_test.go`: delete the `"empty after_purge command": {...},` case.

`event.go`: delete `ResultRemoved` and `ResultMigrated`.

`internal/cmd/bindings.go:50`: `case shellmanager.ResultOK:`.

- [ ] **Step 5: Remove `[legacy]` from modules**

In `registry/tools/fzf.toml`, `starship.toml`, `zoxide.toml` and `nerdfont.toml` delete the `[legacy]` table and the comment lines directly above it (e.g. `# The old install.sh put starship here; purged once mise provides it.`). Unknown fields are rejected by the decoder, so any leftover `[legacy]` makes `TestEmbeddedModulesAreValid` fail.

- [ ] **Step 6: Build, test, lint**

Run: `go build ./... && go test ./... && golangci-lint run`
Expected: build OK, all tests PASS, no lint findings (lint catches any now-unused helper; delete it if reported).

- [ ] **Step 7: Commit**

```bash
git add -A internal/
git commit -m "Remove the shell-setup migration and legacy purge"
```

---

### Task 2: shm's own data in `~/.shm` (Go + install.sh)

**Files:**
- Modify: `internal/shellmanager/paths.go`, `internal/shellmanager/system_test.go` (`TestPaths`), `internal/config/config.go` (`DefaultPath`), `internal/config/config_test.go` (`TestDefaultPath`), `install.sh`, `tests/test_install.sh`

**Interfaces:**
- Consumes: Task 1 (nothing uses `StateDir` anymore).
- Produces: `Paths.ShmDir` (`~/.shm`); `StateFile`, `LockFile`, `LogFile` (`~/.shm/shm.log`), `TmpDir` under it. `Paths.StateDir` is removed.

- [ ] **Step 1: Update the tests first**

`system_test.go`, in `TestPaths` replace the `StateFile` assertion and add the others:

```go
	assert.Equal(t, "/h/.shm", p.ShmDir)
	assert.Equal(t, "/h/.shm/state.toml", p.StateFile)
	assert.Equal(t, "/h/.shm/lock", p.LockFile)
	assert.Equal(t, "/h/.shm/shm.log", p.LogFile)
	assert.Equal(t, "/h/.shm/tmp", p.TmpDir)
```

`config_test.go`:

```go
	assert.Equal(t, "/h/.shm/config.toml", config.DefaultPath("/h"))
```

`tests/test_install.sh`: replace `export SHELL_MANAGER_STATE_DIR="$TMP/state"` with `export SHM_DIR="$TMP/shm"`, replace `$SHELL_MANAGER_STATE_DIR/bootstrap.sha256` (near the end) with `$SHM_DIR/bootstrap.sha256`, and right after the `"installed binary runs"` assertion add:

```bash
assert_true "the stamp is written to SHM_DIR" test -f "$SHM_DIR/bootstrap.sha256"
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/shellmanager/ ./internal/config/ -run 'TestPaths|TestDefaultPath'; bash tests/test_install.sh`
Expected: FAIL (`ShmDir` undefined; old config path; stamp not in `$SHM_DIR`).

- [ ] **Step 3: Implement**

`paths.go` — in `Paths` replace `StateDir string // ~/.local/state/shell-manager` with `ShmDir string // ~/.shm: shm's own config, state, log, lock, tmp`, and in `NewPaths`:

```go
	shm := filepath.Join(home, ".shm")
	// ...
		ShmDir:         shm,
		StateFile:      filepath.Join(shm, "state.toml"),
		LockFile:       filepath.Join(shm, "lock"),
		LogFile:        filepath.Join(shm, "shm.log"),
		TmpDir:         filepath.Join(shm, "tmp"),
```

(remove the `state := ...` variable).

`config.go`:

```go
// DefaultPath is where the config file lives.
func DefaultPath(home string) string {
	return filepath.Join(home, ".shm", "config.toml")
}
```

`install.sh`:

```bash
SHM_DIR="${SHM_DIR:-$HOME/.shm}"
...
STAMP_FILE="$SHM_DIR/bootstrap.sha256"
...
    mkdir -p "$BIN_DIR" "$SHM_DIR"
```

(delete the `STATE_DIR=` line; `STATE_DIR` has no other uses.)

- [ ] **Step 4: Run tests**

Run: `go test ./... && bash tests/test_install.sh`
Expected: PASS, `All tests passed`.

- [ ] **Step 5: Commit**

```bash
git add internal/shellmanager/paths.go internal/shellmanager/system_test.go internal/config install.sh tests/test_install.sh
git commit -m "Keep shm's own config, state and log in ~/.shm"
```

---

### Task 3: Managed blocks (`writeBlock`)

**Files:**
- Create: `internal/shellmanager/block.go`, `internal/shellmanager/block_test.go`
- Modify: `internal/shellmanager/state.go`, `internal/shellmanager/managed.go` (extract `backupPath`), `internal/shellmanager/managed_test.go` and `internal/shellmanager/shellrc_test.go` (State literals get `Blocks`)

**Interfaces:**
- Consumes: `fileWriter{sys, state, now}`, `hashBytes`, `Result*`.
- Produces:
  - `State.Blocks map[string]string` (toml `blocks`), always non-nil after `LoadState`.
  - `const blockBegin = "# >>> shm >>>"`, `blockEnd = "# <<< shm <<<"`.
  - `var ErrBrokenMarkers error`.
  - `func findBlock(content []byte) (start, end int, found bool, err error)`.
  - `func (w fileWriter) writeBlock(path string, block []byte, force bool) (Result, error)` — `block` includes both marker lines and ends with `\n`.
  - `func (w fileWriter) backupPath(path string) string`.
  - `func resolveLink(sys System, path string) string` — follows symlinks to the final target (used by doctor in Task 6).

- [ ] **Step 1: Add `Blocks` to `State`**

`state.go`:

```go
	// Files maps each managed file to the hash of what shell-manager wrote.
	Files map[string]string `toml:"files"`
	// Blocks maps each file holding an shm block to the hash of that block.
	Blocks map[string]string `toml:"blocks"`
```

and in `LoadState`, next to the other nil checks:

```go
	if st.Blocks == nil {
		st.Blocks = map[string]string{}
	}
```

Update the two test literals (`managed_test.go` `newWriter`, `shellrc_test.go` `newShellWriter`) to `&State{Files: map[string]string{}, Blocks: map[string]string{}, Versions: map[string]string{}}`.

- [ ] **Step 2: Write the failing tests** (`block_test.go`)

```go
package shellmanager

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testBlock = "# >>> shm >>>\nsource x\n# <<< shm <<<\n"

func TestFindBlock(t *testing.T) {
	cases := map[string]struct {
		in         string
		start, end int // end < 0 means len(in)
		found      bool
		broken     bool
	}{
		"none":              {in: "a\nb\n"},
		"top":               {in: testBlock + "rest\n", start: 0, end: len(testBlock), found: true},
		"middle":            {in: "a\n" + testBlock + "b\n", start: 2, end: 2 + len(testBlock), found: true},
		"no final newline":  {in: "# >>> shm >>>\nx\n# <<< shm <<<", start: 0, end: -1, found: true},
		"crlf and spaces":   {in: "# >>> shm >>>  \r\nx\r\n# <<< shm <<<\r\n", start: 0, end: -1, found: true},
		"only begin":        {in: "# >>> shm >>>\nx\n", broken: true},
		"only end":          {in: "x\n# <<< shm <<<\n", broken: true},
		"end before begin":  {in: "# <<< shm <<<\n# >>> shm >>>\n", broken: true},
		"duplicated":        {in: testBlock + testBlock, broken: true},
		"indented is plain": {in: "  # >>> shm >>>\n"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			start, end, found, err := findBlock([]byte(tc.in))
			if tc.broken {
				assert.ErrorIs(t, err, ErrBrokenMarkers)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.found, found)
			if tc.end < 0 {
				tc.end = len(tc.in)
			}
			assert.Equal(t, tc.start, start)
			assert.Equal(t, tc.end, end)
		})
	}
}

func TestWriteBlockCreatesMissingFile(t *testing.T) {
	w, path := newWriter(t)
	res, err := w.writeBlock(path, []byte(testBlock), false)
	require.NoError(t, err)
	assert.Equal(t, ResultOK, res)
	assert.Equal(t, testBlock, read(t, path))
	assert.Equal(t, hashBytes([]byte(testBlock)), w.state.Blocks[path])
	assert.NotContains(t, w.state.Files, path)
}

func TestWriteBlockInsertsAtTopAndKeepsContent(t *testing.T) {
	w, path := newWriter(t)
	require.NoError(t, os.WriteFile(path, []byte("export FOO=1"), 0o644)) // no final newline

	res, err := w.writeBlock(path, []byte(testBlock), false)
	require.NoError(t, err)
	assert.Equal(t, ResultOK, res)
	assert.Equal(t, testBlock+"\nexport FOO=1", read(t, path))
	assert.Equal(t, "export FOO=1", read(t, path+backupSuffix))
}

func TestWriteBlockEmptyFileGetsNoBackup(t *testing.T) {
	w, path := newWriter(t)
	require.NoError(t, os.WriteFile(path, nil, 0o644))

	_, err := w.writeBlock(path, []byte(testBlock), false)
	require.NoError(t, err)
	assert.Equal(t, testBlock, read(t, path))
	assert.NoFileExists(t, path+backupSuffix)
}

func TestWriteBlockUnchangedIsNoop(t *testing.T) {
	w, path := newWriter(t)
	_, err := w.writeBlock(path, []byte(testBlock), false)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte(testBlock+"# mine\n"), 0o644)) // outside the block

	res, err := w.writeBlock(path, []byte(testBlock), false)
	require.NoError(t, err)
	assert.Equal(t, ResultOK, res)
	assert.Equal(t, testBlock+"# mine\n", read(t, path))
	assert.NoFileExists(t, path+backupSuffix)
}

func TestWriteBlockUpdatesTemplateChangeInPlace(t *testing.T) {
	w, path := newWriter(t)
	_, err := w.writeBlock(path, []byte(testBlock), false)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte("# above\n"+testBlock+"# below\n"), 0o644))
	v2 := "# >>> shm >>>\nsource y\n# <<< shm <<<\n"

	res, err := w.writeBlock(path, []byte(v2), false)
	require.NoError(t, err)
	assert.Equal(t, ResultOK, res)
	assert.Equal(t, "# above\n"+v2+"# below\n", read(t, path))
	assert.NoFileExists(t, path+backupSuffix)
	assert.Equal(t, hashBytes([]byte(v2)), w.state.Blocks[path])
}

func TestWriteBlockEditedIsLeftAlone(t *testing.T) {
	w, path := newWriter(t)
	_, err := w.writeBlock(path, []byte(testBlock), false)
	require.NoError(t, err)
	edited := "# >>> shm >>>\nsource x\nexport MINE=1\n# <<< shm <<<\n"
	require.NoError(t, os.WriteFile(path, []byte(edited), 0o644))

	res, err := w.writeBlock(path, []byte(testBlock), false)
	require.NoError(t, err)
	assert.Equal(t, ResultModified, res)
	assert.Equal(t, edited, read(t, path))
}

func TestWriteBlockEditedWithForceBacksUp(t *testing.T) {
	w, path := newWriter(t)
	_, err := w.writeBlock(path, []byte(testBlock), false)
	require.NoError(t, err)
	edited := "# >>> shm >>>\nexport MINE=1\n# <<< shm <<<\n# tail\n"
	require.NoError(t, os.WriteFile(path, []byte(edited), 0o644))

	res, err := w.writeBlock(path, []byte(testBlock), true)
	require.NoError(t, err)
	assert.Equal(t, ResultOK, res)
	assert.Equal(t, testBlock+"# tail\n", read(t, path))
	assert.Equal(t, edited, read(t, path+backupSuffix))
}

func TestWriteBlockReinsertsDeletedBlock(t *testing.T) {
	w, path := newWriter(t)
	_, err := w.writeBlock(path, []byte(testBlock), false)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte("export FOO=1\n"), 0o644)) // block deleted by hand

	res, err := w.writeBlock(path, []byte(testBlock), false)
	require.NoError(t, err)
	assert.Equal(t, ResultOK, res)
	assert.Equal(t, testBlock+"\nexport FOO=1\n", read(t, path))
	assert.Equal(t, "export FOO=1\n", read(t, path+backupSuffix))
}

func TestWriteBlockBrokenMarkersLeaveFileAlone(t *testing.T) {
	w, path := newWriter(t)
	broken := "# >>> shm >>>\nsource x\n"
	require.NoError(t, os.WriteFile(path, []byte(broken), 0o644))

	res, err := w.writeBlock(path, []byte(testBlock), false)
	assert.ErrorIs(t, err, ErrBrokenMarkers)
	assert.ErrorContains(t, err, path)
	assert.Equal(t, ResultFailed, res)
	assert.Equal(t, broken, read(t, path))
}

func TestWriteBlockWritesThroughSymlink(t *testing.T) {
	w, path := newWriter(t)
	target := filepath.Join(filepath.Dir(path), "dotfiles", "zshrc")
	require.NoError(t, os.MkdirAll(filepath.Dir(target), 0o755))
	require.NoError(t, os.WriteFile(target, []byte("# from dotfiles\n"), 0o644))
	require.NoError(t, os.Symlink("dotfiles/zshrc", path)) // relative, like stow

	_, err := w.writeBlock(path, []byte(testBlock), false)
	require.NoError(t, err)

	info, err := os.Lstat(path)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&os.ModeSymlink, "the link stays a link")
	assert.Equal(t, testBlock+"\n# from dotfiles\n", read(t, target))
	assert.Equal(t, hashBytes([]byte(testBlock)), w.state.Blocks[path], "state is keyed by the link path")
}
```

- [ ] **Step 3: Run them to see them fail**

Run: `go test ./internal/shellmanager/ -run 'TestFindBlock|TestWriteBlock'`
Expected: FAIL to compile (`findBlock`, `writeBlock`, `ErrBrokenMarkers` undefined).

- [ ] **Step 4: Implement**

`managed.go`: add the helper and use it in `write` (replace the inline `path+".bak."+w.now().Format("20060102150405")`):

```go
// backupPath is where a file is saved before shell-manager replaces it.
func (w fileWriter) backupPath(path string) string {
	return path + ".bak." + w.now().Format("20060102150405")
}
```

`block.go`:

```go
package shellmanager

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
)

// An shm block is the part of a user's file (~/.zshrc, ~/.zshenv) that shm
// owns: the lines from blockBegin to blockEnd. The rest of the file is the
// user's (or other installers') and is never touched.
const (
	blockBegin = "# >>> shm >>>"
	blockEnd   = "# <<< shm <<<"
)

// ErrBrokenMarkers means a file has an incomplete or repeated shm block.
var ErrBrokenMarkers = errors.New("broken shm block markers (fix them by hand)")

// findBlock returns the byte range of the shm block in content, from the
// start of the begin marker line to the end of the end marker line
// (its newline included). Marker lines may end in spaces or \r. found is
// false when neither marker is present.
func findBlock(content []byte) (start, end int, found bool, err error) {
	var begins, ends []int
	for off := 0; off < len(content); {
		next := len(content)
		if i := bytes.IndexByte(content[off:], '\n'); i >= 0 {
			next = off + i + 1
		}
		switch string(bytes.TrimRight(content[off:next], " \t\r\n")) {
		case blockBegin:
			begins = append(begins, off)
		case blockEnd:
			ends = append(ends, next)
		}
		off = next
	}
	switch {
	case len(begins) == 0 && len(ends) == 0:
		return 0, 0, false, nil
	case len(begins) == 1 && len(ends) == 1 && begins[0] < ends[0]:
		return begins[0], ends[0], true, nil
	default:
		return 0, 0, false, ErrBrokenMarkers
	}
}

// writeBlock keeps block (both marker lines included) in path: inserted at
// the top when missing, replaced in place when shm wrote the current one.
// A block edited by hand is left alone unless forced. Anything replaced that
// shm did not write is backed up first. Symlinks are followed, so dotfile
// managers keep their link; state is keyed by path.
func (w fileWriter) writeBlock(path string, block []byte, force bool) (Result, error) {
	target := resolveLink(w.sys, path)
	current, err := w.sys.ReadFile(target)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return ResultFailed, err
	}
	start, end, found, err := findBlock(current)
	if err != nil {
		return ResultFailed, fmt.Errorf("%s: %w", path, err)
	}
	var next []byte
	switch {
	case found:
		recorded, managed := w.state.Blocks[path]
		edited := managed && hashBytes(current[start:end]) != recorded
		if edited && !force {
			return ResultModified, nil
		}
		if bytes.Equal(current[start:end], block) {
			w.state.Blocks[path] = hashBytes(block)
			return ResultOK, nil
		}
		if !managed || edited {
			if err := w.sys.WriteFile(w.backupPath(path), current, 0o644); err != nil {
				return ResultFailed, err
			}
		}
		next = slices.Concat(current[:start], block, current[end:])
	case len(bytes.TrimSpace(current)) > 0:
		if err := w.sys.WriteFile(w.backupPath(path), current, 0o644); err != nil {
			return ResultFailed, err
		}
		next = slices.Concat(block, []byte("\n"), current)
	default:
		next = block
	}
	if err := w.sys.WriteFile(target, next, 0o644); err != nil {
		return ResultFailed, err
	}
	w.state.Blocks[path] = hashBytes(block)
	return ResultOK, nil
}

// resolveLink follows path's symlinks (at most 40, like the kernel) and
// returns the final target; path itself if it is not a link or is broken.
func resolveLink(sys System, path string) string {
	for range 40 {
		info, err := sys.Lstat(path)
		if err != nil || info.Mode()&fs.ModeSymlink == 0 {
			return path
		}
		dest, err := sys.Readlink(path)
		if err != nil {
			return path
		}
		if !filepath.IsAbs(dest) {
			dest = filepath.Join(filepath.Dir(path), dest)
		}
		path = dest
	}
	return path
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/shellmanager/`
Expected: PASS (all, including the existing `managed_test.go`).

- [ ] **Step 6: Commit**

```bash
git add internal/shellmanager/block.go internal/shellmanager/block_test.go internal/shellmanager/state.go internal/shellmanager/managed.go internal/shellmanager/managed_test.go internal/shellmanager/shellrc_test.go
git commit -m "Manage an shm block inside user files"
```

---

### Task 4: `ShellSpec.Env`

**Files:**
- Modify: `internal/shellmanager/tool.go` (`ShellSpec`), `internal/shellmanager/catalog.go` (`loadTool`), `internal/shellmanager/catalog_test.go` (invalid-module table + a valid case)

**Interfaces:**
- Consumes: nothing new.
- Produces: `ShellSpec{Priority int; Env string; Snippet string}`; a `[shell]` table is valid when `1 <= priority <= 89` and at least one of `env`/`snippet` is non-empty.

- [ ] **Step 1: Write the failing tests**

In `catalog_test.go`, add to the invalid-module `cases` map:

```go
		"shell without env or snippet": {
			testRegistry([]string{"a"}, map[string]string{"a": module("a", "plugin", "") + "[shell]\npriority = 10\n"}),
			"shell needs a priority in 1..89 and an env or a snippet",
		},
```

and a new test:

```go
func TestShellEnvOnlyIsValid(t *testing.T) {
	cat, err := LoadCatalog(testRegistry([]string{"a"}, map[string]string{
		"a": module("a", "plugin", "") + "[shell]\npriority = 10\nenv = 'export A=1'\n",
	}))
	require.NoError(t, err)
	a, _ := cat.Get("a")
	assert.Equal(t, "export A=1", a.Shell.Env)
}
```

If an existing case asserts the old message (`"shell needs a priority in 1..89 and a snippet"`), update its expected text to the new one.

- [ ] **Step 2: Run to see them fail**

Run: `go test ./internal/shellmanager/ -run 'TestLoadCatalog|TestShellEnvOnlyIsValid'`
Expected: FAIL (`env` is an unknown field).

- [ ] **Step 3: Implement**

`tool.go`:

```go
// ShellSpec is the tool's part of the zsh config: Env goes to
// ~/.zsh/env/NN-<id>.zsh (every zsh, interactive or not; must print
// nothing) and Snippet to ~/.zsh/tools/NN-<id>.zsh (interactive only).
type ShellSpec struct {
	Priority int    `toml:"priority"` // 1..89, the NN load-order prefix
	Env      string `toml:"env"`
	Snippet  string `toml:"snippet"`
}
```

`catalog.go`:

```go
	if t.Shell != nil && (t.Shell.Priority < 1 || t.Shell.Priority > 89 || (t.Shell.Env == "" && t.Shell.Snippet == "")) {
		return Tool{}, errors.New("shell needs a priority in 1..89 and an env or a snippet")
	}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/shellmanager/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/shellmanager/tool.go internal/shellmanager/catalog.go internal/shellmanager/catalog_test.go
git commit -m "Let modules ship an env fragment for every zsh"
```

---

### Task 5: Compose the zsh config under `~/.zsh/`

**Files:**
- Create: `internal/shellmanager/registry/files/zsh/{zshenv-block,zshrc-block,env.zsh,init.zsh,env/00-path.zsh,tools/20-core.zsh,aliases/00-base.zsh}`
- Delete: `internal/shellmanager/registry/files/profile/` (whole dir)
- Modify: `internal/shellmanager/paths.go`, `internal/shellmanager/shellrc.go`, `internal/shellmanager/registry/tools/{mise,antidote,starship,fzf,zoxide}.toml`
- Modify tests: `internal/shellmanager/shellrc_test.go` (rewrite), `internal/shellmanager/testdata/TestShellConfigGolden.golden` (regenerated), `internal/shellmanager/helpers_test.go` (`runZsh`), `internal/shellmanager/engine_test.go` (`newTestEngine`, `Paths.Stub`), `internal/shellmanager/doctor_test.go` (`TestDoctorFlagsModifiedAndMissingManagedFiles`), `internal/shellmanager/catalog_test.go` (`TestEmbeddedCatalogIsValid`), `internal/shellmanager/system_test.go` (`TestPaths`)

**Interfaces:**
- Consumes: `fileWriter.writeBlock`, `ErrBrokenMarkers` (Task 3); `ShellSpec.Env` (Task 4); `Paths.ShmDir` etc. (Task 2).
- Produces: `Paths.Zshenv` (`~/.zshenv`), `Paths.Zshrc` (`~/.zshrc`), `Paths.ZshDir` (`~/.zsh`). `Paths.ConfigDir`, `ZshD`, `GeneratedZshrc`, `Stub`, `UserZshDir` are removed. `shellWriter.apply(okTools []Tool, force bool) ([]FileReport, error)` keeps its signature.

- [ ] **Step 1: Create the templates**

`registry/files/zsh/zshenv-block`:

```zsh
# >>> shm >>>
# Managed by shm; changes inside this block are overwritten. Config: ~/.zsh/
skip_global_compinit=1
[[ -r "$HOME/.zsh/env.zsh" ]] && source "$HOME/.zsh/env.zsh"
# <<< shm <<<
```

`registry/files/zsh/zshrc-block`:

```zsh
# >>> shm >>>
# Managed by shm; changes inside this block are overwritten. Config: ~/.zsh/
[[ -r "$HOME/.zsh/init.zsh" ]] && source "$HOME/.zsh/init.zsh"
# <<< shm <<<
```

`registry/files/zsh/env.zsh`:

```zsh
# Managed by shm; changes here are overwritten by `shm update`.
# Your environment goes in ~/.zsh/env/*.zsh (loaded by every zsh, keep it quiet).
[[ -n $_SHM_ENV_LOADED ]] && return
_SHM_ENV_LOADED=1
typeset -U path
for _shm_f in "$HOME"/.zsh/env/*.zsh(N); do source "$_shm_f"; done
unset _shm_f
```

Files are sourced at top level, not inside a function: inside one, a plain
`typeset`/`declare` in a user file or plugin would become local and vanish.

`registry/files/zsh/init.zsh`:

```zsh
# Managed by shm; changes here are overwritten by `shm update`.
# Your config goes in ~/.zsh/{env,functions,completions,aliases,local}/
[[ -o interactive ]] || return
source "$HOME/.zsh/env.zsh"
fpath=("$HOME/.zsh/completions" "$HOME/.zsh/functions" $fpath)
() { (( $# )) && autoload -Uz "$@" } "$HOME"/.zsh/functions/*(N:t)
for _shm_f in "$HOME"/.zsh/{tools,aliases,local}/*.zsh(N); do source "$_shm_f"; done
unset _shm_f
```

(Brace expansion runs before globbing, so the order is tools/*, then
aliases/*, then local/*, each sorted.)

`registry/files/zsh/env/00-path.zsh`:

```zsh
# Managed by shm (profile: path)
path=("$HOME/.local/bin" $path)
```

`registry/files/zsh/tools/20-core.zsh`:

```zsh
# Managed by shm (profile: core)
autoload -Uz compinit && compinit

HISTFILE="$HOME/.zsh_history"
HISTSIZE=10000
SAVEHIST=10000
setopt SHARE_HISTORY
setopt HIST_IGNORE_DUPS
```

`registry/files/zsh/aliases/00-base.zsh`:

```zsh
# Managed by shm (profile: aliases)
alias ll='ls -alF'
alias la='ls -A'
alias l='ls -CF'
```

Then `git rm -r internal/shellmanager/registry/files/profile`.

- [ ] **Step 2: Update the modules**

`mise.toml` `[shell]`:

```toml
[shell]
priority = 10
# Shims let non-interactive shells (scripts, AI agents) find mise tools;
# interactive shells also run `mise activate`.
env = 'path=("${XDG_DATA_HOME:-$HOME/.local/share}/mise/shims" $path)'
snippet = 'eval "$(mise activate zsh)"'
```

`antidote.toml`: snippet and file:

```toml
[shell]
priority = 30
snippet = '''
source "$HOME/.antidote/antidote.zsh"
antidote load
'''

[[files]]
src = "files/antidote/plugins.txt"
dest = "~/.zsh_plugins.txt"
```

`starship.toml`: `snippet = '[[ $TERM == dumb ]] || eval "$(starship init zsh)"'`

`fzf.toml`: `snippet = '[[ -t 0 && $TERM != dumb ]] && fzf --zsh >/dev/null 2>&1 && source <(fzf --zsh)'`

`zoxide.toml`: `snippet = 'eval "$(zoxide init zsh)"'` (single line; the `alias cd="z"` line is gone — it breaks `cd` inside Claude Code's shell snapshot).

- [ ] **Step 3: Rewrite `shellrc_test.go` (failing tests)**

Replace the file with:

```go
package shellmanager

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/exp/golden"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newShellWriter(t *testing.T) shellWriter {
	t.Helper()
	sys, _ := newTestSystem(t)
	cat, err := DefaultCatalog()
	require.NoError(t, err)
	st := &State{Files: map[string]string{}, Blocks: map[string]string{}, Versions: map[string]string{}}
	return shellWriter{
		sys:     sys,
		catalog: cat,
		paths:   NewPaths(sys.HomeDir()),
		files:   fileWriter{sys: sys, state: st, now: func() time.Time { return fixedNow }},
	}
}

func catalogTools(t *testing.T, w shellWriter, ids ...string) []Tool {
	t.Helper()
	var tools []Tool
	for _, id := range ids {
		tool, ok := w.catalog.Get(id)
		require.True(t, ok, id)
		tools = append(tools, tool)
	}
	return tools
}

func names(t *testing.T, dir string) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "*.zsh"))
	require.NoError(t, err)
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = filepath.Base(p)
	}
	return out
}

// zshFiles are every generated zsh file: the two dotfiles, then ~/.zsh/**.zsh.
func zshFiles(t *testing.T, w shellWriter) []string {
	t.Helper()
	files := []string{w.paths.Zshenv, w.paths.Zshrc}
	err := filepath.WalkDir(w.paths.ZshDir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".zsh") {
			files = append(files, p)
		}
		return err
	})
	require.NoError(t, err)
	return files
}

func TestApplyWritesZshLayout(t *testing.T) {
	w := newShellWriter(t)

	reports, err := w.apply(catalogTools(t, w, "mise", "starship", "zoxide"), false)
	require.NoError(t, err)

	zsh := w.paths.ZshDir
	assert.Equal(t, []string{"00-path.zsh", "10-mise.zsh"}, names(t, filepath.Join(zsh, "env")))
	assert.Equal(t, []string{"10-mise.zsh", "20-core.zsh", "50-starship.zsh", "60-zoxide.zsh"}, names(t, filepath.Join(zsh, "tools")))
	assert.Equal(t, []string{"00-base.zsh"}, names(t, filepath.Join(zsh, "aliases")))
	assert.Equal(t, "# managed by shm (tool: zoxide)\neval \"$(zoxide init zsh)\"\n",
		read(t, filepath.Join(zsh, "tools", "60-zoxide.zsh")))
	assert.FileExists(t, filepath.Join(zsh, "env.zsh"))
	assert.FileExists(t, filepath.Join(zsh, "init.zsh"))
	assert.Contains(t, read(t, w.paths.Zshrc), `source "$HOME/.zsh/init.zsh"`)
	assert.Contains(t, read(t, w.paths.Zshenv), "skip_global_compinit=1")
	assert.FileExists(t, filepath.Join(w.paths.Home, ".config", "starship.toml"))
	for _, d := range []string{"env", "functions", "completions", "tools", "aliases", "local"} {
		assert.DirExists(t, filepath.Join(zsh, d))
	}
	assert.NoDirExists(t, filepath.Join(w.paths.Home, ".config", "shell-manager"))
	assert.NoDirExists(t, filepath.Join(w.paths.Home, ".config", "zsh"))
	// 5 shm files + env/10-mise + 3 tool fragments + starship.toml + 2 blocks
	assert.Len(t, reports, 12)
	for _, r := range reports {
		assert.Equal(t, ResultOK, r.Result, r.Path)
	}
}

func TestApplyWritesAntidotePluginsAtItsDefaultPath(t *testing.T) {
	w := newShellWriter(t)
	_, err := w.apply(catalogTools(t, w, "antidote"), false)
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(w.paths.Home, ".zsh_plugins.txt"))
}

func TestApplyKeepsUserZshrcBelowTheBlock(t *testing.T) {
	w := newShellWriter(t)
	require.NoError(t, os.WriteFile(w.paths.Zshrc, []byte("export NVM_DIR=x\n"), 0o644))

	_, err := w.apply(nil, false)
	require.NoError(t, err)

	got := read(t, w.paths.Zshrc)
	assert.True(t, strings.HasPrefix(got, blockBegin+"\n"), got)
	assert.True(t, strings.HasSuffix(got, "\nexport NVM_DIR=x\n"), got)
	assert.Equal(t, "export NVM_DIR=x\n", read(t, w.paths.Zshrc+backupSuffix))
}

func TestApplyTwiceIsIdempotent(t *testing.T) {
	w := newShellWriter(t)
	tools := catalogTools(t, w, "mise", "zoxide")
	_, err := w.apply(tools, false)
	require.NoError(t, err)

	reports, err := w.apply(tools, false)
	require.NoError(t, err)
	for _, r := range reports {
		assert.Equal(t, ResultOK, r.Result, r.Path)
	}
	backups, err := filepath.Glob(filepath.Join(w.paths.Home, ".*.bak.*"))
	require.NoError(t, err)
	assert.Empty(t, backups)
}

func TestApplyRemovesOnlyFragmentsOfRemovedTools(t *testing.T) {
	w := newShellWriter(t)
	tools := filepath.Join(w.paths.ZshDir, "tools")
	gone := filepath.Join(tools, "42-gone.zsh")                         // managed, tool no longer in catalog
	goneEnv := filepath.Join(w.paths.ZshDir, "env", "42-gone.zsh")      // same, env part
	failed := filepath.Join(tools, "50-starship.zsh")                   // managed, tool failed this run
	mine := filepath.Join(tools, "99-mine.zsh")                         // not managed
	for _, p := range []string{gone, goneEnv, failed} {
		_, err := w.files.write(p, []byte("# old\n"), false)
		require.NoError(t, err)
	}
	require.NoError(t, os.WriteFile(mine, []byte("# mine\n"), 0o644))

	reports, err := w.apply(nil, false)
	require.NoError(t, err)

	assert.NoFileExists(t, gone)
	assert.NoFileExists(t, goneEnv)
	assert.FileExists(t, failed)
	assert.FileExists(t, mine)
	assert.Contains(t, reports, FileReport{Path: gone, Result: ResultOK})
}

func TestApplyRemovesStaleFragmentOfOKTool(t *testing.T) {
	w := newShellWriter(t)
	stale := filepath.Join(w.paths.ZshDir, "tools", "60-starship.zsh") // starship is priority 50 in the catalog
	_, err := w.files.write(stale, []byte("# old\n"), false)
	require.NoError(t, err)

	_, err = w.apply(catalogTools(t, w, "starship"), false)
	require.NoError(t, err)

	assert.NoFileExists(t, stale)
	assert.FileExists(t, filepath.Join(w.paths.ZshDir, "tools", "50-starship.zsh"))
}

func TestApplyBrokenZshrcMarkersStillWritesTheRest(t *testing.T) {
	w := newShellWriter(t)
	broken := blockBegin + "\n# half a block\n"
	require.NoError(t, os.WriteFile(w.paths.Zshrc, []byte(broken), 0o644))

	reports, err := w.apply(nil, false)

	assert.ErrorIs(t, err, ErrBrokenMarkers)
	assert.Equal(t, broken, read(t, w.paths.Zshrc))
	assert.FileExists(t, filepath.Join(w.paths.ZshDir, "init.zsh"))
	assert.Contains(t, read(t, w.paths.Zshenv), blockBegin)
	assert.Contains(t, reports, FileReport{Path: w.paths.Zshrc, Result: ResultFailed})
}

func TestShellConfigGolden(t *testing.T) {
	w := newShellWriter(t)
	_, err := w.apply(catalogTools(t, w, "mise", "antidote", "starship", "fzf", "zoxide"), false)
	require.NoError(t, err)

	var b strings.Builder
	for _, p := range zshFiles(t, w) {
		b.WriteString("== " + strings.TrimPrefix(p, w.paths.Home) + " ==\n")
		b.WriteString(read(t, p))
	}
	golden.RequireEqual(t, []byte(b.String()))
}

func TestShellConfigIsValidZsh(t *testing.T) {
	zsh, err := lookSystemZsh()
	if err != nil {
		t.Skip("zsh not installed")
	}
	w := newShellWriter(t)
	_, err = w.apply(catalogTools(t, w, "mise", "antidote", "starship", "fzf", "zoxide"), false)
	require.NoError(t, err)
	for _, p := range zshFiles(t, w) {
		out, err := runZshSyntaxCheck(zsh, p)
		assert.NoError(t, err, "%s: %s", p, out)
	}
}

// Agents and scripts run `zsh -c`, which reads only ~/.zshenv.
func TestNonInteractiveZshGetsThePathSilently(t *testing.T) {
	zsh, err := lookSystemZsh()
	if err != nil {
		t.Skip("zsh not installed")
	}
	w := newShellWriter(t)
	_, err = w.apply(catalogTools(t, w, "mise"), false)
	require.NoError(t, err)
	home := w.paths.Home
	shims := filepath.Join(home, ".local", "share", "mise", "shims")
	bin := filepath.Join(home, ".local", "bin")

	out, err := runZsh(zsh, home, "print -r -- $PATH")
	require.NoError(t, err, out)
	assert.True(t, strings.HasPrefix(out, shims+":"+bin+":"), "PATH=%q", out)

	require.NoError(t, os.RemoveAll(w.paths.ZshDir))
	out, err = runZsh(zsh, home, "print -r -- ok")
	require.NoError(t, err, out)
	assert.Equal(t, "ok\n", out, "no output besides the command's own")
}
```

Add to `helpers_test.go`:

```go
// runZsh runs `zsh -c script` non-interactively with a clean environment
// and HOME set, returning stdout and stderr combined.
func runZsh(zsh, home, script string) (string, error) {
	cmd := exec.Command(zsh, "-c", script)
	cmd.Env = []string{"HOME=" + home, "PATH=/usr/bin:/bin", "TERM=dumb"}
	out, err := cmd.CombinedOutput()
	return string(out), err
}
```

Fix the other tests:
- `system_test.go` `TestPaths`: replace the `Stub`/`ZshD` assertions with
  ```go
  	assert.Equal(t, "/h/.zshrc", p.Zshrc)
  	assert.Equal(t, "/h/.zshenv", p.Zshenv)
  	assert.Equal(t, "/h/.zsh", p.ZshDir)
  ```
- `engine_test.go` `newTestEngine`: replace the `for _, name := range []string{"zshrc", ...}` loop with a copy of the whole `files/zsh` tree:
  ```go
  	err = fs.WalkDir(registry, "files/zsh", func(p string, d fs.DirEntry, err error) error {
  		if err != nil || d.IsDir() {
  			return err
  		}
  		data, err := fs.ReadFile(registry, p)
  		fsys[p] = &fstest.MapFile{Data: data}
  		return err
  	})
  	require.NoError(t, err)
  ```
  and replace every `e.Paths.Stub` with `e.Paths.Zshrc`.
- `doctor_test.go` `TestDoctorFlagsModifiedAndMissingManagedFiles`:
  ```go
  	initZsh := filepath.Join(e.Paths.ZshDir, "init.zsh")
  	envZsh := filepath.Join(e.Paths.ZshDir, "env.zsh")
  	require.NoError(t, os.WriteFile(initZsh, []byte("hand edit"), 0o644))
  	require.NoError(t, os.Remove(envZsh))

  	rep, err := e.Doctor(context.Background())
  	require.NoError(t, err)

  	checks := checksByName(rep)
  	assert.Equal(t, CheckWarn, checks[initZsh].Status)
  	assert.Contains(t, checks[initZsh].Detail, "modified")
  	assert.Equal(t, CheckWarn, checks[envZsh].Status)
  	assert.Contains(t, checks[envZsh].Detail, "missing")
  	assert.Equal(t, CheckOK, checks[filepath.Join(e.Paths.ZshDir, "tools", "20-core.zsh")].Status)
  ```
- `catalog_test.go` `TestEmbeddedCatalogIsValid`: the list becomes `shellProfile` (see Step 5) prefixed with `files/zsh/`, plus the two blocks:
  ```go
  	srcs := []string{"files/zsh/zshenv-block", "files/zsh/zshrc-block"}
  	for _, name := range shellProfile {
  		srcs = append(srcs, "files/zsh/"+name)
  	}
  	for _, src := range srcs {
  		_, err := cat.File(src)
  		assert.NoError(t, err, src)
  	}
  ```

- [ ] **Step 4: Run to see them fail**

Run: `go test ./internal/shellmanager/`
Expected: FAIL to compile (`Paths.Zshrc`, `Zshenv`, `ZshDir`, `shellProfile` undefined).

- [ ] **Step 5: Implement**

`paths.go` — the struct and constructor become:

```go
// Paths are the locations shell-manager reads and writes.
type Paths struct {
	Home      string
	ShmDir    string // ~/.shm: shm's own config, state, log, lock, tmp
	StateFile string
	LockFile  string
	LogFile   string
	TmpDir    string
	Zshenv    string // ~/.zshenv: holds an shm block
	Zshrc     string // ~/.zshrc: holds an shm block
	ZshDir    string // ~/.zsh: the zsh config, shm's files and the user's
	AliasLink string // ~/.local/bin/shm → shell-manager
}

// NewPaths returns the paths for a home directory.
func NewPaths(home string) Paths {
	shm := filepath.Join(home, ".shm")
	return Paths{
		Home:      home,
		ShmDir:    shm,
		StateFile: filepath.Join(shm, "state.toml"),
		LockFile:  filepath.Join(shm, "lock"),
		LogFile:   filepath.Join(shm, "shm.log"),
		TmpDir:    filepath.Join(shm, "tmp"),
		Zshenv:    filepath.Join(home, ".zshenv"),
		Zshrc:     filepath.Join(home, ".zshrc"),
		ZshDir:    filepath.Join(home, ".zsh"),
		AliasLink: filepath.Join(home, ".local", "bin", AliasName),
	}
}
```

`shellrc.go` — replace the vars, `apply`, `desired` and `orphans`:

```go
var (
	// zshDirs are created under ~/.zsh so the user sees where things go.
	zshDirs = []string{"env", "functions", "completions", "tools", "aliases", "local"}
	// shellProfile are shm's own files under ~/.zsh (from registry/files/zsh/).
	shellProfile = []string{"env.zsh", "init.zsh", "env/00-path.zsh", "tools/20-core.zsh", "aliases/00-base.zsh"}
	// fragmentDirs get one generated fragment per tool: env (every zsh) and
	// tools (interactive only).
	fragmentDirs = []string{"env", "tools"}
)

// shellWriter writes the managed zsh config: the shm blocks in ~/.zshenv
// and ~/.zshrc, shm's files under ~/.zsh and one fragment per tool.
type shellWriter struct {
	sys     System
	catalog *Catalog
	paths   Paths
	files   fileWriter
}

// apply writes the config for okTools (the tools that ended OK this run).
// The blocks go last: a block that can't be written (broken markers) fails
// on its own without stopping the rest.
func (w shellWriter) apply(okTools []Tool, force bool) ([]FileReport, error) {
	for _, d := range zshDirs {
		if err := w.sys.MkdirAll(filepath.Join(w.paths.ZshDir, d), 0o755); err != nil {
			return nil, err
		}
	}
	desired, err := w.desired(okTools)
	if err != nil {
		return nil, err
	}
	var reports []FileReport
	for _, path := range slices.Sorted(maps.Keys(desired)) {
		res, err := w.files.write(path, desired[path], force)
		if err != nil {
			return reports, fmt.Errorf("writing %s: %w", path, err)
		}
		reports = append(reports, FileReport{Path: path, Result: res})
	}
	orphans, err := w.orphans(desired, okTools)
	if err != nil {
		return reports, err
	}
	for _, path := range orphans {
		res, err := w.files.remove(path)
		if err != nil {
			return reports, fmt.Errorf("removing %s: %w", path, err)
		}
		reports = append(reports, FileReport{Path: path, Result: res})
	}
	var errs []error
	for _, b := range []struct{ path, src string }{
		{w.paths.Zshenv, "files/zsh/zshenv-block"},
		{w.paths.Zshrc, "files/zsh/zshrc-block"},
	} {
		block, err := w.catalog.File(b.src)
		if err != nil {
			return reports, err
		}
		res, err := w.files.writeBlock(b.path, block, force)
		reports = append(reports, FileReport{Path: b.path, Result: res})
		if err != nil {
			errs = append(errs, fmt.Errorf("writing the shm block: %w", err))
		}
	}
	return reports, errors.Join(errs...)
}

func (w shellWriter) desired(okTools []Tool) (map[string][]byte, error) {
	desired := map[string][]byte{}
	embed := func(dest, src string) error {
		data, err := w.catalog.File(src)
		if err != nil {
			return err
		}
		desired[dest] = data
		return nil
	}
	for _, name := range shellProfile {
		if err := embed(filepath.Join(w.paths.ZshDir, name), "files/zsh/"+name); err != nil {
			return nil, err
		}
	}
	for _, t := range okTools {
		if t.Shell != nil {
			name := fmt.Sprintf("%02d-%s.zsh", t.Shell.Priority, t.ID)
			if t.Shell.Env != "" {
				desired[filepath.Join(w.paths.ZshDir, "env", name)] = fragment(t.ID, t.Shell.Env)
			}
			if t.Shell.Snippet != "" {
				desired[filepath.Join(w.paths.ZshDir, "tools", name)] = fragment(t.ID, t.Shell.Snippet)
			}
		}
		for _, f := range t.Files {
			if err := embed(w.paths.Expand(f.Dest), f.Src); err != nil {
				return nil, err
			}
		}
	}
	return desired, nil
}

func fragment(id, body string) []byte {
	return fmt.Appendf(nil, "# managed by shm (tool: %s)\n%s\n", id, strings.TrimRight(body, "\n"))
}

// orphans are managed fragments in env/ and tools/ of tools no longer in
// the catalog. Fragments of catalog tools that failed this run are kept.
func (w shellWriter) orphans(desired map[string][]byte, okTools []Tool) ([]string, error) {
	ok := map[string]bool{}
	for _, t := range okTools {
		ok[t.ID] = true
	}
	var out []string
	for _, dir := range fragmentDirs {
		existing, err := w.sys.Glob(filepath.Join(w.paths.ZshDir, dir, "*.zsh"))
		if err != nil {
			return nil, err
		}
		for _, path := range existing {
			if _, wanted := desired[path]; wanted {
				continue
			}
			if _, managed := w.files.state.Files[path]; !managed {
				continue
			}
			if rel, err := filepath.Rel(w.paths.ZshDir, path); err == nil && slices.Contains(shellProfile, rel) {
				continue
			}
			_, id, _ := strings.Cut(strings.TrimSuffix(filepath.Base(path), ".zsh"), "-")
			// OK tools have exactly the fragments in desired, so any other
			// managed fragment of theirs is stale.
			if _, inCatalog := w.catalog.Get(id); inCatalog && !ok[id] {
				continue
			}
			out = append(out, path)
		}
	}
	return out, nil
}
```

Imports of `shellrc.go`: `errors`, `fmt`, `maps`, `path/filepath`, `slices`, `strings`.

- [ ] **Step 6: Regenerate the golden file and review it**

Run: `go test ./internal/shellmanager/ -run TestShellConfigGolden -update`
Then open `internal/shellmanager/testdata/TestShellConfigGolden.golden` and check by eye: it starts with `== /.zshenv ==` and `== /.zshrc ==` blocks; `== /.zsh/env/10-mise.zsh ==` holds the shims line; `tools/30-antidote.zsh` ends in `antidote load`; `tools/60-zoxide.zsh` has no `alias cd`; there is no `/.config/` path anywhere.

- [ ] **Step 7: Run all tests**

Run: `go test ./... && golangci-lint run`
Expected: PASS (on a machine with zsh, `TestShellConfigIsValidZsh` and `TestNonInteractiveZshGetsThePathSilently` run instead of skipping); no lint findings.

- [ ] **Step 8: Commit**

```bash
git add -A internal/shellmanager
git commit -m "Generate the zsh config under ~/.zsh with shm blocks in the dotfiles"
```

---

### Task 6: Doctor checks the blocks

**Files:**
- Modify: `internal/shellmanager/doctor.go`, `internal/shellmanager/doctor_test.go`

**Interfaces:**
- Consumes: `State.Blocks`, `findBlock`, `ErrBrokenMarkers`, `resolveLink`, `hashBytes` (Task 3); `Paths.Zshrc` (Task 5); `setupOp`, `runOp`, `newTestEngine`, `checksByName` (existing test helpers).
- Produces: one `CheckResult` per path in `state.Blocks`, `Name` = the path.

- [ ] **Step 1: Write the failing test**

```go
func TestDoctorReportsShmBlocks(t *testing.T) {
	e, _, _ := newTestEngine(t, map[string]string{"a": fakeTool("a", KindBuiltin, "")})
	_, _, err := runOp(t, setupOp(e))
	require.NoError(t, err)

	checks := checksByName(mustDoctor(t, e))
	assert.Equal(t, CheckOK, checks[e.Paths.Zshrc].Status)
	assert.Equal(t, CheckOK, checks[e.Paths.Zshenv].Status)

	require.NoError(t, os.WriteFile(e.Paths.Zshrc, []byte("export FOO=1\n"), 0o644)) // block deleted
	zshenv := read(t, e.Paths.Zshenv)
	require.NoError(t, os.WriteFile(e.Paths.Zshenv, []byte(strings.Replace(zshenv, "skip_global_compinit=1", "X=1", 1)), 0o644))
	checks = checksByName(mustDoctor(t, e))
	assert.Equal(t, CheckWarn, checks[e.Paths.Zshrc].Status)
	assert.Contains(t, checks[e.Paths.Zshrc].Detail, "missing")
	assert.Equal(t, CheckWarn, checks[e.Paths.Zshenv].Status)
	assert.Contains(t, checks[e.Paths.Zshenv].Detail, "modified")

	require.NoError(t, os.WriteFile(e.Paths.Zshrc, []byte(blockBegin+"\n"), 0o644))
	checks = checksByName(mustDoctor(t, e))
	assert.Equal(t, CheckWarn, checks[e.Paths.Zshrc].Status)
	assert.Contains(t, checks[e.Paths.Zshrc].Detail, "broken")
}

func mustDoctor(t *testing.T, e *Engine) Report {
	t.Helper()
	rep, err := e.Doctor(context.Background())
	require.NoError(t, err)
	return rep
}
```

(add `"strings"` to the imports of `doctor_test.go`.)

- [ ] **Step 2: Run to see it fail**

Run: `go test ./internal/shellmanager/ -run TestDoctorReportsShmBlocks`
Expected: FAIL (no check named `~/.zshrc`).

- [ ] **Step 3: Implement**

In `Doctor`, right after the managed-files loop:

```go
	for _, path := range slices.Sorted(maps.Keys(state.Blocks)) {
		rep.Checks = append(rep.Checks, e.blockCheck(path, state.Blocks[path]))
	}
```

and below `Doctor`:

```go
// blockCheck is doctor's view of the shm block in path.
func (e *Engine) blockCheck(path, recorded string) CheckResult {
	c := CheckResult{Name: path, Status: CheckOK}
	data, err := e.System.ReadFile(resolveLink(e.System, path))
	if err != nil {
		c.Status, c.Detail = CheckWarn, "missing (run shm update)"
		return c
	}
	start, end, found, err := findBlock(data)
	switch {
	case err != nil:
		c.Status, c.Detail = CheckWarn, ErrBrokenMarkers.Error()
	case !found:
		c.Status, c.Detail = CheckWarn, "shm block missing (run shm update)"
	case hashBytes(data[start:end]) != recorded:
		c.Status, c.Detail = CheckWarn, "shm block modified by hand (update --force overwrites it)"
	}
	return c
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/shellmanager/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/shellmanager/doctor.go internal/shellmanager/doctor_test.go
git commit -m "Report the shm blocks in doctor"
```

---

### Task 7: Docs and end-to-end check

**Files:**
- Modify: `README.md`, `CLAUDE.md`

**Interfaces:**
- Consumes: everything above.
- Produces: user docs matching the spec.

- [ ] **Step 1: README**

- Delete the whole `## Migrating` section.
- In `## Install`, the paragraph after the one-liner stays; no path there changes.
- Replace `## Where things live` (the table and the paragraph after it) with:

```markdown
## Where things live

| Path | Owner |
|---|---|
| `~/.local/bin/shell-manager`, `~/.local/bin/shm` | shm (binary and alias link) |
| `~/.shm/` | shm: `state.toml`, `shm.log`, lock, tmp; `config.toml` is yours (optional: `log_level`) |
| `~/.zshenv`, `~/.zshrc` | **You.** shm only keeps its block between `# >>> shm >>>` and `# <<< shm <<<` at the top |
| `~/.zsh/` | Your zsh config, auto-discovered (below). shm owns only the files it wrote |
| `~/.zsh_plugins.txt` | antidote plugin list (shm) |
| `~/.config/starship.toml` | starship config (shm) |

`~/.zsh/` is loaded in this order; inside each directory, files load
alphabetically (use an `NN-` prefix to order them). Add or remove a file and
open a new shell; nothing else to edit.

| Directory | What goes there | Loaded by |
|---|---|---|
| `env/*.zsh` | Environment variables and `path` (`00-path.zsh` and tool shims are shm's) | every zsh, including scripts and AI agents (`~/.zshenv`); keep it fast and silent |
| `functions/` | One function per file, file name = function name (autoloaded) | interactive shells |
| `completions/` | Completion functions (`_mycmd`) | interactive shells (on `fpath` before `compinit`) |
| `tools/*.zsh` | Tool setup: one `NN-<tool>.zsh` per tool (shm) | interactive shells |
| `aliases/*.zsh` | Aliases (`00-base.zsh` is shm's) | interactive shells |
| `local/*.zsh` | Anything else; loads last, so it can override the rest | interactive shells |

If you edit a file or block shm manages, `update` leaves it alone and
`doctor` reports it as modified; `update --force` replaces it after a backup
(`<file>.bak.<timestamp>`). Put your own config in new files instead.
```

- [ ] **Step 2: CLAUDE.md**

Replace the design sentence at the top with:

```markdown
See `README.md` for usage. Design: base spec
`docs/superpowers/specs/2026-10-10-shell-setup-cli-design.md` (written under
the old name), `docs/superpowers/specs/2026-10-11-shell-manager-redesign-design.md`
(names, the `setup` command, the installer prompt) and
`docs/superpowers/specs/2026-10-11-shm-layout-design.md`, which overrides
paths (`~/.shm`, `~/.zsh/`, blocks in `~/.zshenv`/`~/.zshrc`) and removes all
migration.
```

In "Repo layout", the `internal/shellmanager` bullet: replace `legacy purge, migration from shell-setup` with `shm blocks in user dotfiles`, and the registry bullet's `files/` sentence with: `` `files/` holds the config files modules ship and shm's zsh files (`files/zsh/`). ``

- [ ] **Step 3: Full verification**

Run: `task test && task lint`
Expected: Go tests PASS, `All tests passed` from the installer tests, no lint findings.

Run, in a throwaway HOME (never the real one):

```bash
H=$(mktemp -d)
HOME=$H go run . doctor --plain; echo "exit=$?"
ls -A "$H"
```

Expected: doctor runs (non-zero exit is fine: nothing is installed) and `$H` contains at most `.shm/` — no `.config/shell-manager` or `.local/state`.

- [ ] **Step 4: Verify mise activate + shims coexist (spec's open point)**

Only if mise is installed on the dev machine; read-only, does not touch `$HOME`:

```bash
zsh -fc 'path=("$HOME/.local/share/mise/shims" $path); eval "$(mise activate zsh)"; print -l $path | head -5'
```

Expected: no error; note in the PR whether `mise activate` keeps or drops the shims dir. If it errors, stop and report before releasing.

- [ ] **Step 5: Commit**

```bash
git add README.md CLAUDE.md
git commit -m "Document ~/.shm and the ~/.zsh layout"
```
