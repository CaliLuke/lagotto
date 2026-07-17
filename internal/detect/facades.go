package detect

import (
	"fmt"
	"go/ast"
	"go/types"
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
			if skipSourceFile(fname, file) {
				continue
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Recv == nil || len(fn.Recv.List) == 0 || fn.Body == nil {
					continue
				}
				if isStandardContractMethod(pkg, fn) {
					continue
				}
				if implementsNamedPackageInterface(pkg, fn) {
					continue
				}
				if isFacade(pkg, fn) {
					recv := receiverTypeName(pkg.TypesInfo, fn.Recv.List[0])
					target := delegateTarget(pkg, fn)
					sev := audit.SevMedium
					suggestion := "Delete the facade method. Update callers to invoke the subpackage function directly. If the receiver type is part of an interface contract that callers depend on, narrow that interface or split it; do not retain the facade."
					classification := "pure_pass_through"
					if receiverEmbedsExternalInterface(pkg, recv) {
						sev = audit.SevLow
						classification = "interface_dispatch"
						suggestion = "This receiver embeds an external interface, so the method may be a load-bearing adapter. Keep it if callers rely on that interface contract; otherwise remove the pass-through and call the underlying package directly."
					} else if callUsesReceiverState(fn) {
						sev = audit.SevLow
						classification = "state_binding"
						suggestion = "This method binds receiver state into the delegated call. Keep it if that state is intentionally encapsulated; otherwise expose a narrower underlying type or move the behavior to the package that owns the state."
					} else if isStandardLibraryTarget(target) {
						sev = audit.SevLow
						classification = "stdlib_boundary"
						suggestion = "This is a small standard-library boundary and may be an intentional test seam. Keep it when it provides a stable contract; otherwise call the standard-library function directly."
					}
					findings = append(findings, audit.Finding{
						Smell:    "Facade Method",
						SmellID:  "G6",
						Severity: sev,
						Location: fmt.Sprintf("%s:%s.%s", sourceLocation(pkg, fname), recv, fn.Name.Name),
						Message: fmt.Sprintf("Method (%s).%s in %s is a thin pass-through to a function in another package.",
							recv, fn.Name.Name, sourceLocation(pkg, fname)),
						Evidence: map[string]any{
							"file":           fname,
							"receiver":       recv,
							"method":         fn.Name.Name,
							"body_stmts":     len(fn.Body.List),
							"delegates_to":   target,
							"classification": classification,
							"package":        pkg.PkgPath,
							"is_unexported":  !fn.Name.IsExported(),
						},
						Suggestion: suggestion,
					})
				}
			}
		}
	}
	return findings
}

// implementsNamedPackageInterface reports whether fn is part of a named
// interface contract declared by the package. Go implementations are
// structural, so there is no explicit declaration to inspect: the receiver
// must implement the complete interface and the interface must contain this
// method. In that case a concise cross-package call is an implementation of
// the contract, not evidence that the method itself is redundant.
func implementsNamedPackageInterface(pkg *packages.Package, fn *ast.FuncDecl) bool {
	if pkg.Types == nil || pkg.TypesInfo == nil {
		return false
	}
	method, ok := pkg.TypesInfo.Defs[fn.Name].(*types.Func)
	if !ok {
		return false
	}
	sig, ok := method.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return false
	}
	receiver := sig.Recv().Type()
	scope := pkg.Types.Scope()
	for _, name := range scope.Names() {
		typeName, ok := scope.Lookup(name).(*types.TypeName)
		if !ok || typeName.IsAlias() {
			continue
		}
		named, ok := typeName.Type().(*types.Named)
		if !ok {
			continue
		}
		iface, ok := named.Underlying().(*types.Interface)
		if !ok {
			continue
		}
		iface.Complete()
		if !iface.IsMethodSet() || !interfaceContainsMethod(iface, fn.Name.Name) {
			continue
		}
		if types.Implements(receiver, iface) {
			return true
		}
	}
	return false
}

func interfaceContainsMethod(iface *types.Interface, name string) bool {
	for i := 0; i < iface.NumMethods(); i++ {
		if iface.Method(i).Name() == name {
			return true
		}
	}
	return false
}

// isStandardContractMethod excludes canonical methods whose names and
// signatures are dictated by ubiquitous Go interfaces. A thin body is
// expected for these contracts and is not evidence of a redundant API.
func isStandardContractMethod(pkg *packages.Package, fn *ast.FuncDecl) bool {
	if pkg.TypesInfo == nil {
		return false
	}
	obj, ok := pkg.TypesInfo.Defs[fn.Name].(*types.Func)
	if !ok {
		return false
	}
	sig, ok := obj.Type().(*types.Signature)
	if !ok || sig.Params().Len() != 0 || sig.Results().Len() != 1 {
		return false
	}
	result := sig.Results().At(0).Type()
	stringType := types.Universe.Lookup("string").Type()
	errorType := types.Universe.Lookup("error").Type()
	switch fn.Name.Name {
	case "Error", "String":
		return types.Identical(result, stringType)
	case "Unwrap":
		if types.Identical(result, errorType) {
			return true
		}
		slice, ok := result.(*types.Slice)
		return ok && types.Identical(slice.Elem(), errorType)
	default:
		return false
	}
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
	if pkg.Fset == nil || pkg.Fset.Position(fn.Body.Rbrace).Line-pkg.Fset.Position(fn.Body.Lbrace).Line+1 > 3 {
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
	if _, ok := pkg.TypesInfo.Uses[sel.Sel].(*types.Func); !ok {
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

func callUsesReceiverState(fn *ast.FuncDecl) bool {
	if fn.Body == nil || len(fn.Body.List) == 0 || fn.Recv == nil || len(fn.Recv.List) == 0 || len(fn.Recv.List[0].Names) == 0 {
		return false
	}
	recvName := fn.Recv.List[0].Names[0].Name
	call := extractDelegateCall(fn.Body.List[len(fn.Body.List)-1])
	if call == nil {
		return false
	}
	for _, arg := range call.Args {
		usesState := false
		ast.Inspect(arg, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == recvName {
				usesState = true
				return false
			}
			return true
		})
		if usesState {
			return true
		}
	}
	return false
}

func receiverEmbedsExternalInterface(pkg *packages.Package, recvName string) bool {
	if pkg.Types == nil {
		return false
	}
	obj := pkg.Types.Scope().Lookup(recvName)
	if obj == nil {
		return false
	}
	named, ok := types.Unalias(obj.Type()).(*types.Named)
	if !ok {
		return false
	}
	st, ok := named.Underlying().(*types.Struct)
	if !ok {
		return false
	}
	for i := 0; i < st.NumFields(); i++ {
		field := st.Field(i)
		if !field.Embedded() {
			continue
		}
		t := types.Unalias(field.Type())
		if ptr, ok := t.(*types.Pointer); ok {
			t = types.Unalias(ptr.Elem())
		}
		if _, ok := t.Underlying().(*types.Interface); !ok {
			continue
		}
		if named, ok := t.(*types.Named); ok && named.Obj().Pkg() != nil && named.Obj().Pkg() != pkg.Types {
			return true
		}
	}
	return false
}

func isStandardLibraryTarget(target string) bool {
	i := strings.LastIndex(target, ".")
	if i < 0 {
		return false
	}
	pkgPath := target[:i]
	return pkgPath != "" && !strings.Contains(pkgPath, ".")
}
