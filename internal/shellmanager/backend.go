package shellmanager

import "context"

// Env is what backends work with.
type Env struct {
	System  System
	Fetcher Fetcher
	Paths   Paths
	State   *State
}

// Backend installs and updates tools of one kind. Whether a tool is
// installed is decided by its check (plus InstallDetector, if implemented).
type Backend interface {
	Install(ctx context.Context, env Env, t Tool) error
	Update(ctx context.Context, env Env, t Tool) error
}

// InstallDetector is implemented by backends that must also confirm a tool
// is theirs: a tool whose check passes counts as installed only if
// Installed also reports true (e.g. a copy outside mise does not count).
type InstallDetector interface {
	Installed(ctx context.Context, env Env, t Tool) bool
}

// DefaultBackends maps manifest backend names to implementations.
func DefaultBackends() map[string]Backend {
	return map[string]Backend{
		"apt":     aptBackend{},
		"mise":    miseBackend{},
		"script":  scriptBackend{},
		"archive": archiveBackend{},
		"font":    fontBackend{},
	}
}
