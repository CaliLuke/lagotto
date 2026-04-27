package main

import (
	"fmt"
	"go/ast"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
	"golang.org/x/tools/go/packages"
)

var stutterCmd = &cobra.Command{
	Use:   "stutter [path]",
	Short: "Find exported names that stutter on the package name.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		root := "."
		if len(args) == 1 {
			root = args[0]
		}
		pkgs, err := loadPackages(root)
		if err != nil {
			return err
		}
		return emit(&Report{Root: root, Tags: resolvedTags(), Findings: scanStutter(pkgs)})
	},
}

// scanStutter detects exported types and functions whose names
// start with the package name (case-insensitive at the leading
// segment, identifier-aware so `lanes.LaneConfig` is flagged but
// `lanes.Land` is not).
func scanStutter(pkgs []*packages.Package) []Finding {
	var findings []Finding
	for _, pkg := range pkgs {
		if pkg.PkgPath == "" || shouldExclude(pkg.PkgPath) {
			continue
		}
		pkgName := pkg.Name
		if pkgName == "" || pkgName == "main" {
			continue
		}
		offenders := map[string][]string{}
		for i, f := range pkg.Syntax {
			fname := pkg.GoFiles[i]
			if strings.HasSuffix(fname, "_test.go") {
				continue
			}
			for _, decl := range f.Decls {
				switch d := decl.(type) {
				case *ast.GenDecl:
					for _, spec := range d.Specs {
						if ts, ok := spec.(*ast.TypeSpec); ok && ts.Name.IsExported() {
							if stutters(pkgName, ts.Name.Name) {
								offenders["type"] = append(offenders["type"], ts.Name.Name)
							}
						}
					}
				case *ast.FuncDecl:
					if d.Recv != nil {
						continue
					}
					if d.Name.IsExported() && stutters(pkgName, d.Name.Name) {
						offenders["func"] = append(offenders["func"], d.Name.Name)
					}
				}
			}
		}
		count := 0
		for _, v := range offenders {
			count += len(v)
		}
		if count == 0 {
			continue
		}
		findings = append(findings, Finding{
			Smell:    "Stutter Names",
			SmellID:  "G2",
			Severity: SevMedium,
			Location: pkg.PkgPath,
			Message: fmt.Sprintf("Package %q has %d exported name(s) that stutter on the package name.",
				pkgName, count),
			Evidence: map[string]any{
				"package":   pkg.PkgPath,
				"pkg_name":  pkgName,
				"offenders": offenders,
			},
			Suggestion: fmt.Sprintf("Rename to drop the leading %q prefix (e.g., %s.Config instead of %s.PkgConfig). Update callers in the same change.", pkgName, pkgName, pkgName),
		})
	}
	return findings
}

// stutters reports whether identifier name begins with pkgName,
// allowing a CamelCase boundary so `lanes` matches `LaneConfig`
// (`Lane` → `lanes`/`lane` head) but does not match `Landfall`.
func stutters(pkgName, name string) bool {
	if pkgName == "" || name == "" {
		return false
	}
	lp := strings.ToLower(pkgName)
	ln := strings.ToLower(name)
	// Singular/plural tolerance.
	if !strings.HasPrefix(ln, lp) {
		// allow lanes ↔ Lane prefix
		if strings.HasSuffix(lp, "s") && strings.HasPrefix(ln, lp[:len(lp)-1]) {
			lp = lp[:len(lp)-1]
		} else {
			return false
		}
	}
	if len(name) == len(lp) {
		// e.g., type Lanes in package lanes is fine — the constructor.
		// But this is "the package's main type"; not a stutter.
		return false
	}
	// Next rune after the matched prefix should be uppercase letter
	// (CamelCase boundary), confirming we matched a whole leading
	// word, not a coincidental prefix match.
	next := rune(name[len(lp)])
	return unicode.IsUpper(next)
}
