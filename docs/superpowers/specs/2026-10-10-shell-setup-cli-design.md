# shell-setup CLI — migración de script a herramienta CLI/TUI en Go

**Fecha:** 2026-10-10
**Estado:** Pendiente de revisión
**Sustituye a:** `2026-09-19-shell-setup-design.md` en distribución, arquitectura
y composición del `.zshrc`. Las decisiones de stack de shell (zsh, starship,
antidote, fzf, zoxide, mise, Nerd Font, AI CLIs) se mantienen.

## Objetivo

Dejar de distribuir un script de instalación de un solo uso (`install.sh`) y
distribuir una herramienta CLI/TUI instalada en `PATH` que cubra todo el ciclo
de vida del entorno de shell en cada máquina: setup inicial, actualización de
componentes, diagnóstico y auto-actualización. La modificación de
configuración desde la herramienta es una funcionalidad futura: el diseño debe
permitir añadirla sin reestructurar, pero no se implementa en v1.

**Éxito:** en una máquina nueva, el mismo one-liner de hoy instala el binario y
ejecuta el setup; a partir de ahí todo se hace con `shell-setup <comando>`, sin
volver a ejecutar `curl | bash`.

## Alcance v1

| Comando | Qué hace |
|---|---|
| `shell-setup init` | Instala lo que falte, escribe la configuración de shell, cambia el shell por defecto a zsh. No actualiza lo ya instalado. |
| `shell-setup update` | Igual que `init`, pero además actualiza las herramientas ya instaladas y reescribe la configuración gestionada que haya cambiado. `--force` sobrescribe archivos gestionados editados a mano (con backup). |
| `shell-setup doctor` | Solo lectura: estado y versión de cada herramienta, archivos gestionados (ok / modified / missing), shell por defecto, restos de instalaciones antiguas que no se pudieron purgar. Sale con código ≠ 0 si falta algo requerido. |
| `shell-setup self-update` | Descarga el último release del binario y se reemplaza. Indica ejecutar `shell-setup update` para aplicar la configuración nueva. |

**Fuera de alcance v1** (el diseño lo deja preparado, no se implementa):
comandos `uninstall` / `disable` / `enable`, edición de configuración desde la
herramienta, comando `diff` / `--dry-run`, manifiestos definidos por el usuario,
otros sistemas operativos distintos de Debian/Ubuntu.

## Decisiones

- **Lenguaje:** Go, un único módulo (`github.com/BlasterM2A/shell-setup`), un
  único binario. Toolchain fijada con mise (`mise.toml`) y tareas con
  Taskfile. Se priorizan librerías populares del ecosistema.
- **Configuración embebida:** los manifiestos de herramientas y los archivos de
  configuración que distribuye la herramienta (fragmentos de shell,
  `starship.toml`, `plugins.txt`) van dentro del binario (`go:embed`). Cada
  release es una versión coherente de código + configuración. Desaparece la
  descarga de dotfiles desde `raw.githubusercontent.com`.
- **Herramientas de usuario vía mise:** starship, zoxide y fzf se instalan con
  `mise use -g <tool>@latest` y se actualizan con `mise upgrade <tool>`.
- **Built-in vs plugin:** las dependencias que la herramienta necesita para
  funcionar (zsh, mise) son *built-in*: siempre se instalan, quedan disponibles
  para el usuario y no son seleccionables. El resto son *plugins*. Ambos
  comparten el mismo contrato; la diferencia es un dato del manifiesto.
- **`curl`, `unzip`, `git` dejan de ser dependencias:** Go descarga y
  descomprime de forma nativa; antidote se instala desde tarball.
- **AI CLIs:** instalador oficial de cada proveedor. Actualización con su
  comando propio si existe; si no, se re-ejecuta el instalador. Son opcionales:
  un fallo es un aviso.
- **Nerd Font:** se instala siempre (también en servidores). Desactivarla
  llegará con la funcionalidad de deshabilitar.
- **`alias cd="z"`:** se mantiene, dentro del fragmento de zoxide. El usuario
  puede anularlo con `unalias cd` en `~/.config/zsh/aliases.d/`.

## Estructura del repositorio

Cada directorio sigue a al menos una de las referencias analizadas
(charmbracelet/crush, twpayne/chezmoi, cli/cli) salvo las desviaciones
justificadas al final de esta sección.

