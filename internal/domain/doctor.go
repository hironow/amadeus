package domain

// CheckStatus represents the outcome of a single doctor check.
type CheckStatus int

const (
	CheckOK CheckStatus = iota
	CheckFail
	CheckSkip
	CheckWarn
)

// DoctorCheck holds the outcome of a single doctor check.
type DoctorCheck struct {
	Name    string
	Status  CheckStatus
	Message string
	Hint    string // optional remediation hint shown on failure
}

// StatusLabel returns a display string for the check status.
func (s CheckStatus) StatusLabel() string {
	switch s {
	case CheckOK:
		return "OK"
	case CheckFail:
		return "FAIL"
	case CheckSkip:
		return "SKIP"
	case CheckWarn:
		return "WARN"
	default:
		return "????"
	}
}
