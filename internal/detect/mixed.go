package detect

import (
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/CaliLuke/lagotto/internal/audit"
)

// MixedOptions controls G5's evidence floor and severity plus G13's separate
// cohesive-file navigation threshold. Defaults favor precision.
type MixedOptions struct {
	MinLines                     int
	MinComponentMembers          int
	MinComponentLines            int
	MinSingleComponentComplexity int
	Severity                     audit.Severity
	CohesiveMinLines             int
}

// DefaultMixedOptions returns Lagotto's precision-oriented G5 policy.
func DefaultMixedOptions() MixedOptions {
	return MixedOptions{
		MinLines:                     600,
		MinComponentMembers:          2,
		MinComponentLines:            40,
		MinSingleComponentComplexity: 5,
		Severity:                     audit.SevMedium,
		CohesiveMinLines:             1200,
	}
}

// ValidateMixedOptions rejects non-positive thresholds and unknown severities.
func ValidateMixedOptions(options MixedOptions) error {
	if options.MinLines < 1 {
		return fmt.Errorf("mixed min-lines must be at least 1")
	}
	if options.MinComponentMembers < 1 {
		return fmt.Errorf("mixed min-component-members must be at least 1")
	}
	if options.MinComponentLines < 1 {
		return fmt.Errorf("mixed min-component-lines must be at least 1")
	}
	if options.MinSingleComponentComplexity < 0 {
		return fmt.Errorf("mixed min-single-component-complexity cannot be negative")
	}
	if _, ok := audit.ParseSeverity(string(options.Severity)); !ok {
		return fmt.Errorf("unknown mixed severity %q (critical|high|medium|low)", options.Severity)
	}
	if options.CohesiveMinLines < 0 {
		return fmt.Errorf("mixed cohesive-min-lines cannot be negative")
	}
	return nil
}

// ScanMixedConcern flags very large files only when their top-level
// declarations form at least two substantial disconnected reference clusters.
// The graph includes implicit interface-implementation edges so idiomatic Go
// interface families remain one cohesive component.
func ScanMixedConcern(pkgs []*packages.Package) []audit.Finding {
	return ScanMixedConcernWithOptions(pkgs, DefaultMixedOptions())
}

// ScanMixedConcernWithOptions runs G5 with explicit thresholds and severity.
func ScanMixedConcernWithOptions(pkgs []*packages.Package, options MixedOptions) []audit.Finding {
	var findings []audit.Finding
	for _, pkg := range pkgs {
		if pkg.PkgPath == "" {
			continue
		}
		for i, file := range pkg.Syntax {
			fname := syntaxFilename(pkg, i, file)
			if skipSourceFile(fname, file) {
				continue
			}
			analysis := analyzeFileCohesion(pkg, file)
			initialSubstantial, _ := qualifyComponents(analysis.Components, options)
			g5Candidate := analysis.LineCount >= options.MinLines && len(initialSubstantial) >= 2
			g13Candidate := options.CohesiveMinLines > 0 && analysis.LineCount >= options.CohesiveMinLines && len(dotImportPaths(file)) == 0
			if !g5Candidate && !g13Candidate {
				continue
			}
			complexity := annotateCyclomaticComplexity(&analysis)
			substantial, minorCount, complexityRejected := qualifyComponentsAfterComplexity(analysis.Components, options)
			ignoredCount := ignoredComponentCount(analysis.Components)
			if analysis.LineCount >= options.MinLines && len(substantial) >= 2 {
				island := smallestComponent(substantial)
				findings = append(findings, audit.Finding{
					Smell:    "Disconnected File Concerns",
					SmellID:  "G5",
					Severity: options.Severity,
					Location: sourceLocation(pkg, fname),
					Message: fmt.Sprintf("File %s is %d lines and contains %d substantial disconnected declaration clusters.",
						filepath.Base(fname), analysis.LineCount, len(substantial)),
					Evidence: map[string]any{
						"file":                                fname,
						"package":                             pkg.PkgPath,
						"line_count":                          analysis.LineCount,
						"component_count":                     len(analysis.Components),
						"substantial_components":              substantial,
						"minor_component_count":               minorCount,
						"ignored_component_count":             ignoredCount,
						"complexity_rejected_component_count": complexityRejected,
						"cyclomatic_complexity":               complexity,
						"thresholds": map[string]any{
							"min_lines":                       options.MinLines,
							"min_component_members":           options.MinComponentMembers,
							"min_component_lines":             options.MinComponentLines,
							"min_single_component_complexity": options.MinSingleComponentComplexity,
						},
					},
					Suggestion: "Treat this as a backlog review signal, not a required refactor. One candidate island is: " + componentPreview(island) + ". If that cluster changes independently, move it together to a content-named file while preserving the public API and package boundary. If the graph misses a semantic relationship, keep the file intact and suppress the finding.",
				})
				continue
			}
			if g13Candidate && len(substantial) <= 1 {
				findings = append(findings, audit.Finding{
					Smell:    "Large Cohesive File",
					SmellID:  "G13",
					Severity: audit.SevLow,
					Location: sourceLocation(pkg, fname),
					Message: fmt.Sprintf("File %s is %d lines and contains %d substantial declaration/reference %s; size is a navigation or layer-policy review signal, not evidence of mixed concerns.",
						filepath.Base(fname), analysis.LineCount, len(substantial), componentWord(len(substantial))),
					Evidence: map[string]any{
						"file":                                fname,
						"package":                             pkg.PkgPath,
						"line_count":                          analysis.LineCount,
						"component_count":                     len(analysis.Components),
						"substantial_component_count":         len(substantial),
						"minor_component_count":               minorCount,
						"ignored_component_count":             ignoredCount,
						"complexity_rejected_component_count": complexityRejected,
						"cyclomatic_complexity":               complexity,
						"cohesive_min_lines":                  options.CohesiveMinLines,
					},
					Suggestion: "Do not split this file based on size alone. Review whether named phases or layer-specific sections change independently; if navigation is costly, move cohesive implementation sections to content-named files without changing the package or public API.",
				})
			}
		}
	}
	return findings
}

