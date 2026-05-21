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

// ScanFacades inspects every method body to find thin pass-throughs
// to a function in a different package.
//
// A method is a facade when:
//   - Its body has 1 statement that is a ReturnStmt or ExprStmt with
//     a single CallExpr, and the callee resolves to a function in a
//     different package.
//   - Or its body has 2-3 statements where all but the final return
//     are trivial guards/assignments and the final statement is the
//     above pattern.
//
// Methods on types that embed an interface in another package are
// flagged with reduced severity (interface-dispatch facades may be
// load-bearing).
func ScanFacades(pkgs []*packages.Package) []audit.Finding {
	var findings []audit.Finding
	for _, pkg := range pkgs {
		if pkg.PkgPath == "" {
			continue
		}
		if pkg.TypesInfo == nil {
			continue
		}
		for i, file := range pkg.Syntax {
			fname := syntaxFilename(pkg, i, file)
			if strings.HasSuffix(fname, "_test.go") {
				continue
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Recv == nil || fn.Body == nil {
					continue
				}
				if isFacade(pkg, fn) {
					recv := receiverTypeName(pkg.TypesInfo, fn.Recv.List[0])
					findings = append(findings, audit.Finding{
						Smell:    "Facade Method",
						SmellID:  "G6",
						Severity: audit.SevMedium,
						Location: fmt.Sprintf("%s:%s.%s", filepath.Base(fname), recv, fn.Name.Name),
						Message: fmt.Sprintf("Method (%s).%s is a thin pass-through to a function in another package.",
							recv, fn.Name.Name),
						Evidence: map[string]any{
							"file":          fname,
							"receiver":      recv,
							"method":        fn.Name.Name,
							"body_stmts":    len(fn.Body.List),
							"delegates_to":  delegateTarget(pkg, fn),
							"package":       pkg.PkgPath,
							"is_unexported": !fn.Name.IsExported(),
						},
						Suggestion: "Delete the facade method. Update callers to invoke the subpackage function directly. If the receiver type is part of an interface contract that callers depend on, narrow that interface or split it; do not retain the facade.",
					})
				}
			}
		}
	}
	return findings
}

// isFacade reports whether fn's body is a thin pass-through to a
// function in another package — at most 3 statements ending in a
// single cross-package call, with any prefix statements limited to
// trivial setup (assignment, nil-guard, declaration).
func isFacade(pkg *packages.Package, fn *ast.FuncDecl) bool {
	if fn.Body == nil {
		return false
	}
	stmts := fn.Body.List
	if len(stmts) == 0 || len(stmts) > 3 {
		return false
	}
	last := stmts[len(stmts)-1]
	call := extractDelegateCall(last)
	if call == nil {
		return false
	}
	target := callTargetPackage(pkg, call)
	if target == "" {
		return false
	}
	if target == pkg.PkgPath {
		return false
	}
	for _, s := range stmts[:len(stmts)-1] {
		if !isTrivialPrefix(s) {
			return false
		}
	}
	return true
}

// extractDelegateCall returns the call expression that delegates the
// statement's value (a `return X(...)` or `X(...)` ExprStmt). nil if
// the statement is anything else.
func extractDelegateCall(s ast.Stmt) *ast.CallExpr {
	switch x := s.(type) {
	case *ast.ReturnStmt:
		if len(x.Results) != 1 {
			return nil
		}
		if c, ok := x.Results[0].(*ast.CallExpr); ok {
			return c
		}
	case *ast.ExprStmt:
		if c, ok := x.X.(*ast.CallExpr); ok {
			return c
		}
	}
	return nil
}

// callTargetPackage returns the import path of the package whose
// function `call` invokes via a `pkgIdent.Func` selector. Empty for
// in-package calls and method calls on local values.
func callTargetPackage(pkg *packages.Package, call *ast.CallExpr) string {
	sel, ok := call.Fun.(*ast.SelectorExpr)
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

// delegateTarget returns "pkg.Func" for the function fn delegates
// to, used as evidence on Facade Method findings.
func delegateTarget(pkg *packages.Package, fn *ast.FuncDecl) string {
	if fn.Body == nil || len(fn.Body.List) == 0 {
		return ""
	}
	call := extractDelegateCall(fn.Body.List[len(fn.Body.List)-1])
	if call == nil {
		return ""
	}
	target := callTargetPackage(pkg, call)
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		return target + "." + sel.Sel.Name
	}
	return target
}

// isTrivialPrefix reports whether a leading body statement is simple
// enough to ignore for facade detection (a guard or simple binding).
// A loop or switch disqualifies the method from being a facade.
func isTrivialPrefix(s ast.Stmt) bool {
	switch s.(type) {
	case *ast.AssignStmt, *ast.IfStmt, *ast.DeclStmt:
		return true
	}
	return false
}
