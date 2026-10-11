# shell-manager — Plan de implementación del rediseño

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Renombrar `shell-setup` a `shell-manager` (alias `shm`), convertir `init` en el comando `setup`, migrar automáticamente las instalaciones v0.1.x y hacer que `install.sh` pregunte si ejecutar `setup`.

**Architecture:** Sin cambios de arquitectura (ver la spec base). Se renombra el módulo Go, el paquete de dominio (`internal/shellsetup` → `internal/shellmanager`), el binario, las rutas, las variables de entorno y el alias. Se añade un paso de migración en el dominio (`migrate.go`), que se ejecuta antes de tomar el lock. El bootstrap gana una pregunta final que lee de `/dev/tty`.

**Tech Stack:** Go 1.27.2 (toolchain vía `mise exec --`), cobra, bubbletea v2, testify, testscript; bash para el bootstrap.

**Spec:** `docs/superpowers/specs/2026-10-11-shell-manager-redesign-design.md` (base: `docs/superpowers/specs/2026-10-10-shell-setup-cli-design.md`).

## Global Constraints

- Nombre `shell-manager`. Repo `BlasterM2A/shell-manager`. Módulo `github.com/BlasterM2A/shell-manager`. Alias `shm`, que se crea como `~/.local/bin/shm` → `shell-manager`.
- El paquete de dominio pasa de `internal/shellsetup` a `internal/shellmanager`.
- Comandos: `setup`, `update`, `doctor` y `self-update`. `init` desaparece sin alias.
- Rutas:
  - `~/.config/shell-manager/`: config.toml, zshrc, `zsh.d/`, plugins.txt.
  - `~/.local/state/shell-manager/`: state.toml, `shell-manager.log`, lock, tmp, bootstrap.sha256.
  - El stub hace `source "$HOME/.config/shell-manager/zshrc"`.
- Variables de entorno:
  - Generales: `SHELL_MANAGER_*`.
  - Del bootstrap: `SHELL_MANAGER_BIN_DIR`, `SHELL_MANAGER_STATE_DIR`, `SHELL_MANAGER_OS_RELEASE`, `SHELL_MANAGER_RELEASE_BASE` y `SHELL_MANAGER_TTY`.
- Assets: `shell-manager_linux_<amd64|arm64>.tar.gz`, con el binario `shell-manager` en la raíz, y `checksums.txt`.
- Los textos visibles van en inglés. Las sugerencias al usuario usan el alias: `run shm setup`, `run shm update`, `Run \`shm update\``.
- Herramientas: siempre con `mise exec -- <cmd>`. Verificación: `mise exec -- task lint test`.
- **Nunca** ejecutes `setup` ni `update` reales contra tu propio `$HOME`, y nunca ejecutes `install.sh main` de verdad. Sin Docker ni shellcheck.
- No tocar `docs/superpowers/specs/2026-10-10-*`, `docs/superpowers/plans/2026-10-10-*` ni los documentos de `2026-09-19`, salvo lo que diga la Task 5: son históricos.

## Desviación respecto a la spec (decidida al planificar)

- **Directorio de estado.** La spec dice que el directorio antiguo se renombra si el nuevo no existe y que nunca se mezclan. Pero el logger (`internal/cmd/factory.go`) **crea `~/.local/state/shell-manager/` y abre `shell-manager.log` antes** de que se ejecute la migración. El directorio nuevo siempre existiría y el estado nunca migraría.
- **Lo que se hace en su lugar:** la migración **mueve entrada a entrada** cada archivo o directorio antiguo que no exista ya en el destino, y nunca sobrescribe nada. Después borra el directorio antiguo si ha quedado vacío. Las entradas en conflicto se quedan en el directorio antiguo y `doctor` las reporta.
- **El log antiguo** se conserva con su nombre (`shell-setup.log`) dentro del directorio nuevo. No se renombra, porque `shell-manager.log` ya existe por la ejecución actual.
- **El directorio de config** usa la misma función, por coherencia.

## Review Focus

