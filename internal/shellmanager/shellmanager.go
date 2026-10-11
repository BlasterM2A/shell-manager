package shellmanager

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// UpdateOptions tune Update.
type UpdateOptions struct {
	// Force overwrites managed files edited by hand (after a backup).
	Force bool
}

// Engine runs the shell-manager operations.
type Engine struct {
	System   System
	Fetcher  Fetcher
	Catalog  *Catalog
	Backends map[string]Backend
	Paths    Paths
	// Sudo prefixes privileged commands; nil when already running as root.
	Sudo      []string
	OSRelease string
	Now       func() time.Time
	// Executable is the running shell-manager binary, the target of the
	// AliasName link; empty skips the link.
	Executable string
}

// NewEngine returns an Engine over the embedded catalog.
func NewEngine(sys System, fetcher Fetcher) (*Engine, error) {
	cat, err := DefaultCatalog()
	if err != nil {
		return nil, err
	}
	e := &Engine{
		System:    sys,
		Fetcher:   fetcher,
		Catalog:   cat,
		Backends:  DefaultBackends(),
		Paths:     NewPaths(sys.HomeDir()),
		Sudo:      []string{"sudo"},
		OSRelease: "/etc/os-release",
		Now:       time.Now,
	}
	for _, t := range cat.Tools() {
		if _, ok := e.Backends[t.Install.Backend]; !ok {
			return nil, fmt.Errorf("tool %s: unknown backend %q", t.ID, t.Install.Backend)
		}
	}
	return e, nil
}

// Setup installs what is missing and writes the shell config. It does not
// update tools that are already installed.
func (e *Engine) Setup(ctx context.Context, events chan<- Event) (Report, error) {
	return e.apply(ctx, modeSetup, false, events)
}

// Update is Setup plus updating every installed tool.
func (e *Engine) Update(ctx context.Context, opts UpdateOptions, events chan<- Event) (Report, error) {
	return e.apply(ctx, modeUpdate, opts.Force, events)
}

// ApplyConfig rewrites the shell config for the tools already installed. It
// installs and updates nothing, needs no sudo and leaves the login shell
// alone: it is what a new shell-manager binary runs to apply its own config
// templates.
func (e *Engine) ApplyConfig(ctx context.Context, opts UpdateOptions, events chan<- Event) (Report, error) {
	return e.apply(ctx, modeConfig, opts.Force, events)
}

type mode int

const (
	modeSetup mode = iota
	modeUpdate
	modeConfig
)

