package audit

import "strings"

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

// ParseSeverity converts a case-insensitive severity name ("high",
// "HIGH") to its Severity value. ok is false for unknown names.
func ParseSeverity(s string) (Severity, bool) {
	sev := Severity(strings.ToUpper(s))
	switch sev {
	case SevCritical, SevHigh, SevMedium, SevLow:
		return sev, true
	}
	return "", false
}

// AtLeast reports whether s is as severe as threshold or worse.
func (s Severity) AtLeast(threshold Severity) bool {
	return sevRank(s) <= sevRank(threshold)
}

// sevRank returns a sort key where CRITICAL < HIGH < MEDIUM < LOW.
// Used to order findings in [Emit].
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
