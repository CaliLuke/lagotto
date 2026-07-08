package audit

import "strconv"

// FindingsError signals that findings met the --fail-on threshold.
// The run itself succeeded — the report was emitted — so main
// translates this into exit code 2, distinguishing "findings found"
// (2) from "run failed" (1).
type FindingsError struct {
	Count     int
	Threshold Severity
}

func (e *FindingsError) Error() string {
	return strconv.Itoa(e.Count) + " finding(s) at or above " + string(e.Threshold)
}