func (e *Engine) apply(ctx context.Context, m mode, force bool, events chan<- Event) (rep Report, err error) {
	defer close(events)

	events <- PhaseStarted{Phase: PhasePreflight}
	if err := e.checkOS(); err != nil {
		return rep, err
	}
	unlock, err := e.System.Lock(e.Paths.LockFile)
	if err != nil {
		return rep, err
	}
	defer unlock()
	state, err := LoadState(e.System, e.Paths.StateFile)
	if err != nil {
		return rep, err
	}
	defer func() {
		if saveErr := state.Save(e.System, e.Paths.StateFile); saveErr != nil && err == nil {
			err = saveErr
		}
	}()
	tools := e.Catalog.Tools()
	if m != modeConfig {
		if err := e.ensureAptPackages(ctx, aptPackages(tools), m == modeUpdate); err != nil {
			return rep, fmt.Errorf("installing system packages: %w", err)
		}
	}

	events <- PhaseStarted{Phase: PhaseTools}
	env := Env{System: e.System, Fetcher: e.Fetcher, Paths: e.Paths, State: state}
	blocked := map[string]bool{}
	var okTools []Tool
	for _, t := range tools {
		if err := ctx.Err(); err != nil {
			return rep, err
		}
		tr := e.applyTool(ctx, env, t, m, blocked, events)
		events <- ToolFinished{ID: tr.ID, Action: tr.Action, Result: tr.Result, Version: tr.Version, Err: tr.Err}
		rep.Tools = append(rep.Tools, tr)
		if tr.Result == ResultOK {
			okTools = append(okTools, t)
		} else {
			blocked[t.ID] = true
		}
	}

	if err := ctx.Err(); err != nil {
		return rep, err
	}
	events <- PhaseStarted{Phase: PhaseShell}
	w := shellWriter{
		sys:     e.System,
		catalog: e.Catalog,
		paths:   e.Paths,
		files:   fileWriter{sys: e.System, state: state, now: e.Now},
	}
	files, shellErr := w.apply(okTools, force)
	for _, f := range files {
		events <- FileFinished(f)
	}
	rep.Files = append(rep.Files, files...)
	// A broken shm block in a user file fails on its own: the alias link and
	// the login shell are still set up, and the error is reported at the end.
	if shellErr != nil && !errors.Is(shellErr, ErrBrokenMarkers) {
		return rep, shellErr
	}
	if e.Executable != "" {
		alias, err := e.ensureAlias()
		events <- FileFinished(alias)
		rep.Files = append(rep.Files, alias)
		if err != nil {
			return rep, errors.Join(shellErr, fmt.Errorf("linking %s: %w", AliasName, err))
		}
	}
	if m == modeConfig {
		return rep, shellErr
	}
	if rep.ShellChanged, err = e.ensureDefaultShell(ctx); err != nil {
		return rep, errors.Join(shellErr, fmt.Errorf("changing the default shell: %w", err))
	}
	return rep, shellErr
}

func (e *Engine) applyTool(ctx context.Context, env Env, t Tool, m mode, blocked map[string]bool, events chan<- Event) ToolReport {
	tr := ToolReport{ID: t.ID, Optional: t.Optional}
	for _, dep := range t.Depends {
		if blocked[dep] {
			tr.Result = ResultSkipped
			tr.Err = fmt.Errorf("%w: %s", ErrDependencyFailed, dep)
			return tr
		}
	}
	st := e.checkTool(ctx, env, t)
	switch {
	case !st.Installed && m == modeConfig:
		tr.Result = ResultSkipped // not ours to install in this mode
		return tr
	case !st.Installed:
		tr.Action = ActionInstall
	case m == modeUpdate:
		tr.Action = ActionUpdate
	default:
		tr.Result, tr.Version = ResultOK, st.Version
		return tr
	}
	events <- ToolStarted{ID: t.ID, Action: tr.Action}
	err := e.runBackend(ctx, env, t, tr.Action)
	if err == nil {
		st = e.checkTool(ctx, env, t)
		if !st.Installed {
			err = fmt.Errorf("not detected after %s", tr.Action)
		}
	}
	if err != nil {
		tr.Result = ResultFailed
		if t.Optional {
			tr.Result = ResultWarned
		}
		tr.Err = fmt.Errorf("%s: %w", t.ID, err)
		return tr
	}
	tr.Result, tr.Version = ResultOK, st.Version
	return tr
}

func (e *Engine) runBackend(ctx context.Context, env Env, t Tool, action Action) error {
	b := e.Backends[t.Install.Backend]
	var err error
	if action == ActionInstall {
		err = b.Install(ctx, env, t)
	} else {
		err = b.Update(ctx, env, t)
	}
	if err != nil {
		return err
	}
	for _, argv := range t.PostInstall {
		if _, err := e.System.Run(ctx, Cmd{Name: argv[0], Args: argv[1:]}); err != nil {
			return err
		}
	}
	return nil
}

type toolStatus struct {
	Installed bool
	Version   string
}

// checkTool runs the tool's check and, when its backend implements
// InstallDetector, also requires the backend to recognize the tool.
func (e *Engine) checkTool(ctx context.Context, env Env, t Tool) toolStatus {
	st := e.runCheck(ctx, t, env.State)
	if d, ok := e.Backends[t.Install.Backend].(InstallDetector); ok && st.Installed && !d.Installed(ctx, env, t) {
		return toolStatus{}
	}
	return st
}

