package detect

import (
	"fmt"
	"go/ast"
	"go/types"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/CaliLuke/lagotto/internal/audit"
)

// ScanInitCoupling flags packages that have multiple `func init()`
// declarations spread across files when an init in one file reads
// package-level state written by an init in another. This establishes
// a real cross-file ordering dependency rather than merely counting
// independent registration inits.
//
// Single-file multiple init() is not flagged: source order is local
// and visible.
func ScanInitCoupling(pkgs []*packages.Package) []audit.Finding {
	var findings []audit.Finding
	for _, pkg := range pkgs {
		if pkg.PkgPath == "" {
			continue
		}
		fileCounts := map[string]int{}
		var initBodies []initBodyAccess
		total := 0
		for i, file := range pkg.Syntax {
			fname := syntaxFilename(pkg, i, file)
			if skipSourceFile(fname, file) {
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
				initBodies = append(initBodies, analyzeInitBody(pkg, base, fn.Body))
			}
		}
		if total < 2 || len(fileCounts) < 2 {
			continue
		}
		dependencyVars := crossFileInitDependencies(initBodies)
		if len(dependencyVars) == 0 {
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
			Location: packageLocation(pkg),
			Message: fmt.Sprintf("Package has %d init() func(s) across %d file(s) with cross-file ordering dependencies through %s.",
				total, len(fileCounts), strings.Join(dependencyVars, ", ")),
			Evidence: map[string]any{
				"package":         pkg.PkgPath,
				"init_count":      total,
				"files":           sortedKeys(fileCounts),
				"dependency_vars": dependencyVars,
			},
			Suggestion: "Consolidate to a single init() in one file, OR replace package-level init with an explicit initialization function called from main(). Cross-file init ordering is implicit and can change when files are renamed or split.",
		})
	}
	return findings
}

type initBodyAccess struct {
	file   string
	reads  map[*types.Var]bool
	writes map[*types.Var]bool
}

func analyzeInitBody(pkg *packages.Package, filename string, body *ast.BlockStmt) initBodyAccess {
	access := initBodyAccess{file: filename, reads: map[*types.Var]bool{}, writes: map[*types.Var]bool{}}
	if pkg.TypesInfo == nil || body == nil {
		return access
	}
	writtenIDs := map[*ast.Ident]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			for _, lhs := range x.Lhs {
				ast.Inspect(lhs, func(n ast.Node) bool {
					if id, ok := n.(*ast.Ident); ok {
						writtenIDs[id] = true
						if v, ok := pkg.TypesInfo.Uses[id].(*types.Var); ok && v.Pkg() == pkg.Types && v.Parent() == pkg.Types.Scope() {
							access.writes[v] = true
						}
					}
					return true
				})
			}
		case *ast.IncDecStmt:
			ast.Inspect(x.X, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok {
					writtenIDs[id] = true
					if v, ok := pkg.TypesInfo.Uses[id].(*types.Var); ok && v.Pkg() == pkg.Types && v.Parent() == pkg.Types.Scope() {
						access.writes[v] = true
						access.reads[v] = true
					}
				}
				return true
			})
		}
		return true
	})
	ast.Inspect(body, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok || writtenIDs[id] {
			return true
		}
		if v, ok := pkg.TypesInfo.Uses[id].(*types.Var); ok && v.Pkg() == pkg.Types && v.Parent() == pkg.Types.Scope() {
			access.reads[v] = true
		}
		return true
	})
	return access
}

func crossFileInitDependencies(bodies []initBodyAccess) []string {
	dependencies := map[string]bool{}
	for i := range bodies {
		for j := range bodies {
			if i == j || bodies[i].file == bodies[j].file {
				continue
			}
			for variable := range bodies[i].writes {
				if bodies[j].reads[variable] {
					dependencies[variable.Name()] = true
				}
			}
		}
	}
	return sortedKeys(dependencies)
}
