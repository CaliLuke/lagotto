package main

// Severity ranks findings from CRITICAL (always investigate) down to
// LOW (worth knowing about, low blast radius). Severity is a string
// so the JSON output is human-readable.
type Severity string

// Severity values; ranking is CRITICAL < HIGH < MEDIUM < LOW (lower is
// worse), matched by sevRank.
const (
	SevCritical Severity = "CRITICAL"
	SevHigh     Severity = "HIGH"
	SevMedium   Severity = "MEDIUM"
	SevLow      Severity = "LOW"
)

// sevRank returns a sort key where CRITICAL < HIGH < MEDIUM < LOW.
// Used to order findings by severity in [emit].
func sevRank(s Severity) int {
	switch s {
	case SevCritical:
		return 0
	case SevHigh:
		return 1
	case SevMedium:
		return 2
	case SevLow:
		return 3
	}
	return 99
}
