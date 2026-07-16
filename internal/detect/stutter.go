package detect

import (
	"fmt"
	"go/ast"
	"strings"
	"unicode"

	"golang.org/x/tools/go/packages"

	"github.com/CaliLuke/lagotto/internal/audit"
)

// ScanStutter detects exported types and functions whose names start
// with the package name (case-insensitive at the leading segment,
// identifier-aware so `lanes.LaneConfig` is flagged but `lanes.Land`
// is not).
func ScanStutter(pkgs []*packages.Package) []audit.Finding {
	var findings []audit.Finding
	for _, pkg := range pkgs {
		if pkg.PkgPath == "" {
			continue
		}
		pkgName := pkg.Name
		if pkgName == "" || pkgName == "main" {
			continue
		}
		offenders := map[string][]string{}
		for i, f := range pkg.Syntax {
			fname := syntaxFilename(pkg, i, f)
			if skipSourceFile(fname, f) {
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
		findings = append(findings, audit.Finding{
			Smell:    "Stutter Names",
			SmellID:  "G2",
			Severity: audit.SevMedium,
			Location: packageLocation(pkg),
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
	lp := []rune(strings.ToLower(pkgName))
	ln := []rune(strings.ToLower(name))
	matched := len(lp)
	// Singular/plural tolerance.
	if !runePrefix(ln, lp) {
		// allow lanes ↔ Lane prefix
		if len(lp) >= 3 && lp[len(lp)-1] == 's' && runePrefix(ln, lp[:len(lp)-1]) {
			matched--
		} else {
			return false
		}
	}
	nameRunes := []rune(name)
	if len(nameRunes) == matched {
		// e.g., type Lanes in package lanes is fine — the constructor.
		// But this is "the package's main type"; not a stutter.
		return false
	}
	// Next rune after the matched prefix should be uppercase letter
	// (CamelCase boundary), confirming we matched a whole leading
	// word, not a coincidental prefix match.
	next := nameRunes[matched]
	return unicode.IsUpper(next)
}

func runePrefix(s, prefix []rune) bool {
	if len(s) < len(prefix) {
		return false
	}
	for i := range prefix {
		if s[i] != prefix[i] {
			return false
		}
	}
	return true
}