```
main.go                     # solo llama a cmd.Execute() (crush, chezmoi)
internal/
├── cmd/                    # comandos cobra (crush, chezmoi)
│   ├── root.go             # lista explícita de comandos (gh, chezmoi)
│   ├── factory.go          # construye las dependencias reales (gh)
│   ├── init.go  update.go  doctor.go  selfupdate.go
│   └── testdata/scripts/   # e2e txtar (chezmoi)
├── shellsetup/             # dominio en un único paquete (chezmoi)
│   └── registry/           # catalog.toml + tools/*.toml + files/ (embebidos)
├── config/                 # intención del usuario (crush, gh)
├── log/                    # slog → archivo + consola (crush)
├── selfupdate/             # reemplazo del binario (crush/gh "update")
├── iostreams/              # salida sin TTY (gh)
└── ui/
    ├── styles/             # única fuente de estilo (crush)
    ├── common/             # Common{Styles, ancho, alto} (crush)
    ├── model/              # una vista por comando (crush)
    ├── steplist/           # progreso por herramienta
    ├── statustable/        # tabla de estado
    └── notice/             # mensajes info/warn/error
tests/test_install.sh       # test del bootstrap
install.sh                  # bootstrap
mise.toml  Taskfile.yml  .goreleaser.yml  .golangci.yml  go.mod
.github/workflows/          # ci.yml, release.yml
docs/
```

**Sin `pkg/`:** nada está pensado para ser importado por terceros; todo vive en
`internal/` (como crush y chezmoi; la guía oficial de go.dev no usa `pkg/`).

**Desviaciones justificadas:**
- `selfupdate/` en vez de `update/`: en este producto "update" es el comando que
  actualiza herramientas; se evita la ambigüedad.
- `shellsetup/` en vez de `chezmoi/`: mismo patrón (dominio con el nombre del
  producto), distinto producto.
- `install.sh` en la raíz: mantiene la URL del one-liner existente.
- Sin capa `app/` ni `pubsub/`: la orquestación vive en el dominio (ver
  "Dominio"); un canal de eventos por operación sustituye al broker de crush,
  que solo se justifica con varios servicios de larga vida publicando a la vez.

### Reglas de dependencia

Verificadas en CI con `depguard` (golangci-lint):

| Paquete | Puede importar | Nunca |
|---|---|---|
| `ui/styles` | lipgloss | nada interno |
| `ui/**` (resto) | `ui/styles`, `ui/common`, librerías charm | `shellsetup`, `cmd` |
| `iostreams` | `ui/styles` | `shellsetup`, `cmd` |
| `log` | `ui/styles` (handler de consola) | `shellsetup`, `cmd` |
| `shellsetup` | `config`, `log` | `ui`, `iostreams`, `cmd` |
| `selfupdate` | `log` | `ui`, `cmd` |
| `cmd` | todo | — |

`cmd` es la única capa de integración: lee flags, construye el componente de
UI y conecta los eventos del dominio con él. No contiene lógica de negocio ni
estilos.

## Dominio (`internal/shellsetup`)

Un único paquete organizado por archivos, siguiendo el patrón de
`internal/chezmoi` (interfaz + implementaciones como archivos hermanos):

```
shellsetup.go          # Engine: Init, Update, Doctor
event.go               # tipos de evento
tool.go                # Tool (manifiesto cargado) + Kind, Status
catalog.go             # carga catalog.toml y sus módulos (embed)
backend.go             # interfaz Backend
aptbackend.go  misebackend.go  scriptbackend.go
archivebackend.go  fontbackend.go
plan.go                # deseado vs real → lista de acciones
shellrc.go             # stub ~/.zshrc + zshrc generado + zsh.d
check.go               # checks del doctor
system.go              # interfaz para todo el I/O (fs + ejecutar comandos)
realsystem.go  dryrunsystem.go
state.go               # hechos persistidos
registry/              # catalog.toml, tools/*.toml, files/ (embebidos)
```

### API pública del dominio

```go
type Engine struct { /* System, Catalog, State, Config, Logger, HTTP */ }

func (e *Engine) Init(ctx context.Context, events chan<- Event) (Report, error)
func (e *Engine) Update(ctx context.Context, opts UpdateOptions, events chan<- Event) (Report, error)
func (e *Engine) Doctor(ctx context.Context) (Report, error)
```

El dominio nunca imprime: publica eventos y devuelve un `Report`. El canal se
crea por operación y lo cierra el Engine al terminar.

