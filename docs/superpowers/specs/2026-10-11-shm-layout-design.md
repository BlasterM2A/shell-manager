# shell-manager — layout: `~/.shm`, `~/.zsh` y bloques en los dotfiles

**Fecha:** 2026-10-11
**Estado:** Pendiente de revisión
**Base:** `2026-10-10-shell-setup-cli-design.md` y
`2026-10-11-shell-manager-redesign-design.md`. Este documento solo define los
cambios; en rutas, composición de la config de zsh, archivos gestionados y
migración rige lo de aquí sobre ambos.

## Filosofía

shm es una TUI para facilitar la gestión de la configuración del shell. Hay
dos clases de archivos y cada una tiene su sitio:

1. **Lo propio de shm** (su config, estado, log, lock, temporales) vive en
   `~/.shm`. El binario sigue en `~/.local/bin`.
2. **La config de herramientas de terceros que shm genera** sigue la ruta que
   recomienda cada herramienta; si no recomienda ninguna, la más normalizada.
   Para zsh: en `~/.zshenv`/`~/.zshrc` solo las líneas mínimas para cargar el
   resto, y el resto en `~/.zsh/`, con una estructura autodescubierta en la
   que añadir o quitar un elemento es añadir o quitar un archivo.

**Éxito:** tras `shm setup` en un `$HOME` limpio, `~/.config/shell-manager`,
`~/.local/state/shell-manager` y `~/.config/zsh` no existen; un terminal
interactivo tiene prompt, plugins y herramientas; `zsh -c 'command -v
starship'` (como lo invocan scripts y agentes de IA) encuentra las
herramientas de mise; y añadir `~/.zsh/aliases/git.zsh` o
`~/.zsh/functions/mkcd` no requiere tocar ningún otro archivo.

## Decisiones

| Tema | Decisión |
|---|---|
| Datos de shm | `~/.shm/` plano: `config.toml` (del usuario, opcional), `state.toml`, `shm.log`, `lock`, `bootstrap.sha256` (install.sh), `tmp/`. |
| Binario | Sin cambios: `~/.local/bin/shell-manager` + enlace `~/.local/bin/shm`. |
| `~/.zshenv`, `~/.zshrc` | Del usuario. shm solo mantiene **su bloque con marcadores**, insertado arriba. |
| Config de zsh | `~/.zsh/`, por categoría (ver abajo). Conviven archivos de shm y del usuario; shm solo toca los registrados en `state.toml`. |
| antidote | Plugins en `~/.zsh_plugins.txt` (ruta por defecto de antidote); `antidote load` sin argumentos. |
| starship | Sin cambios: `~/.config/starship.toml`. |
| Migración | **Ninguna.** Se elimina la migración desde shell-setup y el purge de restos del install.sh original. Solo se ha usado en la máquina del autor, que se limpia a mano. |
| Desaparecen | `~/.config/shell-manager/`, `~/.local/state/shell-manager/`, `~/.config/zsh/*.d`, el stub de `~/.zshrc`. |
| Versión | `v0.3.0` (incompatible con v0.2.0). |

## Estructura

```
~/.local/bin/shell-manager      binario
~/.local/bin/shm  → shell-manager

~/.shm/                         config.toml, state.toml, shm.log, lock,
                                bootstrap.sha256, tmp/

~/.zshenv                       bloque shm (todas las invocaciones de zsh)
~/.zshrc                        bloque shm (solo shells interactivas)
~/.zsh/
├── env.zsh                     (shm) carga env/
├── init.zsh                    (shm) capa interactiva
├── env/                        variables y PATH; se carga SIEMPRE
│   ├── 00-path.zsh             (shm) ~/.local/bin
│   └── NN-<tool>.zsh           (shm) parte "env" de un módulo (p. ej. shims de mise)
├── functions/                  usuario: 1 archivo = 1 función (autoload)
├── completions/                usuario: _comando (fpath, antes de compinit)
├── tools/                      solo interactivo
│   ├── 20-core.zsh             (shm) compinit, historial
│   └── NN-<tool>.zsh           (shm) parte "snippet" de un módulo
├── aliases/
│   └── 00-base.zsh             (shm) ll, la, l
└── local/                      usuario: lo último que se carga (overrides)

~/.zsh_plugins.txt              antidote
~/.config/starship.toml         starship
```

shm crea todos los directorios de `~/.zsh/` (vacíos si no tienen archivos
suyos) para que se vea dónde va cada cosa. Dentro de cada directorio el orden
es alfabético; la convención es el prefijo `NN-`.

## Arranque de zsh

Requisito: funcionar a la vez en terminales interactivos y en consumidores no
interactivos (agentes de IA, scripts, IDEs, `ssh host cmd`).

