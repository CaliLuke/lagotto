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
// or SMELL_ID@LOCATION to suppress only findings whose stable location
// starts with LOCATION.
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
		out = append(out, suppression{smellID: id, locationPrefix: filepathSlash(location)})
	}
	return out, nil
}

func isSuppressed(finding Finding, suppressions []suppression) bool {
	location := filepathSlash(finding.Location)
	for _, selector := range suppressions {
		if strings.ToUpper(finding.SmellID) != selector.smellID {
			continue
		}
		if selector.locationPrefix == "" || strings.HasPrefix(location, selector.locationPrefix) {
			return true
		}
	}
	return false
}

func filepathSlash(value string) string {
	return strings.ReplaceAll(value, `\`, "/")
}
