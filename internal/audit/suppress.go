package audit

import (
	"fmt"
	"strings"
)

type suppression struct {
	smellID        string
	locationPrefix string
}

// ValidateSuppressions checks --suppress selectors without applying
// them. A selector is SMELL_ID to suppress that detector everywhere,
// or SMELL_ID@LOCATION to suppress only findings at LOCATION or nested
// beneath it (see [locationWithin]).
func ValidateSuppressions(raw []string) error {
	_, err := parseSuppressions(raw)
	return err
}

// ApplySuppressions filters findings using validated selectors and
// returns both the kept findings and the number removed.
func ApplySuppressions(findings []Finding, raw []string) ([]Finding, int, error) {
	suppressions, err := parseSuppressions(raw)
	if err != nil {
		return nil, 0, err
	}
	kept := make([]Finding, 0, len(findings))
	suppressed := 0
	for _, finding := range findings {
		if isSuppressed(finding, suppressions) {
			suppressed++
			continue
		}
		kept = append(kept, finding)
	}
	return kept, suppressed, nil
}

func parseSuppressions(raw []string) ([]suppression, error) {
	out := make([]suppression, 0, len(raw))
	for _, value := range raw {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, fmt.Errorf("--suppress contains an empty selector (use SMELL_ID or SMELL_ID@LOCATION)")
		}
		id, location, hasLocation := strings.Cut(value, "@")
		id = strings.ToUpper(strings.TrimSpace(id))
		location = strings.TrimSpace(location)
		if id == "" || strings.ContainsAny(id, " \t/:") {
			return nil, fmt.Errorf("invalid --suppress selector %q (use SMELL_ID or SMELL_ID@LOCATION)", value)
		}
		if hasLocation && location == "" {
			return nil, fmt.Errorf("invalid --suppress selector %q: location after @ is empty", value)
		}
		location = filepathSlash(location)
		if trimmed := strings.TrimRight(location, "/"); trimmed != "" {
			location = trimmed
		}
		out = append(out, suppression{smellID: id, locationPrefix: location})
	}
	return out, nil
}

func isSuppressed(finding Finding, suppressions []suppression) bool {
	location := filepathSlash(finding.Location)
	for _, selector := range suppressions {
		if strings.ToUpper(finding.SmellID) != selector.smellID {
			continue
		}
		if selector.locationPrefix == "" || locationWithin(location, selector.locationPrefix) {
			return true
		}
	}
	return false
}

// locationWithin reports whether location is scope itself or nested
// beneath it. Finding locations are built from components separated by
// '/' (directories and files), ':' (symbols) and ' ' ("pkg (Type)"),
// so scope must end on one of those boundaries: cmd/server matches
// cmd/server/main.go and cmd/server (Server) but not cmd/serverless.
// formal/Lagotto/Suppress.lean proves this sound and complete for the
// specification and a refinement of plain prefix matching.
func locationWithin(location, scope string) bool {
	rest, ok := strings.CutPrefix(location, scope)
	if !ok {
		return false
	}
	return rest == "" || strings.ContainsRune("/: ", rune(rest[0]))
}

func filepathSlash(value string) string {
	return strings.ReplaceAll(value, `\`, "/")
}
