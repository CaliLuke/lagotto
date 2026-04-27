package main

// Finding is one entry in a lagotto audit. Every detector emits zero
// or more findings; the JSON shape is the stable downstream contract.
//
// Field semantics:
//
//   - Smell: human-readable name ("Receiver Monolith")
//   - SmellID: short stable ID ("G1") — use this in tooling
//   - Severity: CRITICAL | HIGH | MEDIUM | LOW
//   - Location: directory and (for type-level smells) the type name
//   - Message: one-line summary suitable for a terminal
//   - Evidence: structured per-detector data (method counts, file
//     lists, package paths) for tooling that wants to drill in
//   - Suggestion: concrete imperative remediation guidance
type Finding struct {
	Smell      string         `json:"smell"`
	SmellID    string         `json:"smell_id"`
	Severity   Severity       `json:"severity"`
	Location   string         `json:"location"`
	Message    string         `json:"message"`
	Evidence   map[string]any `json:"evidence,omitempty"`
	Suggestion string         `json:"suggestion,omitempty"`
}

// Report is the top-level audit envelope written to stdout. Root is
// the path the audit was run against, Tags echoes the build tags the
// loader used, and Findings is severity-sorted (CRITICAL first).
type Report struct {
	Root     string    `json:"root"`
	Tags     []string  `json:"tags,omitempty"`
	Findings []Finding `json:"findings"`
}
