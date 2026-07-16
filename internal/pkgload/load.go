package pkgload

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

var fullMode = packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
	packages.NeedImports | packages.NeedTypes |
	packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedModule

var typesMode = packages.NeedName | packages.NeedCompiledGoFiles |
	packages.NeedImports | packages.NeedTypes | packages.NeedSyntax | packages.NeedModule

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
	return load(root, tags, exclude, fullMode, []string{"./..."}, nil)
}

// LoadTypes loads module-wide type metadata and declaration-only syntax
// without TypesInfo maps or implementation bodies. It is sufficient for
// receiver-layout detectors and substantially smaller than a full load.
func LoadTypes(root, tags string, exclude []string) ([]*packages.Package, []string, error) {
	imports, err := loadImportMetadata(root, tags)
	if err != nil {
		return nil, nil, err
	}
	return load(root, tags, exclude, typesMode, []string{"./..."}, imports)
}

// LoadPatterns fully loads only the requested package import paths.
// Audit uses this in bounded batches so TypesInfo for a large module
// is not retained for every package at once.
func LoadPatterns(root, tags string, exclude, patterns []string) ([]*packages.Package, []string, error) {
	return load(root, tags, exclude, fullMode, patterns, nil)
}

func load(root, tags string, exclude []string, mode packages.LoadMode, patterns []string, imports map[string]importMetadata) ([]*packages.Package, []string, error) {
	cfg := &packages.Config{
		Mode:  mode,
		Dir:   root,
		Tests: false,
	}
	if imports != nil {
		cfg.ParseFile = parseDeclarations(imports)
	}
	if tags != "" {
		cfg.BuildFlags = []string{"-tags=" + tags}
	}
	pkgs, err := packages.Load(cfg, patterns...)
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

// parseDeclarations preserves package-level declarations and method
// signatures while dropping function bodies. Receiver analysis needs
// the former but not statement-level AST, so this keeps unexported
// types visible without retaining the module's implementation bodies.
type importMetadata struct {
	name string
}

func parseDeclarations(imports map[string]importMetadata) func(*token.FileSet, string, []byte) (*ast.File, error) {
	return func(fset *token.FileSet, filename string, src []byte) (*ast.File, error) {
		file, err := parser.ParseFile(fset, filename, src, parser.AllErrors)
		if file == nil {
			return nil, err
		}
		// Dot-imported DSL packages expose unqualified symbols that
		// cannot be attributed syntactically to one import. These files
		// are uncommon, so retain them intact rather than risk changing
		// their type-checking semantics.
		for _, spec := range file.Imports {
			if spec.Name != nil && spec.Name.Name == "." {
				return file, err
			}
		}
		usedPackages := map[string]bool{}
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok {
				if fn.Recv != nil {
					inspectPackageSelectors(fn.Recv, usedPackages)
				}
				if fn.Type != nil {
					inspectPackageSelectors(fn.Type, usedPackages)
				}
				fn.Body = terminatingStub()
				continue
			}
			inspectPackageSelectors(decl, usedPackages)
		}
		for _, spec := range file.Imports {
			if spec.Name != nil && spec.Name.Name == "_" {
				continue
			}
			path, unquoteErr := strconv.Unquote(spec.Path.Value)
			if unquoteErr != nil {
				continue
			}
			metadata := imports[path]
			name := metadata.name
			if spec.Name != nil {
				name = spec.Name.Name
			}
			if name != "" && !usedPackages[name] {
				spec.Name = ast.NewIdent("_")
			}
		}
		return file, err
	}
}

func inspectPackageSelectors(node ast.Node, used map[string]bool) {
	if node == nil {
		return
	}
	ast.Inspect(node, func(node ast.Node) bool {
		sel, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if id, ok := sel.X.(*ast.Ident); ok {
			used[id.Name] = true
		}
		return true
	})
}

func terminatingStub() *ast.BlockStmt {
	return &ast.BlockStmt{List: []ast.Stmt{&ast.ExprStmt{X: &ast.CallExpr{
		Fun:  ast.NewIdent("panic"),
		Args: []ast.Expr{&ast.BasicLit{Kind: token.STRING, Value: `""`}},
	}}}}
}

func loadImportMetadata(root, tags string) (map[string]importMetadata, error) {
	cfg := &packages.Config{
		Mode:  packages.NeedName | packages.NeedImports | packages.NeedDeps,
		Dir:   root,
		Tests: false,
	}
	if tags != "" {
		cfg.BuildFlags = []string{"-tags=" + tags}
	}
	roots, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("load package names: %w", err)
	}
	metadata := map[string]importMetadata{}
	seen := map[string]bool{}
	var visit func(*packages.Package)
	visit = func(pkg *packages.Package) {
		if pkg == nil || seen[pkg.ID] {
			return
		}
		seen[pkg.ID] = true
		if pkg.PkgPath != "" && pkg.Name != "" {
			metadata[pkg.PkgPath] = importMetadata{name: pkg.Name}
		}
		for _, imported := range pkg.Imports {
			visit(imported)
		}
	}
	for _, root := range roots {
		visit(root)
	}
	return metadata, nil
}

// ValidateTags rejects malformed --tags values before they are handed
// to the go toolchain, whose own error ("go: -tags space-separated
// list contains comma") leaks build plumbing the user never invoked.
// Accepts the empty string (no tags) and comma-separated identifiers
// of letters, digits, underscores, and dots.
func ValidateTags(tags string) error {
	if tags == "" {
		return nil
	}
	for _, tag := range strings.Split(tags, ",") {
		if tag == "" {
			return fmt.Errorf("--tags %q contains an empty tag (use comma-separated names, e.g. cgo,typedb)", tags)
		}
		for _, r := range tag {
			if !isTagRune(r) {
				return fmt.Errorf("invalid build tag %q (tags are letters, digits, '_' and '.')", tag)
			}
		}
	}
	return nil
}

func isTagRune(r rune) bool {
	return r == '_' || r == '.' ||
		('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z') || ('0' <= r && r <= '9')
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
