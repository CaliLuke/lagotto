package main

import (
	"fmt"
	"strings"

	"golang.org/x/tools/go/packages"
)

// loadPackages loads all packages under the given root with the
// configured build tags. Returns the loaded packages plus any
// non-fatal errors encountered.
func loadPackages(root string) ([]*packages.Package, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedDeps | packages.NeedTypes |
			packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedModule,
		Dir:   root,
		Tests: false,
	}
	if flagTags != "" {
		cfg.BuildFlags = []string{"-tags=" + flagTags}
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("load packages: %w", err)
	}
	return pkgs, nil
}

// shouldExclude reports whether the given import path or directory
// path should be skipped per the user's --exclude list.
func shouldExclude(p string) bool {
	for _, ex := range flagExclude {
		if ex == "" {
			continue
		}
		if strings.Contains(p, ex) {
			return true
		}
	}
	return false
}
