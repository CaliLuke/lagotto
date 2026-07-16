package detect

import (
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/CaliLuke/lagotto/internal/audit"
)

// ScanMixedConcern flags files that mix three or more declaration
// groups (types, methods, validation, utilities) over the 100-line
// floor. The smell predicts the file is a junk drawer disguised as
// a module.
func ScanMixedConcern(pkgs []*packages.Package) []audit.Finding {
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
			groups, lineCount := classifyFile(pkg.Fset, file)
			if len(groups) < 3 {
				continue
			}
			if lineCount < 100 {
				continue
			}
			sev := audit.SevMedium
			if lineCount > 300 {
				sev = audit.SevHigh
			}
			if lineCount >= 600 {
				sev = audit.SevCritical
			}
			findings = append(findings, audit.Finding{
				Smell:    "Mixed-Concern File",
				SmellID:  "G5",
				Severity: sev,
				Location: sourceLocation(pkg, fname),
				Message: fmt.Sprintf("File %s mixes %d decl groups (%s) over %d lines.",
					filepath.Base(fname), len(groups), strings.Join(groupNames(groups), ", "), lineCount),
				Evidence: map[string]any{
					"file":       fname,
					"package":    pkg.PkgPath,
					"groups":     groupNames(groups),
					"line_count": lineCount,
				},
				Suggestion: "Split the file by concern: types in one file, methods in another, validation/utilities in their own files. Avoid mixing pure types with arbitrary helpers in the same file.",
			})
		}
	}
	return findings
}
