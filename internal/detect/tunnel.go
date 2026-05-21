package detect

import (
	"fmt"
	"go/ast"
	"go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/CaliLuke/lagotto/internal/audit"
)

// ScanReExportTunnel finds packages whose top-level public surface
// is dominated by re-exports from a single deeper package. This is
// the TypeScript "barrel" pattern, which is wrong in Go: it adds
// indirection, makes call paths harder to trace, and the package
// has no genuine identity of its own.
//
// A re-export is one of:
//
//	type X = otherPkg.Y                       (type alias to another pkg)
//	var  X = otherPkg.Y                       (variable bound to qualified ident)
//	func X(args) ret { return otherPkg.X(args...) } (transparent function wrapper)
//
// A package is flagged when ≥50% of its exported top-level decls
// are re-exports AND a single target package accounts for at least
// half of those re-exports.
func ScanReExportTunnel(pkgs []*packages.Package) []audit.Finding {
	var findings []audit.Finding
	for _, pkg := range pkgs {
		if pkg.PkgPath == "" {
			continue
		}
		if pkg.TypesInfo == nil {
			continue
		}
		total := 0
		reExports := 0
		targets := map[string]int{}

		for i, file := range pkg.Syntax {
			fname := syntaxFilename(pkg, i, file)
			if strings.HasSuffix(fname, "_test.go") {
				continue
			}
			for _, decl := range file.Decls {
				switch d := decl.(type) {
				case *ast.GenDecl:
					for _, spec := range d.Specs {
						switch sp := spec.(type) {
						case *ast.TypeSpec:
							if !sp.Name.IsExported() {
								continue
							}
							total++
							if sp.Assign != 0 { // type alias
								if t := selectorPackage(pkg, sp.Type); t != "" && t != pkg.PkgPath {
									reExports++
									targets[t]++
								}
							}
						case *ast.ValueSpec:
							for j, name := range sp.Names {
								if !name.IsExported() {
									continue
								}
								total++
								if j < len(sp.Values) {
									if t := selectorPackage(pkg, sp.Values[j]); t != "" && t != pkg.PkgPath {
										reExports++
										targets[t]++
									}
								}
							}
						}
					}
				case *ast.FuncDecl:
					if d.Recv != nil {
						continue
					}
					if !d.Name.IsExported() {
						continue
					}
					total++
					if t := functionReExportTargetPkg(pkg, d); t != "" && t != pkg.PkgPath {
						reExports++
						targets[t]++
					}
				}
			}
		}

		if total < 3 {
			continue
		}
		ratio := float64(reExports) / float64(total)
		if ratio < 0.5 {
			continue
		}
		topTarget, topCount := dominantTarget(targets)
		if reExports == 0 {
			continue
		}
		if float64(topCount)/float64(reExports) < 0.5 {
			continue
		}
		sev := audit.SevMedium
		if ratio >= 0.8 {
			sev = audit.SevHigh
		}
		findings = append(findings, audit.Finding{
			Smell:    "Internal Re-Export Tunnel",
			SmellID:  "G8",
			Severity: sev,
			Location: pkg.PkgPath,
			Message: fmt.Sprintf("Package re-exports %d of %d exported decls (%.0f%%); %d target %s.",
				reExports, total, ratio*100, topCount, topTarget),
			Evidence: map[string]any{
				"package":         pkg.PkgPath,
				"total_exports":   total,
				"reexport_count":  reExports,
				"reexport_ratio":  ratio,
				"dominant_target": topTarget,
				"target_count":    topCount,
				"all_targets":     targets,
			},
			Suggestion: "Delete this package; update callers to import " + topTarget + " directly. Re-export tunnels add indirection without identity — every call path passes through a layer that contributes no logic, making the codebase harder to read and grep.",
		})
	}
	return findings
}

// selectorPackage resolves the package path that an expression's
// leading qualified identifier refers to (e.g., for `pkg.X` it
// returns the import path of `pkg`). Returns "" if the expression
// is not a qualified identifier.
func selectorPackage(pkg *packages.Package, expr ast.Expr) string {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return ""
	}
	if pkg.TypesInfo == nil {
		return ""
	}
	obj := pkg.TypesInfo.Uses[id]
	pkgName, ok := obj.(*types.PkgName)
	if !ok {
		return ""
	}
	return pkgName.Imported().Path()
}

// functionReExportTargetPkg returns the import path that a function
// transparently re-exports. G8 is about packages that expose another
// package's API surface, so a function only counts when it preserves
// the delegated function name and forwards its own parameters without
// adding literals, constants, validation, or other package-specific
// meaning.
func functionReExportTargetPkg(pkg *packages.Package, fn *ast.FuncDecl) string {
	if fn.Body == nil || len(fn.Body.List) != 1 {
		return ""
	}
	call := extractDelegateCall(fn.Body.List[0])
	if call == nil {
		return ""
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != fn.Name.Name {
		return ""
	}
	if !forwardsParamsInOrder(fn, call) {
		return ""
	}
	return callTargetPackage(pkg, call)
}

func forwardsParamsInOrder(fn *ast.FuncDecl, call *ast.CallExpr) bool {
	params := paramNames(fn.Type.Params)
	if len(params) != len(call.Args) {
		return false
	}
	for i, arg := range call.Args {
		id, ok := arg.(*ast.Ident)
		if !ok || id.Name != params[i] {
			return false
		}
	}
	return true
}

func paramNames(fields *ast.FieldList) []string {
	if fields == nil {
		return nil
	}
	var names []string
	for _, field := range fields.List {
		for _, name := range field.Names {
			names = append(names, name.Name)
		}
	}
	return names
}

// dominantTarget returns the most-referenced package and its count
// from a map of package path → reference count. Ties broken
// alphabetically. Used by ScanReExportTunnel to identify the single
// deeper package a tunnel re-exports from.
func dominantTarget(targets map[string]int) (string, int) {
	type kv struct {
		k string
		v int
	}
	var pairs []kv
	for k, v := range targets {
		pairs = append(pairs, kv{k, v})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].v != pairs[j].v {
			return pairs[i].v > pairs[j].v
		}
		return pairs[i].k < pairs[j].k
	})
	if len(pairs) == 0 {
		return "", 0
	}
	return pairs[0].k, pairs[0].v
}
