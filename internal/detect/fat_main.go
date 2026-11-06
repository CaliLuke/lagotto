package detect

import (
	"fmt"
	"go/ast"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/CaliLuke/lagotto/internal/audit"
)

type mainHotspot struct {
	File       string `json:"file"`
	Name       string `json:"name"`
	Line       int    `json:"line"`
	Complexity int    `json:"complexity"`
}

// ScanFatMains reports large main packages with structural evidence of logic
// beyond entry-point wiring. Complexity is evidence, never a trigger alone.
func ScanFatMains(pkgs []*packages.Package) []audit.Finding {
	var findings []audit.Finding
	for _, pkg := range pkgs {
		if pkg == nil || pkg.Name != "main" || pkg.Fset == nil || skipMainPath(pkg.PkgPath) {
			continue
		}
		lines, files, functions := 0, 0, 0
		receivers := map[string]bool{}
		declaredTypes := map[string]bool{}
		var hotspots []mainHotspot
		for i, file := range pkg.Syntax {
			filename := syntaxFilename(pkg, i, file)
			if skipSourceFile(filename, file) {
				continue
			}
			lines += physicalLineCount(pkg.Fset, file)
			files++
			for _, declaration := range file.Decls {
				switch decl := declaration.(type) {
				case *ast.GenDecl:
					for _, spec := range decl.Specs {
						if typ, ok := spec.(*ast.TypeSpec); ok && !typ.Assign.IsValid() {
							declaredTypes[typ.Name.Name] = true
						}
					}
				case *ast.FuncDecl:
					if decl.Body == nil {
						continue
					}
					name := decl.Name.Name
					if decl.Recv != nil && len(decl.Recv.List) > 0 {
						receiver := receiverTypeName(pkg.TypesInfo, decl.Recv.List[0])
						receivers[receiver] = true
						name = receiver + "." + name
					} else if name != "main" && name != "init" {
						functions++
					}
					hotspots = append(hotspots, mainHotspot{File: sourceLocation(pkg, filename), Name: name, Line: pkg.Fset.Position(decl.Pos()).Line, Complexity: cyclomaticComplexity(decl.Body)})
				}
			}
		}
		methodTypes := 0
		for name := range receivers {
			if declaredTypes[name] {
				methodTypes++
			}
		}
		if lines < 1000 || (methodTypes < 3 && functions < 15) {
			continue
		}
		sort.Slice(hotspots, func(i, j int) bool {
			if hotspots[i].Complexity != hotspots[j].Complexity {
				return hotspots[i].Complexity > hotspots[j].Complexity
			}
			if hotspots[i].File != hotspots[j].File {
				return hotspots[i].File < hotspots[j].File
			}
			return hotspots[i].Line < hotspots[j].Line
		})
		if len(hotspots) > 5 {
			hotspots = hotspots[:5]
		}
		hasInternal := false
		if pkg.Module != nil {
			info, err := os.Stat(filepath.Join(pkg.Module.Dir, "internal"))
			hasInternal = err == nil && info.IsDir()
		}
		findings = append(findings, audit.Finding{
			Smell: "Fat Main", SmellID: "G17", Severity: audit.SevLow, Location: packageLocation(pkg),
			Message:    fmt.Sprintf("Main package has %d lines, %d types with methods and %d helper functions; review whether reusable logic belongs in an importable package.", lines, methodTypes, functions),
			Evidence:   map[string]any{"package": pkg.PkgPath, "line_count": lines, "file_count": files, "type_count": len(declaredTypes), "types_with_methods": methodTypes, "function_count": functions, "hotspots": hotspots, "has_internal": hasInternal},
			Suggestion: "Keep flag parsing and wiring in main. Consider moving independently useful logic into focused importable packages; a self-contained command may intentionally keep its logic together.",
		})
	}
	return findings
}

func skipMainPath(path string) bool {
	if isTestPackage(path) {
		return true
	}
	for _, part := range strings.Split(path, "/") {
		if part == "example" || part == "examples" || part == "testdata" {
			return true
		}
	}
	return false
}
