package shellsetup

// ToolReport is the outcome for one tool.
type ToolReport struct {
	ID       string
	Optional bool
	Action   Action
	Result   Result
	Version  string
	Err      error
}

// FileReport is the outcome for one managed file.
type FileReport struct {
	Path   string
	Result Result
}

// CheckStatus is a doctor check outcome.
type CheckStatus string

const (
	CheckOK   CheckStatus = "ok"
	CheckWarn CheckStatus = "warn"
	CheckFail CheckStatus = "fail"
	CheckSkip CheckStatus = "skip"
)

// CheckResult is one doctor check.
type CheckResult struct {
	Name   string
	Status CheckStatus
	Detail string
}

// Report is what an operation returns.
type Report struct {
	Tools  []ToolReport
	Files  []FileReport
	Checks []CheckResult
	// ShellChanged is set when this run changed the login shell to zsh.
	ShellChanged bool
}

// Failed reports whether a required tool failed or a check failed.
func (r Report) Failed() bool {
	for _, t := range r.Tools {
		if t.Result == ResultFailed {
			return true
		}
	}
	for _, c := range r.Checks {
		if c.Status == CheckFail {
			return true
		}
	}
	return false
}
