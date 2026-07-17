package audit

import "fmt"

// Finding is one entry in a lagotto audit. Every detector emits zero
// or more findings; the JSON shape is the stable downstream contract.
//
// Field semantics:
//
//   - Smell: human-readable name ("Receiver Monolith")
//   - SmellID: short stable ID ("G1") — use this in tooling
//   - Severity: CRITICAL | HIGH | MEDIUM | LOW
//   - Location: stable import path plus file/type/symbol where applicable
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
// LoadErrors carries per-package load/type errors so JSON consumers
// can tell a clean audit from a degraded one; when it is non-empty,
// detectors may have skipped the affected packages.
type Report struct {
	Version            string                 `json:"version"`
	Root               string                 `json:"root"`
	Config             string                 `json:"config,omitempty"`
	Configuration      EffectiveConfiguration `json:"configuration"`
	Tags               []string               `json:"tags,omitempty"`
	LoadErrors         []string               `json:"load_errors,omitempty"`
	SuppressedFindings int                    `json:"suppressed_findings,omitempty"`
	Findings           []Finding              `json:"findings"`
}

// EffectiveConfiguration records the policy actually used for an audit. It is
// deliberately resolved (defaults, repository config, and CLI overrides) so a
// saved JSON report can be reproduced without reconstructing flag precedence.
type EffectiveConfiguration struct {
	Exclude     []string                   `json:"exclude"`
	Suppress    []string                   `json:"suppress,omitempty"`
	FailOn      string                     `json:"fail_on,omitempty"`
	Mixed       MixedConfiguration         `json:"mixed"`
	LayerPolicy []LayerPolicyConfiguration `json:"layer_policy,omitempty"`
}

// LayerPolicyConfiguration records one resolved G14 policy.
type LayerPolicyConfiguration struct {
	Name                       string   `json:"name"`
	Paths                      []string `json:"paths"`
	Dependencies               []string `json:"dependencies"`
	GeneratedTypes             []string `json:"generated_types"`
	MaxCoordinatedDependencies int      `json:"max_coordinated_dependencies"`
	Severity                   Severity `json:"severity"`
}

// MixedConfiguration is the resolved policy for G5 and G13.
type MixedConfiguration struct {
	MinLines                     int      `json:"min_lines"`
	MinComponentMembers          int      `json:"min_component_members"`
	MinComponentLines            int      `json:"min_component_lines"`
	MinSingleComponentComplexity int      `json:"min_single_component_complexity"`
	Severity                     Severity `json:"severity"`
	CohesiveMinLines             int      `json:"cohesive_min_lines"`
}

// IncompleteLoadError means the report was emitted, but one or more
// packages did not load or type-check completely. Callers must treat
// this as a failed audit rather than a clean result.
type IncompleteLoadError struct {
	Count int
}

func (e *IncompleteLoadError) Error() string {
	return fmt.Sprintf("audit incomplete: %d package load error(s); see load_errors in the report", e.Count)
}
