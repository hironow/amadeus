package amadeus

import "fmt"

// ExecuteCheckCommand represents the intent to run an amadeus check.
// Independent of cobra — framework concerns are separated at the cmd layer.
type ExecuteCheckCommand struct {
	RepoPath string
}

// Validate checks that the command has valid required fields.
func (c *ExecuteCheckCommand) Validate() []error {
	var errs []error
	if c.RepoPath == "" {
		errs = append(errs, fmt.Errorf("RepoPath is required"))
	}
	return errs
}