func ignoredComponentCount(components []cohesionComponent) int {
	count := 0
	for _, component := range components {
		if component.PrimaryCount == 0 {
			count++
		}
	}
	return count
}

func componentWord(count int) string {
	if count == 1 {
		return "component"
	}
	return "components"
}

func qualifyComponentsAfterComplexity(components []cohesionComponent, options MixedOptions) ([]cohesionComponent, int, int) {
	var substantial []cohesionComponent
	minor, rejected := 0, 0
	for _, component := range components {
		qualifiesByMembers := component.PrimaryCount >= options.MinComponentMembers
		qualifiesByLines := component.LineCount >= options.MinComponentLines
		if !qualifiesByMembers && qualifiesByLines && options.MinSingleComponentComplexity > 0 && isSingleCallableComponent(component) && component.CyclomaticMax < options.MinSingleComponentComplexity {
			rejected++
			minor++
			continue
		}
		if qualifiesByMembers || qualifiesByLines {
			substantial = append(substantial, component)
		} else if component.PrimaryCount > 0 {
			minor++
		}
	}
	return substantial, minor, rejected
}

func isSingleCallableComponent(component cohesionComponent) bool {
	if component.PrimaryCount != 1 {
		return false
	}
	callables := 0
	for _, member := range component.Members {
		if member.Kind == "function" || member.Kind == "method" {
			callables++
		}
	}
	return callables == 1
}

func qualifyComponents(components []cohesionComponent, options MixedOptions) ([]cohesionComponent, int) {
	var substantial []cohesionComponent
	minor := 0
	for _, component := range components {
		// A single tiny declaration can be an incidental helper. Requiring a
		// configurable member count or line span keeps G5 focused on separable
		// implementation islands rather than harmless orphans.
		if component.PrimaryCount >= options.MinComponentMembers || component.LineCount >= options.MinComponentLines {
			substantial = append(substantial, component)
		} else if component.PrimaryCount > 0 {
			minor++
		}
	}
	return substantial, minor
}

func smallestComponent(components []cohesionComponent) cohesionComponent {
	smallest := components[0]
	for _, component := range components[1:] {
		if component.PrimaryCount < smallest.PrimaryCount ||
			(component.PrimaryCount == smallest.PrimaryCount && component.LineCount < smallest.LineCount) {
			smallest = component
		}
	}
	return smallest
}

func componentPreview(component cohesionComponent) string {
	const limit = 6
	names := make([]string, 0, min(limit, len(component.Members)))
	for _, member := range component.Members[:min(limit, len(component.Members))] {
		names = append(names, member.Name)
	}
	preview := strings.Join(names, ", ")
	if len(component.Members) > limit {
		preview += fmt.Sprintf(" (and %d more)", len(component.Members)-limit)
	}
	return preview
}
