package shellmanager

// Kind separates the tool's own runtime dependencies from user tools.
type Kind string

const (
	// KindBuiltin tools are always installed and never user-selectable.
	KindBuiltin Kind = "builtin"
	// KindPlugin tools are the user's third-party tools.
	KindPlugin Kind = "plugin"
)

// Tool is one third-party tool, loaded from registry/<id>.toml.
type Tool struct {
	ID       string   `toml:"id"`
	Kind     Kind     `toml:"kind"`
	Optional bool     `toml:"optional"` // a failure is a warning, not an error
	Depends  []string `toml:"depends"`
	// SystemPackages are apt packages installed in the preflight.
	SystemPackages []string `toml:"system_packages"`
	// PostInstall commands run after every successful Install or Update.
	PostInstall [][]string  `toml:"post_install"`
	Install     InstallSpec `toml:"install"`
	Check       CheckSpec   `toml:"check"`
	Shell       *ShellSpec  `toml:"shell"`
	Files       []FileSpec  `toml:"files"`
}

// InstallSpec selects a backend and its parameters.
type InstallSpec struct {
	Backend     string   `toml:"backend"`     // apt | mise | script | archive | font
	Package     string   `toml:"package"`     // apt, mise
	URL         string   `toml:"url"`         // script: official installer
	Interpreter string   `toml:"interpreter"` // script: default "bash"
	UpdateCmd   []string `toml:"update_cmd"`  // script: tool's own updater
	Repo        string   `toml:"repo"`        // archive, font: GitHub owner/name
	Asset       string   `toml:"asset"`       // font: release asset name
	Dest        string   `toml:"dest"`        // archive, font: install directory
}

// CheckSpec tells whether a tool is installed: exactly one of Cmd or Path.
type CheckSpec struct {
	Cmd  []string `toml:"cmd"`  // installed if it exits 0; version = first output line
	Path string   `toml:"path"` // installed if it exists; version comes from state
}

// ShellSpec is the tool's fragment of the zsh config.
type ShellSpec struct {
	Priority int    `toml:"priority"` // 1..89, sets the load order in zsh.d
	Snippet  string `toml:"snippet"`
}

// FileSpec is a config file the tool ships, embedded under registry/.
type FileSpec struct {
	Src  string `toml:"src"`
	Dest string `toml:"dest"`
}
