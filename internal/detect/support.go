package detect

import (
	"go/ast"
	"go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// receiverTypeName returns the receiver type name for a method's
// receiver field, using TypesInfo for accurate resolution. Strips
// pointer indirection and generic instantiation arguments. Used by
// detectors (facades) that work at the AST level.
func receiverTypeName(info *types.Info, recv *ast.Field) string {
	if info == nil {
		return astReceiverFallback(recv.Type)
	}
	t := info.TypeOf(recv.Type)
	if t == nil {
		return astReceiverFallback(recv.Type)
	}
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	named, ok := t.(*types.Named)
	if !ok {
		return ""
	}
	return named.Obj().Name()
}

// astReceiverFallback walks an ast.Expr looking for the underlying
// type identifier when type info is unavailable.
func astReceiverFallback(expr ast.Expr) string {
	switch x := expr.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.StarExpr:
		return astReceiverFallback(x.X)
	case *ast.IndexExpr:
		return astReceiverFallback(x.X)
	case *ast.IndexListExpr:
		return astReceiverFallback(x.X)
	}
	return ""
}

// isTestDouble matches names that signal an intentional comprehensive
// interface implementation (Fake/Mock/Stub/Spy). Such types legitimately
// own a method set as wide as the interface they satisfy.
func isTestDouble(name string) bool {
	for _, prefix := range []string{"Fake", "Mock", "Stub", "Spy", "Noop", "Nop"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// isTestPackage filters out testutil/testing helper packages.
func isTestPackage(path string) bool {
	for _, frag := range []string{"/testutil", "/testing", "/testfixture", "/testdouble", "/internal/testutil"} {
		if strings.Contains(path, frag) {
			return true
		}
	}
	return false
}

// embeddedFieldType returns the name of the named type embedded at
// field index i of named's struct underlying, stripping pointer
// indirection. Empty if the field is not an embedded named type.
func embeddedFieldType(named *types.Named, i int) string {
	st, ok := named.Underlying().(*types.Struct)
	if !ok || i < 0 || i >= st.NumFields() {
		return ""
	}
	f := st.Field(i)
	if !f.Embedded() {
		return ""
	}
	t := f.Type()
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	if n, ok := t.(*types.Named); ok {
		return n.Obj().Name()
	}
	return ""
}

// firstFilename returns a representative source filename for pkg, or
// the package path if no Go files are loaded.
func firstFilename(pkg *packages.Package) string {
	if len(pkg.GoFiles) > 0 {
		return pkg.GoFiles[0]
	}
	if len(pkg.CompiledGoFiles) > 0 {
		return pkg.CompiledGoFiles[0]
	}
	return pkg.PkgPath
}

// sortedKeys returns the map's keys in sorted order.
func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// sortedCopy returns a sorted copy of s.
func sortedCopy(s []string) []string {
	out := make([]string, len(s))
	copy(out, s)
	sort.Strings(out)
	return out
}
