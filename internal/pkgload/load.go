package pkgload

import (
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

// Load loads all packages under root with the given build tags and
// returns those whose import paths do not match any path segment in
// exclude. Excluded packages are filtered here so detectors do not
// have to repeat the check.
//
// The second return value collects per-package load errors (syntax
// errors, type errors, unresolved imports) from the packages that
// were kept. These do not abort the audit — detectors still run on
// whatever type-checked — but callers must surface them so a broken
// package is distinguishable from a clean one.
func Load(root, tags string, exclude []string) ([]*packages.Package, []string, error) {
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
		return nil, nil, fmt.Errorf("load packages: %w", err)
	}
	out := pkgs[:0]
	for _, p := range pkgs {
		if p.PkgPath == "" || ShouldExclude(p.PkgPath, exclude) {
			continue
		}
		out = append(out, p)
	}
	var loadErrs []string
	for _, p := range out {
		for _, e := range p.Errors {
			loadErrs = append(loadErrs, fmt.Sprintf("%s: %s", p.PkgPath, e.Error()))
		}
	}
	return out, loadErrs, nil
}

// ShouldExclude reports whether any non-empty pattern in exclude
// matches a run of complete path segments in path. Matching is
// segment-anchored, not substring: "gen" excludes "a/gen/b" but not
// "a/agent/b", and "design/generated" excludes "x/design/generated"
// but not "x/redesign/generated". Used by Load and by
// filesystem-walking detectors that inspect directories outside the
// loaded package set.
func ShouldExclude(path string, exclude []string) bool {
	segs := strings.Split(filepath.ToSlash(path), "/")
	for _, ex := range exclude {
		ex = strings.Trim(filepath.ToSlash(ex), "/")
		if ex == "" {
			continue
		}
		if matchesSegments(segs, strings.Split(ex, "/")) {
			return true
		}
	}
	return false
}

// matchesSegments reports whether pat occurs as a consecutive run of
// complete segments anywhere in segs.
func matchesSegments(segs, pat []string) bool {
	for i := 0; i+len(pat) <= len(segs); i++ {
		match := true
		for j, p := range pat {
			if segs[i+j] != p {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