1. **Migración con el logger ya abierto:** el directorio de estado nuevo ya existe (con el log de esta ejecución) y el `state.toml` antiguo aun así se migra, con las claves reescritas. Lo prueba la Task 3 (`TestMigrateMergesIntoStateDirCreatedByTheLogger`).
2. **Primer `setup` tras migrar:** reescribe el stub de `~/.zshrc` y el zshrc generado **sin** crear `.bak`. Lo prueba la Task 3 (`TestSetupAfterMigrationRewritesStubWithoutBackup`).
3. **Archivo real `~/.local/bin/shs` del usuario:** nunca se borra. Lo prueba la Task 3 (`TestMigrateKeepsRealFileNamedShs`).
4. **`install.sh` sin terminal (CI, pipe):** no se queda esperando una respuesta ni ejecuta `setup`. Lo prueba la Task 4 (caso "no tty").
5. **Respuesta vacía (Enter) en la pregunta:** ejecuta `setup`. Lo prueba la Task 4 (caso "empty answer").

---

### Task 1: Renombrado mecánico a `shell-manager` (sin cambiar todavía `init`)

**Files:**
- Move: `internal/shellsetup/` → `internal/shellmanager/`, y `internal/shellmanager/shellsetup.go` → `internal/shellmanager/shellmanager.go`.
- Modify: todos los archivos versionados salvo `docs/**`, `install.sh`, `tests/test_install.sh`, `README.md`, `CLAUDE.md` y `go.sum`. En concreto: `go.mod`, `main.go`, `internal/**`, `.goreleaser.yml`, `.golangci.yml`, `Taskfile.yml`, `.gitignore`, `internal/cmd/testdata/scripts/*.txtar` y `internal/shellmanager/testdata/TestShellConfigGolden.golden` (regenerado).

**Interfaces:**
- Produces: paquete `shellmanager` (antes `shellsetup`) con la misma API. `AliasName = "shm"`. `NewPaths` devuelve rutas `.../shell-manager` y `LogFile` es `shell-manager.log`. `config.DefaultPath` devuelve `~/.config/shell-manager/config.toml` con prefijo de entorno `SHELL_MANAGER_`. El comando raíz se llama `shell-manager`. `repoSlug = "BlasterM2A/shell-manager"`.

- [ ] **Step 1: Mover el paquete de dominio**

```bash
git mv internal/shellsetup internal/shellmanager
git mv internal/shellmanager/shellsetup.go internal/shellmanager/shellmanager.go
```

- [ ] **Step 2: Reemplazos en orden**

El orden importa. Primero se convierten las sugerencias de comandos al alias y después el nombre genérico.

```bash
FILES=$(git ls-files -- ':!docs' ':!install.sh' ':!tests/test_install.sh' ':!README.md' ':!CLAUDE.md' ':!go.sum')
sed -i \
  -e 's/shell-setup init/shm init/g' \
  -e 's/shell-setup update/shm update/g' \
  -e 's/shell-setup self-update/shm self-update/g' \
  -e 's/shs update/shm update/g' \
  -e 's/shell-setup/shell-manager/g' \
  -e 's/SHELL_SETUP/SHELL_MANAGER/g' \
  -e 's/_shell_setup_source_dir/_shell_manager_source_dir/g' \
  -e 's/shellsetup/shellmanager/g' \
  $FILES
sed -i -e 's/AliasName = "shs"/AliasName = "shm"/' -e 's/\bshs\b/shm/g' \
  internal/shellmanager/alias.go internal/shellmanager/paths.go internal/cmd/factory.go
```

- [ ] **Step 3: Comprobar que no quedan restos**

Run: `git grep -n -I -e 'shell-setup' -e 'shellsetup' -e 'SHELL_SETUP' -e '\bshs\b' -- ':!docs' ':!install.sh' ':!tests/test_install.sh' ':!README.md' ':!CLAUDE.md' ':!go.sum'`
Expected: sin salida. Si queda algo (por ejemplo un comentario), corrígelo a mano siguiendo las mismas reglas.

Run: `git grep -n -e 'run shm init' -- internal`
Expected: aparece en `internal/shellmanager/alias.go` y `doctor.go`. Es correcto por ahora: la Task 2 lo cambia por `setup`.

- [ ] **Step 4: Regenerar el golden y verificar**

```bash
mise exec -- gofmt -l .
mise exec -- go build ./...
mise exec -- go test ./internal/shellmanager/ -run TestShellConfigGolden -update
mise exec -- task lint test
```

Expected: `gofmt` sin salida, la compilación correcta, `0 issues.` y todos los tests en verde, incluidos los del bootstrap, que todavía usan el `install.sh` antiguo. Revisa en `internal/shellmanager/testdata/TestShellConfigGolden.golden` que todas las rutas digan `.config/shell-manager` y las cabeceras `managed by shell-manager`.