func (e *Engine) runCheck(ctx context.Context, t Tool, state *State) toolStatus {
	if t.Check.Path != "" {
		if _, err := e.System.Stat(e.Paths.Expand(t.Check.Path)); err != nil {
			return toolStatus{}
		}
		return toolStatus{Installed: true, Version: state.Versions[t.ID]}
	}
	args := make([]string, 0, len(t.Check.Cmd)-1)
	for _, a := range t.Check.Cmd[1:] {
		args = append(args, e.Paths.Expand(a))
	}
	out, err := e.System.Run(ctx, Cmd{Name: t.Check.Cmd[0], Args: args})
	if err != nil {
		return toolStatus{}
	}
	first, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")
	return toolStatus{Installed: true, Version: first}
}

func (e *Engine) ensureAptPackages(ctx context.Context, pkgs []string, upgrade bool) error {
	targets := missingAptPackages(ctx, e.System, pkgs)
	if upgrade {
		targets = pkgs
	}
	if len(targets) == 0 {
		return nil
	}
	if _, err := e.System.Run(ctx, e.privileged("apt-get", "update", "-qq")); err != nil {
		return err
	}
	_, err := e.System.Run(ctx, e.privileged("apt-get", append([]string{"install", "-y"}, targets...)...))
	return err
}

// privileged builds an interactive command run through sudo (unless root).
func (e *Engine) privileged(name string, args ...string) Cmd {
	if len(e.Sudo) == 0 {
		return Cmd{Name: name, Args: args, Interactive: true}
	}
	full := append(append(slices.Clone(e.Sudo[1:]), name), args...)
	return Cmd{Name: e.Sudo[0], Args: full, Interactive: true}
}

func (e *Engine) checkOS() error {
	data, err := e.System.ReadFile(e.OSRelease)
	if err != nil {
		return fmt.Errorf("cannot detect the OS (%s): %w", e.OSRelease, err)
	}
	vals := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		if k, v, ok := strings.Cut(strings.TrimSpace(line), "="); ok {
			vals[k] = strings.Trim(v, `"`)
		}
	}
	for _, id := range append([]string{vals["ID"]}, strings.Fields(vals["ID_LIKE"])...) {
		if id == "debian" || id == "ubuntu" {
			return nil
		}
	}
	return fmt.Errorf("unsupported OS %q: shell-manager supports Debian/Ubuntu only", vals["PRETTY_NAME"])
}

// loginShell is the user's login shell from the passwd database ($SHELL
// only reflects it after the next login).
func (e *Engine) loginShell(ctx context.Context) string {
	out, err := e.System.Run(ctx, Cmd{Name: "getent", Args: []string{"passwd", e.System.Getenv("USER")}})
	if err == nil {
		if fields := strings.Split(strings.TrimSpace(string(out)), ":"); len(fields) >= 7 {
			return fields[6]
		}
	}
	return e.System.Getenv("SHELL")
}

// ensureDefaultShell makes zsh the login shell; changed reports whether it
// had to change it.
func (e *Engine) ensureDefaultShell(ctx context.Context) (changed bool, err error) {
	if filepath.Base(e.loginShell(ctx)) == "zsh" {
		return false, nil
	}
	zsh, err := e.System.LookPath("zsh")
	if err != nil {
		return false, err
	}
	if _, err := e.System.Run(ctx, Cmd{Name: "chsh", Args: []string{"-s", zsh}, Interactive: true}); err == nil {
		return true, nil
	}
	// chsh fails on accounts without a password (SSH-key-only); retry as root.
	if _, err := e.System.Run(ctx, e.privileged("chsh", "-s", zsh, e.System.Getenv("USER"))); err != nil {
		return false, err
	}
	return true, nil
}
