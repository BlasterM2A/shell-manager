# shell-manager — rediseño: nombre, comando `setup` e instalación con pregunta

**Fecha:** 2026-10-11
**Estado:** Pendiente de revisión
**Base:** `2026-10-10-shell-setup-cli-design.md` (sigue siendo la referencia de
arquitectura, catálogo, backends, archivos gestionados, UI y testing). Este
documento solo define los cambios; donde la base diga `shell-setup`, `shs`,
`init` o `SHELL_SETUP_*`, rige lo de aquí.

## Objetivo

1. La herramienta y el repo pasan a llamarse **`shell-manager`**, con alias
   **`shm`**.
2. La configuración inicial pasa a ser el comando **`shm setup`** (sustituye a
   `init`).
3. `install.sh` solo instala la herramienta y, al terminar, **pregunta** si se
   quiere ejecutar `setup` en lugar de lanzarlo siempre.
4. Las instalaciones existentes de `shell-setup` (v0.1.x) **migran
   automáticamente** al ejecutar el nuevo instalador una vez.

**Éxito:** en una máquina con `shell-setup` v0.1.x, ejecutar el one-liner nuevo
y responder "Y" deja `shm` funcionando con la misma configuración y estado, sin
backups espurios de `~/.zshrc` ni restos de `shell-setup`/`shs`.

## Decisiones

| Tema | Decisión |
|---|---|
| Nombre | `shell-manager` (binario, repo `BlasterM2A/shell-manager`, módulo Go `github.com/BlasterM2A/shell-manager`, `project_name` de goreleaser). Comprobado: ni `shm` ni `shell-manager` existen como comando ni paquete apt en Ubuntu 26.04. |
| Alias | `shm` → `~/.local/bin/shm` enlace a `shell-manager` (mismas reglas que tenía `shs`: relativo junto al binario, nunca reemplaza un archivo real, `doctor` lo comprueba). |
| Paquete de dominio | `internal/shellsetup` → `internal/shellmanager` (convención de la base: el dominio lleva el nombre del producto). |
| Comandos | `setup` (antes `init`), `update`, `doctor`, `self-update`. `init` desaparece sin alias. |
| Rutas | `~/.config/shell-manager/` (config.toml, zshrc generado, `zsh.d/`, plugins.txt), `~/.local/state/shell-manager/` (state.toml, `shell-manager.log`, lock, tmp, `bootstrap.sha256`). `~/.zshrc` sigue siendo el stub, ahora con `source "$HOME/.config/shell-manager/zshrc"`. `~/.config/zsh/*.d` no cambia. |
| Variables de entorno | `SHELL_MANAGER_*` (antes `SHELL_SETUP_*`). En `install.sh`: `SHELL_MANAGER_BIN_DIR`, `SHELL_MANAGER_STATE_DIR`, `SHELL_MANAGER_OS_RELEASE`, `SHELL_MANAGER_RELEASE_BASE`, más `SHELL_MANAGER_TTY` (nuevo, ver abajo). |
| Assets de release | `shell-manager_linux_<amd64|arm64>.tar.gz` (binario `shell-manager` en la raíz) + `checksums.txt`. |
| Versión | `v0.2.0`. |
| Repo en GitHub | Se renombra con `gh repo rename shell-manager`. GitHub redirige las URLs antiguas (git, web, releases, raw), así que el one-liner viejo descarga el `install.sh` nuevo, que instala `shell-manager`. |

## `install.sh`

1. Igual que antes hasta instalar el binario: comprobar Debian/Ubuntu,
   arquitectura, descarga + `sha256sum`, instalación en
   `~/.local/bin/shell-manager` (salta la descarga si el stamp coincide).
2. Crea el enlace `~/.local/bin/shm → shell-manager` (no toca un archivo real
   llamado `shm`).
3. Pregunta final:
   - Si el terminal es usable (se puede abrir `$SHELL_MANAGER_TTY`, por
     defecto `/dev/tty`): muestra `Run 'shm setup' now? [Y/n] ` y lee una
     línea de ese terminal. Vacío, `y`, `Y`, `yes` → `exec shell-manager setup
     </dev/tty`. Cualquier otra respuesta → mensaje "Run `shm setup` whenever
     you're ready." y termina con 0.
   - Sin terminal (CI, pipe sin tty): no pregunta ni ejecuta; muestra el mismo
     mensaje y termina con 0.
   - El `--plain` automático del bootstrap anterior desaparece: sin terminal
     no se ejecuta `setup`.
4. Tests (`tests/test_install.sh`): respuesta vacía → ejecuta setup;
   `n` → no ejecuta; sin tty → no ejecuta; el enlace `shm` se crea y no pisa
   un archivo real. La ejecución de setup se comprueba con el binario falso
   del test (que registra sus argumentos), sin ejecutar nada real.

## Migración desde `shell-setup` (v0.1.x)

Paso nuevo en `setup` y `update`, después de comprobar el SO y **antes de
tomar el lock** (el lock vive ya en el directorio de estado nuevo). Es
idempotente y cada acción se publica como `FileFinished`:

| Si existe… | …y | Acción | Resultado |
|---|---|---|---|
| `~/.config/shell-setup/` | no existe `~/.config/shell-manager/` | renombrar al nuevo | `migrated` |
| `~/.local/state/shell-setup/` | no existe `~/.local/state/shell-manager/` | renombrar al nuevo; renombrar `shell-setup.log` → `shell-manager.log`; en `state.toml`, reescribir las claves de `files` (rutas absolutas) que empiecen por `$HOME/.config/shell-setup/` al prefijo `$HOME/.config/shell-manager/` | `migrated` |
| `~/.local/bin/shell-setup` | — | borrar | `removed` |
| `~/.local/bin/shs` | es un enlace (un archivo real no se toca) | borrar | `removed` |
| directorio antiguo | el nuevo **ya** existe | no se toca (no se mezclan); `doctor` lo avisa | — |

Tras la migración, el flujo normal reescribe el stub de `~/.zshrc`, el `zshrc`
generado y los fragmentos cuyo contenido cambia (rutas nuevas). Como el state
migrado los reconoce como gestionados y sin editar, se sobrescriben **sin
backup**; uno editado a mano sigue la regla de siempre (no se toca sin
`--force`).

`doctor` añade un check `shell-setup leftovers` (warn) si sigue existiendo
alguna ruta de la tabla, con el detalle "run shm setup".

## Fuera de alcance

- Assets "puente" con el nombre antiguo para que `shs self-update` de v0.1.x
  encuentre la release nueva: se migra reinstalando con el one-liner.
- Alias `init` o `shs` por compatibilidad.
- Renombrar el directorio local del repo del usuario (`~/Workspace/shell-setup`):
  queda a su criterio.

## Testing

- Migración (dominio): sobre un HOME temporal con la estructura de v0.1.x —
  directorios renombrados, claves de `state.toml` reescritas, binario y enlace
  `shs` borrados, archivo real `shs` respetado, directorio antiguo ignorado si
  el nuevo existe, idempotencia (segunda ejecución sin acciones), y un `setup`
  completo tras migrar que reescribe el stub **sin** crear `.bak`.
- `doctor`: aviso de restos de `shell-setup`.
- CLI: `setup` registrado, `init` ya no existe; mensajes con `shm setup`.
- Bootstrap: casos de la sección `install.sh`.
- e2e (`testscript`): `version.txtar` con `shell-manager`, `doctor.txtar` sin
  cambios de comportamiento.
- Como siempre: sin red, sin instalaciones reales, sin Docker.
