package audit

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// ValidateFormat rejects unknown --format values. The CLI calls this
// before loading any packages so a typo fails in milliseconds instead
// of after a full typecheck of the target module.
func ValidateFormat(format string) error {
	switch format {
	case "json", "text":
		return nil
	}
	return fmt.Errorf("unknown format %q (json|text)", format)
}

// Emit serializes a report in the requested format ("json" or "text").
// Findings are sorted by severity, smell ID, then location for stable
// output across runs. A nil Findings slice is normalized to an empty
// array so JSON consumers always see an array.
func Emit(report *Report, format string) error {
	if report.Findings == nil {
		report.Findings = []Finding{}
	}
	sort.SliceStable(report.Findings, func(i, j int) bool {
		ri, rj := sevRank(report.Findings[i].Severity), sevRank(report.Findings[j].Severity)
		if ri != rj {
			return ri < rj
		}
		if report.Findings[i].SmellID != report.Findings[j].SmellID {
			return report.Findings[i].SmellID < report.Findings[j].SmellID
		}
		if report.Findings[i].Location != report.Findings[j].Location {
			return report.Findings[i].Location < report.Findings[j].Location
		}
		return report.Findings[i].Message < report.Findings[j].Message
	})

	switch format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	case "text":
		return emitText(os.Stdout, report)
	default:
		return ValidateFormat(format)
	}
}

// emitText writes a human-readable rendering of the report to w.
// JSON remains the contract for tooling; the text format exists to
// make manual `lagotto` invocations readable. Write errors surface
// through the buffered writer's sticky error on Flush, so a truncated
// report (full disk, closed pipe) fails the run instead of exiting 0.
func emitText(w io.Writer, r *Report) error {
	bw := bufio.NewWriter(w)
	// bufio's write error is sticky, so per-line errors discarded here
	// resurface from Flush.
	p := func(format string, a ...any) {
		_, _ = fmt.Fprintf(bw, format, a...)
	}
	p("Lagotto audit — %s\n", r.Root)
	if len(r.Tags) > 0 {
		p("Build tags: %s\n", strings.Join(r.Tags, ","))
	}
	p("%d findings\n\n", len(r.Findings))
	for _, f := range r.Findings {
		p("[%s] %s (%s)\n", f.Severity, f.Smell, f.SmellID)
		p("  location: %s\n", f.Location)
		p("  %s\n", f.Message)
		if len(f.Evidence) > 0 {
			ev, err := json.Marshal(f.Evidence)
			if err != nil {
				return fmt.Errorf("marshal evidence for %s: %w", f.Location, err)
			}
			p("  evidence: %s\n", ev)
		}
		if f.Suggestion != "" {
			p("  suggestion: %s\n", f.Suggestion)
		}
		p("\n")
	}
	return bw.Flush()
}

// ResolvedTags splits a comma-separated --tags flag value into the
// slice that goes into the report envelope. Returns nil when no tags
// were supplied.
func ResolvedTags(tags string) []string {
	if tags == "" {
		return nil
	}
	return strings.Split(tags, ",")
}
