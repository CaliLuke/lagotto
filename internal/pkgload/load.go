package pkgload

import (
	"fmt"
	"strings"

	"golang.org/x/tools/go/packages"
)

// Load loads all packages under root with the given build tags and
// returns those whose import paths do not match any substring in
// exclude. Excluded packages are filtered here so detectors do not
// have to repeat the check.
func Load(root, tags string, exclude []string) ([]*packages.Package, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedDeps | packages.NeedTypes |
			packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedModule,
		Dir:   root,
		Tests: false,
	}
	if tags != "" {
		cfg.BuildFlags = []string{"-tags=" + tags}
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("load packages: %w", err)
	}
	if len(exclude) == 0 {
		return pkgs, nil
	}
	out := pkgs[:0]
	for _, p := range pkgs {
		if p.PkgPath == "" || ShouldExclude(p.PkgPath, exclude) {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

// ShouldExclude reports whether path matches any non-empty substring
// in exclude. Used by Load and by filesystem-walking detectors that
// inspect directories outside the loaded package set.
func ShouldExclude(path string, exclude []string) bool {
	for _, ex := range exclude {
		if ex == "" {
			continue
		}
		if strings.Contains(path, ex) {
			return true
		}
	}
	return false
}