Eventos: `PhaseStarted{Phase}` (preflight, tools, shell), `ToolStarted{ID,
Action}`, `ToolFinished{ID, Result, Version, Err}`, `FileFinished{Path,
Result}`. `Result` ∈ ok / skipped / modified / warned / failed.

### Manifiesto de herramienta

Una herramienta = un módulo `registry/tools/<id>.toml`. Qué módulos se
instalan lo decide **solo** la lista de `registry/catalog.toml`
(`tools = ["zsh", "mise", …]`): incluir o retirar una herramienta es añadir
o quitar su id ahí, sin tocar código ni tests. Un módulo fuera del catálogo
se conserva en el codebase y se sigue validando como manifiesto; no deja
referencias fuera de su propio archivo (frente a una lista `disabled`, que
obligaría a mantenerlas). Las `depends` de un módulo incluido deben estar
incluidas. Añadir una herramienta cuyo backend ya existe no requiere
código Go.

```toml
id          = "starship"
kind        = "plugin"          # "builtin" | "plugin"
optional    = false             # true → un fallo es aviso, no error
depends     = ["mise"]
system_packages = []            # paquetes apt que necesita (preflight)

[install]
backend = "mise"                # apt | mise | script | archive | font
package = "starship"

[check]
cmd = ["starship", "--version"] # instalado si sale con 0; versión = 1ª línea

[shell]
priority = 50
snippet  = 'eval "$(starship init zsh)"'

[[files]]
src  = "files/starship.toml"
dest = "~/.config/starship.toml"
```

Campos por backend:
- `apt`: `package`. Instalación y actualización se agrupan en el preflight.
- `mise`: `package`. `mise use -g <package>@latest` / `mise upgrade <package>`.
- `script`: `url` del instalador oficial, `update_cmd` opcional.
- `archive`: `url` (tarball), `dest`. Actualizar = volver a descargar si cambió
  la versión publicada.
- `font`: `url` (zip de release), `dest`; ejecuta `fc-cache` tras extraer;
  guarda el tag instalado para no re-descargar si no cambió.

Campo común opcional: `post_install` (lista de comandos, cada uno como array
de argumentos) que se ejecuta tras `Install` y tras `Update`.

Sección opcional `[legacy]`: lo que instalaciones anteriores dejaron y esta
herramienta sustituye — `paths` (archivos; admite `~/` y comodines) y `apt`
(paquetes). Se purga en `init`/`update` solo cuando la herramienta ha
terminado OK con su backend en esa ejecución (nunca se deja al usuario sin
la herramienta); los paquetes con `sudo apt-get remove -y`, solo si dpkg
los da por instalados. Un fallo al purgar es un aviso, no un error; `doctor`
sigue mostrando el resto hasta que `update` lo consiga.

### Catálogo v1

| id | kind | backend | optional | depends | system_packages | shell (prioridad) | files |
|---|---|---|---|---|---|---|---|
| zsh | builtin | apt | no | — | — | — | — |
| mise | builtin | script (`mise.run`, update: `mise self-update`) | no | — | — | 10: `mise activate zsh` | — |
| antidote | plugin | archive | no | — | — | 30: source + `antidote load` | `plugins.txt` |
| starship | plugin | mise | no | mise | — | 50: `starship init zsh` | `starship.toml` |
| fzf | plugin | mise | no | mise | — | 60: `fzf --zsh` | — |
| zoxide | plugin | mise | no | mise | — | 60: `zoxide init zsh` + `alias cd="z"` | — |
| nerdfont | plugin | font | no | — | fontconfig | — | — |
| claude | plugin | script | sí | — | — | — | — |
| junie | plugin | script | sí | — | — | — | — |
| agy | plugin | script | sí | — | — | — | — |

Además, al instalar mise el Engine ejecuta `mise settings set auto_update true`
(comportamiento actual), declarado como
`post_install = [["mise", "settings", "set", "auto_update", "true"]]` en el
manifiesto de mise.

### Flujo de `Init` / `Update`

1. **Preflight** (antes de mostrar la TUI):
   - SO Debian/Ubuntu (`/etc/os-release`) y arquitectura amd64/arm64.
   - Lockfile en `~/.local/state/shell-setup/lock` (evita ejecuciones
     concurrentes).
   - Un único `sudo apt-get install` con los `system_packages` y paquetes de
     backend `apt` que falten (en `Update`, además `--only-upgrade` de los ya
     instalados).
