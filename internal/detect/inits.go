package detect

import (
	"fmt"
	"go/ast"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/CaliLuke/lagotto/internal/audit"
)

// ScanInitCoupling flags packages that have multiple `func init()`
// declarations spread across more than one file. Go runs init()
// functions in alphabetical filename order (and source order within
// a file), so cross-file init coupling depends on an implicit ordering
// rule that is fragile when files are renamed or split.
//
// Single-file multiple init() is not flagged: the source-order rule
// is local and visible.
func ScanInitCoupling(pkgs []*packages.Package) []audit.Finding {
	var findings []audit.Finding
	for _, pkg := range pkgs {
		if pkg.PkgPath == "" {
			continue
		}
		fileCounts := map[string]int{}
		total := 0
		for i, file := range pkg.Syntax {
			fname := syntaxFilename(pkg, i, file)
			if strings.HasSuffix(fname, "_test.go") {
				continue
			}
			base := filepath.Base(fname)
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Recv != nil {
					continue
				}
				if fn.Name.Name != "init" {
					continue
				}
				fileCounts[base]++
				total++
			}
		}
		if total < 2 || len(fileCounts) < 2 {
			continue
		}
		sev := audit.SevLow
		if total >= 3 || len(fileCounts) >= 3 {
			sev = audit.SevMedium
		}
		findings = append(findings, audit.Finding{
			Smell:    "Init Coupling",
			SmellID:  "G7",
			Severity: sev,
			Location: pkg.PkgPath,
			Message: fmt.Sprintf("Package has %d init() func(s) across %d file(s); cross-file initialization order is implicit and fragile.",
				total, len(fileCounts)),
			Evidence: map[string]any{
				"package":    pkg.PkgPath,
				"init_count": total,
				"files":      sortedKeys(fileCounts),
			},
			Suggestion: "Consolidate to a single init() in one file, OR replace package-level init with an explicit initialization function called from main(). Cross-file init ordering depends on alphabetical filename order — a rule that is invisible at the call site and breaks silently on file renames.",
		})
	}
	return findings
}