| Invocación | Lee | Resultado |
|---|---|---|
| `zsh -c cmd` (scripts, agentes) | `~/.zshenv` | `env/`: PATH y shims de mise |
| `zsh -lc cmd` | `~/.zshenv`, `~/.zprofile` | igual |
| terminal interactivo | `~/.zshenv`, `~/.zshrc` | `env/` + capa interactiva completa |

Fundamentos (investigados en el brainstorming, con fuentes):
- Toda invocación de zsh lee `~/.zshenv`; solo las interactivas leen
  `~/.zshrc` (https://zsh.sourceforge.io/Doc/Release/Files.html).
- El `/etc/zsh/zshrc` de Ubuntu ejecuta `compinit` salvo que
  `skip_global_compinit` esté definido en `~/.zshenv`; sin él compinit corre
  dos veces (https://www.zsh.org/mla/users/2021/msg00879.html). Inofensivo en
  Debian, que no lo hace.
- mise recomienda `activate` para shells interactivas y shims para el resto;
  ambos conviven (https://mise.jdx.dev/dev-tools/shims.html).
- Claude Code lee `~/.zshrc` y captura alias/funciones en un snapshot que
  pierde las funciones auxiliares con prefijo `_`; con zoxide, `alias cd=z`
  rompe `cd` (`__zoxide_z` no encontrada)
  (https://code.claude.com/docs/en/tools-reference, issue #9633). Por eso no
  se define `alias cd=z`.
- En Ubuntu `/etc/zsh/zprofile` está vacío, así que nada resetea el PATH
  fijado en `~/.zshenv` (comprobado en Ubuntu 26.04; no verificado en Debian).
- El arranque no imprime nada en ningún caso.

### Bloques

`~/.zshenv`:
```zsh
# >>> shm >>>
# Managed by shm; changes inside this block are overwritten. Config: ~/.zsh/
skip_global_compinit=1
[[ -r "$HOME/.zsh/env.zsh" ]] && source "$HOME/.zsh/env.zsh"
# <<< shm <<<
```

`~/.zshrc`:
```zsh
# >>> shm >>>
# Managed by shm; changes inside this block are overwritten. Config: ~/.zsh/
[[ -r "$HOME/.zsh/init.zsh" ]] && source "$HOME/.zsh/init.zsh"
# <<< shm <<<
```

### `~/.zsh/env.zsh`

```zsh
# Managed by shm; changes here are overwritten by `shm update`.
# Your environment goes in ~/.zsh/env/*.zsh (loaded by every zsh, keep it quiet).
[[ -n $_SHM_ENV_LOADED ]] && return
_SHM_ENV_LOADED=1
typeset -U path
for _shm_f in "$HOME"/.zsh/env/*.zsh(N); do source "$_shm_f"; done
unset _shm_f
```

La guarda evita cargarlo dos veces en la misma shell (`init.zsh` también lo
carga, por si el bloque de `~/.zshenv` falta). `typeset -U path` evita
duplicados en shells anidadas.

`env/00-path.zsh`: `path=("$HOME/.local/bin" $path)`.

### `~/.zsh/init.zsh`

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

Orden: env → functions/completions → tools (compinit en `20-core`) → aliases
→ local. Los archivos se cargan a nivel superior, no dentro de una función:
dentro de una, un `typeset`/`declare` sin `-g` de un archivo del usuario o de
un plugin quedaría local y se perdería.

## Archivos y bloques gestionados

### Bloques (nuevo)

Un bloque es el texto entre `# >>> shm >>>` y `# <<< shm <<<` (marcadores
incluidos). `state.toml` guarda su hash en un mapa nuevo `blocks` (ruta →
hash del bloque), separado de `files`. Lo de fuera del bloque nunca cuenta
como modificación.

| Situación | `setup`/`update` |
|---|---|
| El archivo no existe | Lo crea con solo el bloque. |
| Existe sin bloque | Si no está vacío, backup `<archivo>.bak.<timestamp>`; inserta el bloque arriba y conserva el resto. |
| Bloque igual al deseado | Nada (resultado `ok`). |
| Bloque = lo que shm escribió, pero la plantilla cambió | Lo reescribe en su sitio. |
| Bloque editado a mano (hash ≠ registrado) | Lo deja (`modified`); con `--force`, backup del archivo y lo reescribe. |
| Marcadores rotos (falta uno, duplicados, fin antes de inicio) | No toca el archivo; error claro. |

`doctor` añade un check por bloque: `ok`, `missing (run shm update)`,
`modified by hand (update --force overwrites it)` o `broken markers`.

### Archivos completos

Siguen la política actual (`fileWriter`): `~/.zsh/env.zsh`, `init.zsh`,
`env/00-path.zsh`, `tools/20-core.zsh`, `aliases/00-base.zsh`, los fragmentos
`env/` y `tools/` de cada módulo y los `files` de los módulos
(`~/.zsh_plugins.txt`, `~/.config/starship.toml`).

Huérfanos: un fragmento gestionado en `env/` o `tools/` de una herramienta
que ya no está en el catálogo se borra (salvo si se editó), igual que hoy en
`zsh.d`.

## Módulos

`ShellSpec` gana un campo opcional `env`:

```toml
[shell]
priority = 10
env = '...'       # → ~/.zsh/env/NN-<id>.zsh (todas las shells; sin salida)
snippet = '...'   # → ~/.zsh/tools/NN-<id>.zsh (solo interactivo)
```

Al menos uno de `env`/`snippet` debe estar presente. `priority` (1..89) da el
`NN` en ambos directorios.

| Módulo | Cambio |
|---|---|
| mise | `env = 'path=("${XDG_DATA_HOME:-$HOME/.local/share}/mise/shims" $path)'`; `snippet` sigue con `mise activate zsh`. |
| antidote | `snippet`: `source ~/.antidote/antidote.zsh` + `antidote load`; `files.dest = "~/.zsh_plugins.txt"`. |
| starship | Guarda `[[ $TERM == dumb ]] && return` antes de `starship init`. |
| fzf | Solo si `[[ -t 0 && $TERM != dumb ]]`. |
| zoxide | Sin `alias cd="z"` (se usa `z`). |
| starship, zoxide, fzf, nerdfont | Se elimina `[legacy]`. |

Las plantillas propias pasan de `registry/files/profile/` a
`registry/files/zsh/` (`zshenv-block`, `zshrc-block`, `env.zsh`, `init.zsh`,
`env/00-path.zsh`, `tools/20-core.zsh`, `aliases/00-base.zsh`).

## Código

- `paths.go`: `ShmDir`, `StateFile`, `LockFile`, `LogFile` (`shm.log`),
  `TmpDir` bajo `~/.shm`; `Zshenv`, `Zshrc`, `ZshDir` y sus subdirectorios.
  Desaparecen `ConfigDir`, `ZshD`, `GeneratedZshrc`, `Stub`, `UserZshDir`.
- `config.go`: `~/.shm/config.toml`.
- `managed.go`: `writeBlock(path, block, force)` con la política de la tabla.
- `state.go`: mapa `Blocks`.
- `shellrc.go`: compone los dos bloques, los archivos de `~/.zsh/`, los
  fragmentos `env`/`tools` y los huérfanos de ambos directorios.
- `doctor.go`: checks de bloques; sin checks de restos.
- Se eliminan `migrate.go`, `legacy.go` (y tests), `LegacySpec`, las llamadas
  a migración y purge en `Engine`, y `ResultMigrated`/`ResultRemoved` si nada
  más los usa (también en `internal/cmd/bindings.go`).
- `install.sh`: `SHM_DIR` (por defecto `~/.shm`) sustituye a
  `SHELL_MANAGER_STATE_DIR`; el stamp es `$SHM_DIR/bootstrap.sha256`.
  `tests/test_install.sh` se actualiza.

## Testing

- `writeBlock`: un caso por fila de la tabla de bloques, incluido `--force`
  con backup y marcadores rotos.
- `shellWriter`: rutas nuevas, fragmentos `env`/`tools` desde módulos,
  huérfanos en ambos directorios, directorios de usuario creados.
- Catálogo: todo módulo tiene `env` o `snippet` si declara `[shell]`.
- `doctor`: estados de los bloques.
- `task test` y `task lint`. Manual solo con `HOME=$(mktemp -d)`: `doctor` y,
  sobre archivos generados, `zsh -c 'print -l $path'` (shims presentes) y
  `zsh -ic` (sin salida en el arranque).
- Durante la implementación se verifica lo que el brainstorming dejó sin
  confirmar: que `mise activate` convive con las shims en el PATH.

## Documentación

`README.md`: nueva sección "Where things live" con `~/.shm` y `~/.zsh/`
(cómo añadir alias, funciones, completions, overrides); se elimina
"Migrating". `CLAUDE.md`: rutas y referencia a este spec.

## Esta máquina

Tras publicar v0.3.0, limpieza manual (lista exacta revisada con el autor
antes de borrar): `~/.config/shell-setup`, `~/.local/state/shell-setup`,
`~/.local/bin/shell-setup`, `~/.local/bin/shs`, `~/.config/zsh` (vacío) y el
stub de `~/.zshrc` (con backup). Después, instalación desde cero con
`install.sh`.