2. **Herramientas**, en orden topológico por `depends` (built-ins primero):
   `Check` → si no está instalada, `Install`; si está y es `Update`, `Update`.
3. **Shell** (`shellrc.go`):
   - Escribe los `files` de cada herramienta que terminó bien.
   - Escribe `~/.config/shell-setup/zsh.d/NN-<id>.zsh` por cada herramienta
     con `shell.snippet` que terminó bien; borra fragmentos de herramientas que
     ya no estén en el catálogo.
   - Escribe `~/.config/shell-setup/zshrc` (esqueleto propio que carga los
     fragmentos y los directorios del usuario) y el stub `~/.zshrc`.
   - Crea `~/.config/zsh/{env,functions,aliases,custom}.d` si no existen; nunca
     toca su contenido.
   - Cambia el shell por defecto a zsh si no lo es (`chsh`, con fallback a
     `sudo chsh`). El `Report` indica si lo cambió (`ShellChanged`) y el
     mensaje final se adapta: si cambió, "Run `exec zsh` (or open a new
     terminal)…; if new terminals still open your old shell, log out and back
     in" (la mayoría de emuladores leen el shell de login de passwd al abrir
     ventana); si no, "Run `exec zsh` to load the new configuration".
4. Devuelve el `Report` (el comando muestra el resumen de doctor).

### Composición del `.zshrc`

`~/.zshrc` (stub, gestionado):

```zsh
# ~/.zshrc — managed by shell-setup. Personal config: ~/.config/zsh/*.d
source "$HOME/.config/shell-setup/zshrc"
```

`~/.config/shell-setup/zshrc` (generado) carga en este orden fijo:

| Prioridad | Contenido |
|---|---|
| 00 | PATH (`~/.local/bin`) + `~/.config/zsh/env.d/*.zsh` |
| 10 | mise |
| 20 | `compinit` + historial (esqueleto propio) |
| 30 | antidote |
| 50 | prompt |
| 60 | integraciones (fzf, zoxide) |
| 70 | aliases propios (`ll`, `la`, `l`) |
| 90 | `~/.config/zsh/{functions,aliases,custom}.d/*.zsh` |

Esto corrige el orden actual, en el que mise se activa después de comprobar
`command -v` de herramientas que ahora instala mise.

### Archivos gestionados y ediciones manuales

`state` guarda el hash de lo último que escribió la herramienta en cada
archivo gestionado (stub, zshrc generado, fragmentos, `files`).

| Situación | `init` / `update` | `update --force` | `doctor` |
|---|---|---|---|
| Sin hash en state y el archivo existe (primera vez) | backup `<archivo>.bak.<timestamp>` + escribe | igual | — |
| Hash en disco = hash en state | escribe si el contenido nuevo difiere | igual | ok |
| Hash en disco ≠ hash en state (editado a mano) | no toca; evento `modified` | backup + escribe | modified |
| No existe | escribe | escribe | missing |

Las escrituras son atómicas (archivo temporal + rename).

### Doctor

Lista de checks (patrón `doctorcmd.go` de chezmoi), cada uno con resultado ok /
warn / fail / skip:
- Por herramienta: `check.cmd` (instalada + versión).
- Por archivo gestionado: ok / modified / missing.
- Shell por defecto es zsh.
- Alias `shs`: enlace presente y apuntando a shell-setup; ausente, apuntando
  a otro sitio o archivo real → warn.
- Restos de instalaciones antiguas (sección `[legacy]` de cada módulo) que
  siguen presentes → warn ("run shs update to remove it").
- Versión del binario vs último release (warn si hay una más nueva).

Código de salida: 1 si algún check de una herramienta no opcional es fail.

### System (I/O)

Toda operación de sistema pasa por la interfaz `System` (patrón de chezmoi):
leer/escribir/renombrar archivos, crear directorios, ejecutar comandos.
- `realsystem.go`: implementación real. La ejecución de comandos acepta
  `Interactive bool`; los comandos interactivos (sudo, chsh) heredan la
  terminal.
- `dryrunsystem.go`: registra las operaciones sin ejecutarlas. En v1 se usa en
  tests; es la base de un futuro `--dry-run`.

### Errores

- Herramienta `optional = true` que falla → `warned`, código de salida 0.
- Herramienta no opcional que falla → `failed`, código de salida 1.
- Las dependientes de una herramienta fallida se marcan `skipped` (bloqueada
  por X); las independientes continúan. Su fragmento de shell no se escribe.
