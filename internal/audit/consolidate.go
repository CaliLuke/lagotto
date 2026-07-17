package audit

import (
	"fmt"
	"sort"
)

const prematurePackageGroupFloor = 5

// ConsolidateRepositoryPatterns collapses detector output that describes a
// repository convention rather than independent defects. It runs after
// suppression so selectors such as G12@internal/storage remain precise.
func ConsolidateRepositoryPatterns(findings []Finding) []Finding {
	var g12, other []Finding
	for _, finding := range findings {
		if finding.SmellID == "G12" {
			g12 = append(g12, finding)
		} else {
			other = append(other, finding)
		}
	}
	if len(g12) < prematurePackageGroupFloor {
		return findings
	}
	sort.Slice(g12, func(i, j int) bool { return g12[i].Location < g12[j].Location })
	packages := make([]map[string]any, 0, len(g12))
	for _, finding := range g12 {
		entry := map[string]any{"location": finding.Location}
		if file, ok := finding.Evidence["file"]; ok {
			entry["file"] = file
		}
		packages = append(packages, entry)
	}
	other = append(other, Finding{
		Smell:    "Single-File Package Pattern",
		SmellID:  "G12",
		Severity: SevLow,
		Location: ".",
		Message: fmt.Sprintf("%d single-file packages appear across the repository; at this frequency they are likely a visibility or organization convention, not %d independent defects.",
			len(g12), len(g12)),
		Evidence: map[string]any{
			"package_count": len(g12),
			"packages":      packages,
			"grouped":       true,
		},
		Suggestion: "Review a representative sample. If these packages enforce intentional boundaries, keep the convention and suppress G12 globally; otherwise use the member list as a low-priority organization backlog.",
	})
	return other
}
