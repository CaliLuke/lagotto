package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/tools/go/packages"
)

var depsCmd = &cobra.Command{
	Use:   "deps [path]",
	Short: "Find God Dependency Bag structs.",
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
		return emit(&Report{Root: root, Tags: resolvedTags(), Findings: scanDepsBag(pkgs)})
	},
}

// depsBagNames is the set of struct names recognized as
// dependency-aggregation bags. The detector never flags structs
// outside this list; the smell is specifically about a struct that
// presents itself as a collection of dependencies.
var depsBagNames = []string{"Deps", "Dependencies", "Container", "Services", "App", "Bag"}

// isDepsBagName reports whether n is one of the recognized
// dependency-bag struct names.
func isDepsBagName(n string) bool {
	for _, candidate := range depsBagNames {
		if n == candidate {
			return true
		}
	}
	return false
}

// scanDepsBag finds struct types whose name matches the dependency-
// bag pattern and counts heterogeneous field package types.
func scanDepsBag(pkgs []*packages.Package) []Finding {
	var findings []Finding
	for _, pkg := range pkgs {
		if pkg.PkgPath == "" || shouldExclude(pkg.PkgPath) {
			continue
		}
		for i, file := range pkg.Syntax {
			fname := pkg.GoFiles[i]
			if strings.HasSuffix(fname, "_test.go") {
				continue
			}
			for _, decl := range file.Decls {
				gen, ok := decl.(*ast.GenDecl)
				if !ok {
					continue
				}
				for _, spec := range gen.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok || !isDepsBagName(ts.Name.Name) {
						continue
					}
					st, ok := ts.Type.(*ast.StructType)
					if !ok || st.Fields == nil {
						continue
					}
					fieldCount, distinctPkgs := analyzeStructFields(pkg, st)
					if fieldCount < 8 {
						continue
					}
					// The smell is cross-domain mixing, not field count
					// alone. A wide struct whose fields all come from one
					// layer is a legitimate aggregator.
					if len(distinctPkgs) < 5 {
						continue
					}
					sev := SevHigh
					if fieldCount >= 12 || len(distinctPkgs) >= 8 {
						sev = SevCritical
					}
					findings = append(findings, Finding{
						Smell:    "God Dependency Bag",
						SmellID:  "G4",
						Severity: sev,
						Location: fmt.Sprintf("%s:%s", filepath.Base(fname), ts.Name.Name),
						Message: fmt.Sprintf("Struct %s in %s has %d fields drawing from %d distinct packages.",
							ts.Name.Name, pkg.PkgPath, fieldCount, len(distinctPkgs)),
						Evidence: map[string]any{
							"file":              fname,
							"struct":            ts.Name.Name,
							"field_count":       fieldCount,
							"distinct_packages": len(distinctPkgs),
							"packages":          distinctPkgs,
						},
						Suggestion: "Split into per-concern dependency bags (e.g., StorageDeps, GraphDeps, AuthDeps) and have each consumer accept only the bag it needs. Delete the original god bag in the same change; do not retain a wrapper that holds the smaller bags.",
					})
				}
			}
		}
	}
	return findings
}

// analyzeStructFields counts the fields of st (each named slot
// counts once; embedded fields count once) and returns the sorted
// list of distinct external packages those fields draw from. Local
// types and the package's own types are filtered out so the result
// reflects cross-domain mixing only.
func analyzeStructFields(pkg *packages.Package, st *ast.StructType) (int, []string) {
	pkgs := map[string]bool{}
	count := 0
	for _, field := range st.Fields.List {
		n := len(field.Names)
		if n == 0 {
			n = 1 // embedded field
		}
		count += n
		recordFieldPackage(pkg, field.Type, pkgs)
	}
	delete(pkgs, "")
	delete(pkgs, pkg.PkgPath) // local types don't count as cross-domain
	return count, sortedKeys(pkgs)
}

// recordFieldPackage records the import path of the named type used
// in expr (if any) into out. It strips pointer, slice, array, and
// map wrappers so `*pkg.T`, `[]pkg.T`, and `map[K]pkg.T` all count
// as a reference to pkg.
func recordFieldPackage(pkg *packages.Package, expr ast.Expr, out map[string]bool) {
	if pkg.TypesInfo == nil {
		return
	}
	t := pkg.TypesInfo.TypeOf(expr)
	if t == nil {
		return
	}
	for {
		switch x := t.(type) {
		case *types.Pointer:
			t = x.Elem()
			continue
		case *types.Slice:
			t = x.Elem()
			continue
		case *types.Array:
			t = x.Elem()
			continue
		case *types.Map:
			t = x.Elem()
			continue
		}
		break
	}
	named, ok := t.(*types.Named)
	if !ok {
		return
	}
	if obj := named.Obj(); obj != nil && obj.Pkg() != nil {
		out[obj.Pkg().Path()] = true
	}
}