- Preflight fallido (SO no soportado, sudo denegado, lock tomado) → aborta
  antes de modificar nada.
- Ctrl-C → cancela el `context`; el subproceso en curso recibe la señal; no
  quedan archivos a medias gracias a la escritura atómica.
- Errores envueltos con `%w` y el id de la herramienta; nada se descarta en
  silencio; sin `panic`.
- Red: reintentos con backoff (go-retryablehttp); un HTTP ≠ 2xx es error, nunca
  contenido.

## Config y state

| | Ruta | Contenido v1 | Quién escribe |
|---|---|---|---|
| config | `~/.config/shell-setup/config.toml` | nivel de log. Punto de extensión de la futura "modificar configuración" (p. ej. herramientas deshabilitadas). | el usuario |
| state | `~/.local/state/shell-setup/state.toml` | `schema_version`, hashes de archivos gestionados, tag instalado de la Nerd Font | la herramienta |
| log | `~/.local/state/shell-setup/shell-setup.log` | log completo en texto plano, incluida la salida de subprocesos | la herramienta |

`config` se carga con koanf: valores por defecto → archivo → variables
`SHELL_SETUP_*` → flags. `state` incluye `schema_version`; al cargar una
versión antigua se migra.

## UI

- **`ui/styles`** es la única fuente de estilo: paleta semántica (texto
  primario, atenuado, éxito, aviso, error, acento), iconos (✓ • ! ✗) y estilos
  lipgloss. Ningún otro paquete define colores.
- **Componentes** (`steplist`, `statustable`, `notice`): un paquete cada uno
  bajo `ui/`. Reciben props (struct de entrada) y un `common.Common`; exponen
  mensajes tipados. Cada uno tiene render TTY y render plano.
- **`ui/model`**: una vista por comando (init, update, doctor) que compone los
  componentes.
- **`iostreams`**: detecta TTY y respeta `NO_COLOR`. Sin TTY (pipe, CI) o con
  `--plain`, los mismos componentes se renderizan en texto plano usando
  `ui/styles` sin color.
- **Logging**: `log/slog`. Mientras la TUI está activa, solo a archivo. En
  modo plano, además a consola con un handler que usa `ui/styles` (mismo
  formato que `notice`). `--verbose` / `--quiet`.

### Integración en `cmd/`

Patrón de gh: `newXCmd(f *Factory, runF func(*XOptions) error)`. `RunE` rellena
las opciones y llama a `runF` (tests) o `runX` (producción). `runX`:
1. Construye la vista (`ui/model`) con sus componentes.
2. Lanza la operación del Engine en una goroutine con un canal de eventos.
3. Reenvía cada evento a la TUI con `program.Send` (o a la vista plana).
4. Para comandos interactivos del dominio, `Factory` provee un `System` cuyo
   `Interactive` suspende la TUI con `tea.ExecProcess` y la reanuda.
5. Traduce el `Report` a código de salida.

Ilustrativo:

```go
func newUpdateCmd(f *Factory, runF func(*UpdateOptions) error) *cobra.Command {
    opts := &UpdateOptions{Factory: f}
    cmd := &cobra.Command{
        Use: "update",
        RunE: func(cmd *cobra.Command, _ []string) error {
            if runF != nil {
                return runF(opts)
            }
            return runUpdate(cmd.Context(), opts)
        },
    }
    cmd.Flags().BoolVar(&opts.Force, "force", false, "overwrite modified managed files")
    return cmd
}
```

## Self-update (`internal/selfupdate`)

- Usa `creativeprojects/go-selfupdate` contra los GitHub Releases del repo,
  validando `checksums.txt`.
- Compara con la versión inyectada por ldflags; si es la última, no hace nada.
- Reemplazo atómico del binario. Si la ruta no es escribible, error claro.
- Tras actualizar, indica ejecutar `shell-setup update`.
- El owner/repo (`BlasterM2A/shell-setup`) es una única constante inyectada por
  ldflags, compartida con goreleaser.

## Distribución

### Bootstrap (`install.sh`)

Se mantiene en la raíz con la misma URL. Pasa a ~40 líneas con una sola
responsabilidad:
1. Comprueba Debian/Ubuntu.
2. Detecta arquitectura (`x86_64` → `amd64`, `aarch64` → `arm64`).
3. Descarga `shell-setup_linux_<arch>.tar.gz` y `checksums.txt` del último
   release; verifica con `sha256sum`.
