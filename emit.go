package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// emit serializes a report in the format requested by --format.
// Findings are sorted by severity, smell ID, then location for stable
// output across runs.
func emit(report *Report) error {
	sort.SliceStable(report.Findings, func(i, j int) bool {
		ri, rj := sevRank(report.Findings[i].Severity), sevRank(report.Findings[j].Severity)
		if ri != rj {
			return ri < rj
		}
		if report.Findings[i].SmellID != report.Findings[j].SmellID {
			return report.Findings[i].SmellID < report.Findings[j].SmellID
		}
		return report.Findings[i].Location < report.Findings[j].Location
	})

	switch flagFormat {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	case "text":
		emitText(report)
		return nil
	default:
		return fmt.Errorf("unknown format %q (json|text)", flagFormat)
	}
}

// emitText writes a human-readable rendering of the report to
// stdout. JSON remains the contract for tooling; the text format
// exists to make manual `lagotto` invocations readable.
func emitText(r *Report) {
	fmt.Printf("Lagotto audit — %s\n", r.Root)
	if len(r.Tags) > 0 {
		fmt.Printf("Build tags: %s\n", strings.Join(r.Tags, ","))
	}
	fmt.Printf("%d findings\n\n", len(r.Findings))
	for _, f := range r.Findings {
		fmt.Printf("[%s] %s (%s)\n", f.Severity, f.Smell, f.SmellID)
		fmt.Printf("  location: %s\n", f.Location)
		fmt.Printf("  %s\n", f.Message)
		if len(f.Evidence) > 0 {
			ev, _ := json.Marshal(f.Evidence)
			fmt.Printf("  evidence: %s\n", ev)
		}
		if f.Suggestion != "" {
			fmt.Printf("  suggestion: %s\n", f.Suggestion)
		}
		fmt.Println()
	}
}

// resolvedTags returns the build tags currently configured by the
// --tags flag, split for inclusion in the report envelope. nil if
// no tags were supplied.
func resolvedTags() []string {
	if flagTags == "" {
		return nil
	}
	return strings.Split(flagTags, ",")
}
