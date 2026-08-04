package detect

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"golang.org/x/tools/go/packages"

	"github.com/CaliLuke/lagotto/internal/audit"
)

func TestScannersHandleMismatchedPackageFileMetadata(t *testing.T) {
	fset := token.NewFileSet()
	first, err := parser.ParseFile(fset, "first.go", "package foo\n\ntype Config struct{}\n", 0)
	if err != nil {
		t.Fatal(err)
	}
	second, err := parser.ParseFile(fset, "second.go", "package foo\n\ntype FooConfig struct{}\nfunc init() {}\n", 0)
	if err != nil {
		t.Fatal(err)
	}
	pkg := &packages.Package{
		PkgPath: "example.com/test/foo",
		Name:    "foo",
		GoFiles: []string{"first.go"},
		Syntax:  []*ast.File{first, second},
		Fset:    fset,
		TypesInfo: &types.Info{
			Uses: map[*ast.Ident]types.Object{},
		},
	}
	pkgs := []*packages.Package{pkg}

	scanners := map[string]func([]*packages.Package) []audit.Finding{
		"stutter":   ScanStutter,
		"deps":      ScanDepsBag,
		"mixed":     ScanMixedConcern,
		"facades":   ScanFacades,
		"inits":     ScanInitCoupling,
		"tunnel":    ScanReExportTunnel,
		"results":   ScanMaterializedResultPipelines,
		"receivers": ScanReceivers,
	}
	for name, scan := range scanners {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("scanner panicked with mismatched file metadata: %v", r)
				}
			}()
			_ = scan(pkgs)
		})
	}
}

func TestSyntaxFilenameUsesASTPositionBeforeFileSlices(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "/real/compiled.go", "package foo\n", 0)
	if err != nil {
		t.Fatal(err)
	}
	pkg := &packages.Package{
		GoFiles:         []string{"/wrong/source.go"},
		CompiledGoFiles: []string{"/also/wrong.go"},
		Fset:            fset,
	}
	if got := syntaxFilename(pkg, 0, file); got != "/real/compiled.go" {
		t.Fatalf("syntaxFilename = %q, want AST position filename", got)
	}
}