4. Instala en `~/.local/bin/shell-setup` (si ya está en la última versión, no
   re-descarga).
5. Crea el alias corto `~/.local/bin/shs → shell-setup` (enlace relativo).
   Un archivo real llamado `shs` nunca se reemplaza. `init`/`update` también
   aseguran el enlace (apuntando al binario en ejecución), por si el binario
   se instaló sin el bootstrap.
6. `exec shell-setup init </dev/tty` si `/dev/tty` existe; si no,
   `shell-setup init --plain`.

```bash
curl -fsSL https://raw.githubusercontent.com/BlasterM2A/shell-setup/master/install.sh | bash
```

### Releases y CI

- **goreleaser** (`.goreleaser.yml`): `CGO_ENABLED=0`, linux amd64/arm64,
  tar.gz + `checksums.txt`, ldflags con versión, commit y repo.
- Versionado semver por tag manual (`git tag vX.Y.Z && git push --tags`).
- **`ci.yml`** (push y PR): `task lint test`.
- **`release.yml`** (tag `v*`): tests + `goreleaser release`.

### Toolchain local

- `mise.toml`: `go`, `task`, `goreleaser`, `golangci-lint`.
- `Taskfile.yml`: `build`, `run -- <args>`, `test` (`go test ./...` +
  `bash tests/test_install.sh`), `lint`, `fmt`, `tidy`, `snapshot`
  (`goreleaser --snapshot --clean`).
- golangci-lint (con depguard) para Go. shellcheck y Docker siguen fuera de
  alcance.

## Librerías

| Uso | Librería |
|---|---|
| Comandos | `spf13/cobra` |
| TUI | `charmbracelet/bubbletea`, `bubbles`, `lipgloss` |
| Logging | `log/slog` + `charmbracelet/log` (handler) |
| Config | `knadh/koanf` |
| TOML | `pelletier/go-toml/v2` |
| HTTP | `hashicorp/go-retryablehttp` |
| Self-update | `creativeprojects/go-selfupdate` |
| Tests | `stretchr/testify`, `rogpeppe/go-internal/testscript`, `charmbracelet/x/exp/golden` |

## Testing

Sin red, sin instalaciones reales, sin Docker.

| Qué | Cómo |
|---|---|
| Dominio | unit tests con `dryrunsystem` y un `$HOME` en `t.TempDir` |
| Catálogo | reglas (inclusión, orden, errores) con un registro de prueba; el registro real se valida dinámicamente: lo que lista `catalog.toml` carga (backends existentes, `depends` incluidas, sin ciclos) y todo `tools/*.toml` es un manifiesto válido. Ningún test nombra ni cuenta herramientas |
| Orden y fallos | Engine con herramientas fake: orden topológico, skip de dependientes, optional vs requerido, eventos |
| Archivos gestionados | tabla de la sección "Archivos gestionados": cada fila es un caso |
| `.zshrc` y fragmentos | golden files |
| Componentes UI | golden files (render TTY y plano) |
| Comandos | `newXCmd(f, runF)` con `runF` fake: parseo de flags |
| E2E | `internal/cmd/testdata/scripts/*.txtar` con testscript sobre HOME falso y backends fake |
| Bootstrap | `tests/test_install.sh` reducido: mapeo de arquitectura, checksum incorrecto → aborta, idempotencia |

## Migración

- **Repo:** `zsh/zshrc`, `zsh/plugins.txt` y `starship/starship.toml` pasan a
  `internal/shellsetup/registry/files/` (el zshrc se divide en esqueleto +
  fragmentos por herramienta). `install.sh` y `tests/test_install.sh` se
  reescriben. Se actualizan `README.md` y `CLAUDE.md`.
- **Máquinas existentes:** ejecutar el one-liner una vez. `init` hace backup del
  `~/.zshrc` actual y escribe el stub (primera vez, sin hash en state). Los
  binarios instalados por el script anterior (`~/.local/bin/starship`,
  `~/.local/bin/zoxide`, `fzf` de apt) y la fuente descomprimida en
  `~/.local/share/fonts` se purgan automáticamente (sección `[legacy]` de
  cada módulo) en cuanto la nueva copia está instalada.
- A partir de ahí: `shell-setup self-update` y `shell-setup update`.