Run: `mise exec -- go run . --version`
Expected: `shell-manager dev (none)`

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "Rename shell-setup to shell-manager (alias shm) in code and config"
```

---

### Task 2: El comando `setup` sustituye a `init`

**Files:**
- Move: `internal/cmd/init.go` → `internal/cmd/setup.go` (se reescribe entero).
- Modify:
  - `internal/cmd/root.go`, `internal/cmd/root_test.go`, `internal/cmd/commands_test.go` y `internal/cmd/apply.go` (comentario).
  - `internal/shellmanager/shellmanager.go` y `internal/shellmanager/*_test.go`.
  - `internal/shellmanager/alias.go` y `internal/shellmanager/doctor.go` (mensajes).

**Interfaces:**
- Consumes: el paquete `shellmanager` de la Task 1.
- Produces:
  - `(*Engine).Setup(ctx, chan<- Event) (Report, error)`, que sustituye a `Init`.
  - El helper de test `setupOp(e)`, que sustituye a `initOp`.
  - En `cmd`: `SetupOptions`, `newSetupCmd(f, runF)` y `runSetup`. El comando es `setup`.

- [ ] **Step 1: Tests que fallan**

En `internal/cmd/root_test.go`, sustituye la aserción de `TestRootRegistersCommands` por:

```go
	assert.Subset(t, names, []string{"setup", "update", "doctor", "self-update"})
	assert.NotContains(t, names, "init")
```

En `internal/cmd/commands_test.go`, sustituye `TestInitRejectsArguments` por:

```go
func TestSetupRejectsArguments(t *testing.T) {
	f, _ := newTestFactory(t)
	cmd := newSetupCmd(f, func(context.Context, *SetupOptions) error { return nil })
	cmd.SetArgs([]string{"extra"})
	assert.Error(t, cmd.Execute())
}
```

Renombra en los tests del dominio:

```bash
sed -i -e 's/e\.Init(/e.Setup(/g' -e 's/\binitOp\b/setupOp/g' -e 's/func TestInit/func TestSetup/g' internal/shellmanager/*_test.go
```

Run: `mise exec -- go vet ./internal/...`
Expected: FAIL, con `undefined: newSetupCmd`, `e.Setup undefined` y similares.

- [ ] **Step 2: Renombrar en el dominio**

```bash
sed -i -e 's/func (e \*Engine) Init(/func (e *Engine) Setup(/' -e 's/modeInit/modeSetup/g' \
  -e 's#^// Init installs what is missing#// Setup installs what is missing#' internal/shellmanager/shellmanager.go
sed -i 's/run shm init/run shm setup/g' internal/shellmanager/alias.go internal/shellmanager/doctor.go
```

Comprueba que el comentario de `Update` en `shellmanager.go` dice ahora "Update is Setup plus updating every installed tool." Si sigue diciendo "Init", cámbialo a mano.

- [ ] **Step 3: El comando**

```bash
git mv internal/cmd/init.go internal/cmd/setup.go
```

`internal/cmd/setup.go` (contenido completo):

```go
package cmd

import (
	"context"

	"github.com/spf13/cobra"
)

// SetupOptions are the inputs of setup.
type SetupOptions struct {
	Factory *Factory
}

func newSetupCmd(f *Factory, runF func(context.Context, *SetupOptions) error) *cobra.Command {
	opts := &SetupOptions{Factory: f}
	return &cobra.Command{
		Use:   "setup",
		Short: "Install what is missing, write the shell config and make zsh the login shell",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if runF != nil {
				return runF(cmd.Context(), opts)
			}
			return runSetup(cmd.Context(), opts)
		},
	}
}

func runSetup(ctx context.Context, opts *SetupOptions) error {
	e, err := opts.Factory.Engine()
	if err != nil {
		return err
	}
	return runApply(ctx, opts.Factory, e, "Setting up your shell", e.Setup)
}
```

En `internal/cmd/root.go`, cambia `newInitCmd(f, nil),` por `newSetupCmd(f, nil),`. En `internal/cmd/apply.go`, el comentario de `runApply` debe decir "runs Setup or Update".

- [ ] **Step 4: Verificar**

Run: `git grep -n -w -e Init -e initOp -e modeInit -e InitOptions -e newInitCmd -- internal/shellmanager internal/cmd`
Expected: sin salida. El `Init()` de `internal/ui/model` es el de bubbletea y no se toca.

Run: `mise exec -- task lint test && mise exec -- go run . --help | grep -E '^\s+(setup|init)\b'`
Expected: todo en verde y solo aparece la línea de `setup`.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "Replace the init command with setup"
```

---

### Task 3: Migración automática desde `shell-setup` v0.1.x

**Files:**
- Create: `internal/shellmanager/migrate.go` y `internal/shellmanager/migrate_test.go`.
- Modify:
  - `internal/shellmanager/event.go` (añade `ResultMigrated`).
  - `internal/shellmanager/shellmanager.go` (llamada en `apply`).
  - `internal/shellmanager/doctor.go` (check de restos).
  - `internal/cmd/bindings.go` (`ResultMigrated` → OK).

**Interfaces:**
- Consumes: `Engine`, `Paths`, `System` (incluidos `Lstat`, `Glob`, `Rename`, `Remove`, `MkdirAll`), `LoadState`/`Save`, `FileReport`, y los helpers de test `newTestEngine`, `fakeTool`, `runOp`, `setupOp`, `checksByName`, `fileResult` y `read`.
- Produces:
  - `ResultMigrated Result = "migrated"`.
  - `(*Engine).migrateFromShellSetup() ([]FileReport, error)`.
  - `(*Engine).oldInstallLeftovers() []string`.
  - El check de doctor `CheckResult{Name: "shell-setup leftovers", Status: CheckWarn}`.

- [ ] **Step 1: Tests que fallan**

`internal/shellmanager/migrate_test.go`:

```go
package shellmanager

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const oldStub = "# ~/.zshrc — managed by shell-setup.\nsource \"$HOME/.config/shell-setup/zshrc\"\n"

// seedOldInstall lays out a shell-setup v0.1.x install under the engine's
// home: config dir with a generated zshrc and a fragment, state dir with
// state.toml (hashes for the stub and both files), its log, the old binary
// and the shs alias link.
func seedOldInstall(t *testing.T, e *Engine) oldInstall {
	t.Helper()
	old := oldInstallPaths(e.Paths.Home)
	write := func(path, content string) {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
	zshrc := filepath.Join(old.ConfigDir, "zshrc")
	fragment := filepath.Join(old.ConfigDir, "zsh.d", "10-a.zsh")
	write(zshrc, "# old generated zshrc\n")
	write(fragment, "# old fragment\n")
	write(e.Paths.Stub, oldStub)
	write(filepath.Join(old.StateDir, "shell-setup.log"), "old log\n")
	st := &State{
		SchemaVersion: stateSchemaVersion,
		Files: map[string]string{
			e.Paths.Stub: hashBytes([]byte(oldStub)),
			zshrc:        hashBytes([]byte("# old generated zshrc\n")),
			fragment:     hashBytes([]byte("# old fragment\n")),
		},
		Versions: map[string]string{"nerdfont": "v3.5.1"},
	}
	require.NoError(t, st.Save(e.System, filepath.Join(old.StateDir, "state.toml")))
	writeExecutable(t, old.Binary)
	require.NoError(t, os.Symlink("shell-setup", old.Alias))
	return old
}

func TestMigrateMovesConfigAndStateAndRewritesStateKeys(t *testing.T) {
	e, _, _ := newTestEngine(t, map[string]string{"a": fakeTool("a", KindBuiltin, "")})
	old := seedOldInstall(t, e)

	reports, err := e.migrateFromShellSetup()
	require.NoError(t, err)

	assert.NoDirExists(t, old.ConfigDir)
	assert.NoDirExists(t, old.StateDir)
	assert.Equal(t, "# old fragment\n", read(t, filepath.Join(e.Paths.ConfigDir, "zsh.d", "10-a.zsh")))
	assert.FileExists(t, filepath.Join(e.Paths.StateDir, "shell-setup.log"))
	st, err := LoadState(e.System, e.Paths.StateFile)
	require.NoError(t, err)
	assert.Contains(t, st.Files, filepath.Join(e.Paths.ConfigDir, "zsh.d", "10-a.zsh"))
	assert.Contains(t, st.Files, e.Paths.GeneratedZshrc)
	assert.Contains(t, st.Files, e.Paths.Stub)
	assert.NotContains(t, st.Files, filepath.Join(old.ConfigDir, "zshrc"))
	assert.Equal(t, "v3.5.1", st.Versions["nerdfont"])
	assert.Contains(t, reports, FileReport{Path: e.Paths.ConfigDir, Result: ResultMigrated})
	assert.Contains(t, reports, FileReport{Path: e.Paths.StateDir, Result: ResultMigrated})
}

func TestMigrateRemovesOldBinaryAndAliasLink(t *testing.T) {
	e, _, _ := newTestEngine(t, map[string]string{"a": fakeTool("a", KindBuiltin, "")})
	old := seedOldInstall(t, e)

	reports, err := e.migrateFromShellSetup()
	require.NoError(t, err)

	assert.NoFileExists(t, old.Binary)
	_, err = os.Lstat(old.Alias)
	assert.ErrorIs(t, err, os.ErrNotExist)
	assert.Contains(t, reports, FileReport{Path: old.Binary, Result: ResultRemoved})
	assert.Contains(t, reports, FileReport{Path: old.Alias, Result: ResultRemoved})
}

func TestMigrateKeepsRealFileNamedShs(t *testing.T) {
	e, _, _ := newTestEngine(t, map[string]string{"a": fakeTool("a", KindBuiltin, "")})
	old := oldInstallPaths(e.Paths.Home)
	writeExecutable(t, old.Alias, "echo mine")

	_, err := e.migrateFromShellSetup()
	require.NoError(t, err)

	assert.Contains(t, read(t, old.Alias), "echo mine")
}

func TestMigrateMergesIntoStateDirCreatedByTheLogger(t *testing.T) {
	e, _, _ := newTestEngine(t, map[string]string{"a": fakeTool("a", KindBuiltin, "")})
	old := seedOldInstall(t, e)
	// The logger opens the new log before migration runs.
	require.NoError(t, os.MkdirAll(e.Paths.StateDir, 0o755))
	require.NoError(t, os.WriteFile(e.Paths.LogFile, []byte("current run\n"), 0o644))

	_, err := e.migrateFromShellSetup()
	require.NoError(t, err)

	assert.Equal(t, "current run\n", read(t, e.Paths.LogFile), "never overwritten")
	st, err := LoadState(e.System, e.Paths.StateFile)
	require.NoError(t, err)
	assert.Contains(t, st.Files, e.Paths.GeneratedZshrc)
	assert.NoDirExists(t, old.StateDir)
}

func TestMigrateLeavesConflictingEntriesInOldDir(t *testing.T) {
	e, _, _ := newTestEngine(t, map[string]string{"a": fakeTool("a", KindBuiltin, "")})
	old := seedOldInstall(t, e)
	require.NoError(t, os.MkdirAll(e.Paths.ConfigDir, 0o755))
	require.NoError(t, os.WriteFile(e.Paths.GeneratedZshrc, []byte("# new\n"), 0o644))

	_, err := e.migrateFromShellSetup()
	require.NoError(t, err)

	assert.Equal(t, "# new\n", read(t, e.Paths.GeneratedZshrc), "never overwritten")
	assert.FileExists(t, filepath.Join(old.ConfigDir, "zshrc"), "conflict stays behind")
	assert.FileExists(t, filepath.Join(e.Paths.ConfigDir, "zsh.d", "10-a.zsh"), "the rest moves")
	assert.Contains(t, e.oldInstallLeftovers(), old.ConfigDir)
}

func TestMigrateIsIdempotent(t *testing.T) {
	e, _, _ := newTestEngine(t, map[string]string{"a": fakeTool("a", KindBuiltin, "")})
	seedOldInstall(t, e)
	_, err := e.migrateFromShellSetup()
	require.NoError(t, err)

	reports, err := e.migrateFromShellSetup()
	require.NoError(t, err)
	assert.Empty(t, reports)
	assert.Empty(t, e.oldInstallLeftovers())
}

func TestSetupAfterMigrationRewritesStubWithoutBackup(t *testing.T) {
	e, _, _ := newTestEngine(t, map[string]string{"a": fakeTool("a", KindBuiltin, "")})
	seedOldInstall(t, e)

	rep, _, err := runOp(t, setupOp(e))
	require.NoError(t, err)

	assert.Contains(t, read(t, e.Paths.Stub), `source "$HOME/.config/shell-manager/zshrc"`)
	backups, err := filepath.Glob(filepath.Join(e.Paths.Home, ".zshrc.bak.*"))
	require.NoError(t, err)
	assert.Empty(t, backups)
	res, ok := fileResult(rep, e.Paths.ConfigDir)
	assert.True(t, ok)
	assert.Equal(t, ResultMigrated, res)
}

func TestDoctorReportsShellSetupLeftovers(t *testing.T) {
	e, _, _ := newTestEngine(t, map[string]string{"a": fakeTool("a", KindBuiltin, "")})
	old := oldInstallPaths(e.Paths.Home)
	writeExecutable(t, old.Binary)

	rep, err := e.Doctor(context.Background())
	require.NoError(t, err)

	c := checksByName(rep)["shell-setup leftovers"]
	assert.Equal(t, CheckWarn, c.Status)
	assert.Contains(t, c.Detail, old.Binary)
	assert.Contains(t, c.Detail, "run shm setup")
}
```

Run: `mise exec -- go vet ./internal/shellmanager/`
Expected: FAIL, con `undefined: oldInstallPaths`, `ResultMigrated` y similares.

- [ ] **Step 2: Implementación**

En `internal/shellmanager/event.go`, añade después de `ResultRemoved`:

```go
	ResultMigrated Result = "migrated" // moved over from a shell-setup install
```

`internal/shellmanager/migrate.go`:

```go
package shellmanager

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
)

// oldName is the tool's name up to v0.1.x; setup and update migrate what it
// left on the machine.
const oldName = "shell-setup"

// oldInstall are the locations a shell-setup v0.1.x install used.
type oldInstall struct {
	ConfigDir string // ~/.config/shell-setup
	StateDir  string // ~/.local/state/shell-setup
	Binary    string // ~/.local/bin/shell-setup
	Alias     string // ~/.local/bin/shs
}

func oldInstallPaths(home string) oldInstall {
	return oldInstall{
		ConfigDir: filepath.Join(home, ".config", oldName),
		StateDir:  filepath.Join(home, ".local", "state", oldName),
		Binary:    filepath.Join(home, ".local", "bin", oldName),
		Alias:     filepath.Join(home, ".local", "bin", "shs"),
	}
}

// migrateFromShellSetup moves a shell-setup install to the new names. It runs
// before the lock is taken, since the lock lives in the new state directory.
// Entries are moved one by one and never overwrite existing ones (the logger
// has already created the new state directory and its log by now); whatever
// conflicts stays in the old directory, which doctor then reports.
func (e *Engine) migrateFromShellSetup() ([]FileReport, error) {
	old := oldInstallPaths(e.Paths.Home)
	var reports []FileReport

	moved, err := e.mergeDir(old.ConfigDir, e.Paths.ConfigDir)
	if err != nil {
		return reports, err
	}
	if len(moved) > 0 {
		reports = append(reports, FileReport{Path: e.Paths.ConfigDir, Result: ResultMigrated})
	}

	moved, err = e.mergeDir(old.StateDir, e.Paths.StateDir)
	if err != nil {
		return reports, err
	}
	if len(moved) > 0 {
		reports = append(reports, FileReport{Path: e.Paths.StateDir, Result: ResultMigrated})
	}
	// Rewrite managed-file keys whenever the state file now in place may
	// still point into the old config directory.
	if err := e.rewriteStateKeys(old.ConfigDir); err != nil {
		return reports, err
	}

	for _, p := range []string{old.Binary, old.Alias} {
		info, err := e.System.Lstat(p)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return reports, err
		}
		if p == old.Alias && info.Mode()&fs.ModeSymlink == 0 {
			continue // a real file named shs belongs to the user
		}
		if err := e.System.Remove(p); err != nil {
			return reports, err
		}
		reports = append(reports, FileReport{Path: p, Result: ResultRemoved})
	}
	return reports, nil
}

// mergeDir moves every entry of from into to unless to already has an entry
// with that name, then removes from if it ended up empty. It returns the
// names it moved.
func (e *Engine) mergeDir(from, to string) ([]string, error) {
	if _, err := e.System.Lstat(from); errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	if err := e.System.MkdirAll(to, 0o755); err != nil {
		return nil, err
	}
	var entries []string
	for _, pattern := range []string{"*", ".*"} {
		matches, err := e.System.Glob(filepath.Join(from, pattern))
		if err != nil {
			return nil, err
		}
		entries = append(entries, matches...)
	}
	var moved []string
	for _, src := range entries {
		dst := filepath.Join(to, filepath.Base(src))
		if _, err := e.System.Lstat(dst); err == nil {
			continue // never overwrite
		} else if !errors.Is(err, fs.ErrNotExist) {
			return moved, err
		}
		if err := e.System.Rename(src, dst); err != nil {
			return moved, err
		}
		moved = append(moved, filepath.Base(src))
	}
	_ = e.System.Remove(from) // only succeeds when empty; leftovers are reported by doctor
	return moved, nil
}

// rewriteStateKeys points managed-file entries recorded under the old config
// directory at the new one, so those files keep being recognized as
// unedited and are rewritten without backups.
func (e *Engine) rewriteStateKeys(oldConfigDir string) error {
	if _, err := e.System.Lstat(e.Paths.StateFile); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	st, err := LoadState(e.System, e.Paths.StateFile)
	if err != nil {
		return err
	}
	prefix := oldConfigDir + string(filepath.Separator)
	changed := false
	files := make(map[string]string, len(st.Files))
	for path, hash := range st.Files {
		if rest, ok := strings.CutPrefix(path, prefix); ok {
			path = filepath.Join(e.Paths.ConfigDir, rest)
			changed = true
		}
		files[path] = hash
	}
	if !changed {
		return nil
	}
	st.Files = files
	return st.Save(e.System, e.Paths.StateFile)
}

// oldInstallLeftovers lists shell-setup paths still present, for doctor.
func (e *Engine) oldInstallLeftovers() []string {
	old := oldInstallPaths(e.Paths.Home)
	var left []string
	for _, p := range []string{old.ConfigDir, old.StateDir, old.Binary} {
		if _, err := e.System.Lstat(p); err == nil {
			left = append(left, p)
		}
	}
	if info, err := e.System.Lstat(old.Alias); err == nil && info.Mode()&fs.ModeSymlink != 0 {
		left = append(left, old.Alias)
	}
	return left
}
```

En `internal/shellmanager/shellmanager.go`, dentro de `apply`, inserta entre `checkOS` y `Lock`:

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

En `internal/shellmanager/doctor.go`, justo antes de `return rep, nil`:

```go
	if left := e.oldInstallLeftovers(); len(left) > 0 {
		rep.Checks = append(rep.Checks, CheckResult{
			Name: "shell-setup leftovers", Status: CheckWarn,
			Detail: strings.Join(left, ", ") + " (run shm setup)",
		})
	}
```

Si hace falta, añade `"strings"` a los imports de `doctor.go`.

En `internal/cmd/bindings.go`, `resultStatus` pasa a ser:

```go
	case shellmanager.ResultOK, shellmanager.ResultRemoved, shellmanager.ResultMigrated:
		return styles.StatusOK
```

- [ ] **Step 3: Ejecutar los tests**

Run: `mise exec -- go test ./internal/shellmanager/ -run 'Migrate|AfterMigration|ShellSetupLeftovers' -v 2>&1 | grep -E '^(=== RUN|--- |ok|FAIL)'`
Expected: los 8 tests en PASS.

Run: `mise exec -- task lint test`
Expected: `0 issues.` y todo en verde.

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "Migrate shell-setup v0.1.x installs on setup and update"
```

---

### Task 4: `install.sh`: nombre nuevo, enlace `shm` y pregunta final

**Files:**
- Modify: `install.sh` y `tests/test_install.sh` (se reescriben enteros).

**Interfaces:**
- Produces: funciones bash `check_os`, `detect_arch`, `install_binary <arch>`, `link_alias`, `ask_setup` (devuelve 0 = sí) y `main`. Variables `SHELL_MANAGER_BIN_DIR`, `SHELL_MANAGER_STATE_DIR`, `SHELL_MANAGER_OS_RELEASE`, `SHELL_MANAGER_RELEASE_BASE` y `SHELL_MANAGER_TTY`.

- [ ] **Step 1: Tests que fallan**

`tests/test_install.sh` (contenido completo):

```bash
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
```

Run: `bash tests/test_install.sh 2>&1 | tail -3`
Expected: FAIL. El `install.sh` actual usa nombres y variables `shell-setup`.

- [ ] **Step 2: Implementación**

`install.sh` (contenido completo):

```bash
#!/usr/bin/env bash
# shell-manager bootstrap: installs the shell-manager binary (and the shm
# alias) from the latest GitHub release, checksum verified, then offers to
# run `shm setup`.
#   curl -fsSL https://raw.githubusercontent.com/BlasterM2A/shell-manager/master/install.sh | bash
set -euo pipefail

REPO="BlasterM2A/shell-manager"
BIN_DIR="${SHELL_MANAGER_BIN_DIR:-$HOME/.local/bin}"
STATE_DIR="${SHELL_MANAGER_STATE_DIR:-$HOME/.local/state/shell-manager}"
OS_RELEASE="${SHELL_MANAGER_OS_RELEASE:-/etc/os-release}"
RELEASE_BASE="${SHELL_MANAGER_RELEASE_BASE:-https://github.com/$REPO/releases/latest/download}"
TTY="${SHELL_MANAGER_TTY:-/dev/tty}"
STAMP_FILE="$STATE_DIR/bootstrap.sha256"

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
    mkdir -p "$BIN_DIR" "$STATE_DIR"
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
```

- [ ] **Step 3: Ejecutar los tests**

Run: `bash -n install.sh && bash tests/test_install.sh 2>&1 | grep -E 'FAIL|passed|failed'`
Expected: `All tests passed` y ninguna línea `FAIL`.

- [ ] **Step 4: Commit**

```bash
git add install.sh tests/test_install.sh
git commit -m "Bootstrap shell-manager and ask before running setup"
```

---

### Task 5: Documentación

**Files:**
- Modify: `README.md` y `CLAUDE.md` (se reescriben enteros), y `docs/superpowers/specs/2026-10-10-shell-setup-cli-design.md` (solo la cabecera).

- [ ] **Step 1: `README.md` (contenido completo)**

````markdown
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
````

- [ ] **Step 2: `CLAUDE.md` (contenido completo)**

```markdown
# shell-manager

Go CLI/TUI (`shell-manager`, alias `shm`) that bootstraps and maintains a zsh
environment on Ubuntu/Debian (zsh, mise, starship, zoxide, fzf, antidote,
Nerd Font, AI CLIs). Distributed as a GitHub Release binary; `install.sh`
downloads it, links `shm` and asks whether to run `shm setup`.
See `README.md` for usage. Design: base spec
`docs/superpowers/specs/2026-10-10-shell-setup-cli-design.md` (written under
the old name) plus `docs/superpowers/specs/2026-10-11-shell-manager-redesign-design.md`,
which overrides names, the `setup` command, the installer prompt and the
shell-setup migration.

## Repo layout

- `main.go` → `internal/cmd` (cobra). `cmd` is the only integration layer: it
  reads flags, builds UI components and binds domain events to them. No
  business logic, no styles.
- `internal/shellmanager` — the whole domain in one package (chezmoi-style):
  `Engine` (`Setup`, `Update`, `Doctor`), tool manifests + catalog, backends
  (`apt`, `mise`, `script`, `archive`, `font`), zsh config composition,
  managed files + state, legacy purge, migration from shell-setup. All I/O
  goes through the `System` interface.
- `internal/shellmanager/registry/` (embedded) — `tools/<id>.toml` is one
  module (manifest) per tool; `catalog.toml` lists the ids that are
  installed — the only place that decides inclusion. `files/` holds the
  config files modules ship and the zsh profile.
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
```

- [ ] **Step 3: Cabecera de la spec base**

En `docs/superpowers/specs/2026-10-10-shell-setup-cli-design.md`, justo después de la línea `**Estado:** …`, añade:

```markdown
**Nota (2026-10-11):** la herramienta se renombró a `shell-manager` (alias
`shm`) y `init` pasó a ser `setup`; ver
`2026-10-11-shell-manager-redesign-design.md`, que prevalece en nombres,
rutas, comandos, instalador y migración.
```

- [ ] **Step 4: Verificar y commit**

Run: `git grep -n -e 'shell-setup' -e '\bshs\b' -- README.md CLAUDE.md`
Expected: solo las menciones intencionadas, es decir, la sección "Migrating → From shell-setup" del README y la referencia al nombre del archivo de la spec base en CLAUDE.md.

Run: `mise exec -- task lint test`
Expected: todo en verde.

```bash
git add README.md CLAUDE.md docs/superpowers/specs/2026-10-10-shell-setup-cli-design.md
git commit -m "Document shell-manager: setup command, installer prompt, migration"
```

---

## Después del plan (lo hace el controlador, con confirmación del usuario)

Estos pasos son visibles fuera del repo, así que no van dentro de ninguna tarea:

1. PR de `feat/shell-manager` → CI → fusionar.
2. `gh repo rename shell-manager` y actualizar el remoto local (`git remote set-url origin https://github.com/BlasterM2A/shell-manager.git`).
3. Tag y release `v0.2.0`. Comprobar que la URL nueva del one-liner sirve el `install.sh` nuevo y que el asset `shell-manager_linux_amd64.tar.gz` pasa el checksum.
