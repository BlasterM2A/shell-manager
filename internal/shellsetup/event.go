package shellsetup

import "errors"

// ErrDependencyFailed marks a tool skipped because a dependency failed.
var ErrDependencyFailed = errors.New("dependency failed")

// Phase is a stage of Init/Update.
type Phase string

const (
	PhasePreflight Phase = "preflight"
	PhaseTools     Phase = "tools"
	PhaseShell     Phase = "shell"
)

// Action is what was done to a tool.
type Action string

const (
	ActionNone    Action = ""
	ActionInstall Action = "install"
	ActionUpdate  Action = "update"
)

// Result is the outcome for a tool or file.
type Result string

const (
	ResultOK       Result = "ok"
	ResultSkipped  Result = "skipped"
	ResultModified Result = "modified" // managed file edited by hand, left alone
	ResultWarned   Result = "warned"   // optional tool failed
	ResultFailed   Result = "failed"
	ResultRemoved  Result = "removed" // legacy leftover purged
)

// Event is published on the operation's channel while it runs.
type Event interface{ event() }

type PhaseStarted struct{ Phase Phase }

type ToolStarted struct {
	ID     string
	Action Action
}

type ToolFinished struct {
	ID      string
	Action  Action
	Result  Result
	Version string
	Err     error
}

type FileFinished struct {
	Path   string
	Result Result
}

func (PhaseStarted) event() {}
func (ToolStarted) event()  {}
func (ToolFinished) event() {}
func (FileFinished) event() {}
