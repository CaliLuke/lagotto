package detect

import (
	"go/ast"
	"go/token"
	"sort"
	"strings"
)

// declGroup classifies the kind of declaration for mixed-concern
// detection.
type declGroup string

const (
	groupTypes      declGroup = "types"
	groupMethods    declGroup = "methods"
	groupValidation declGroup = "validation"
	groupUtilities  declGroup = "utilities"
)

func classifyFile(fset *token.FileSet, file *ast.File) (map[declGroup]int, int) {
	groups := map[declGroup]int{}
	lineCount := 0
	if tokenFile := fset.File(file.Pos()); tokenFile != nil {
		lineCount = tokenFile.LineCount()
	} else {
		pos := fset.Position(file.Pos())
		endPos := fset.Position(file.End())
		lineCount = endPos.Line - pos.Line + 1
	}

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			switch d.Tok {
			case token.TYPE:
				for _, spec := range d.Specs {
					if _, ok := spec.(*ast.TypeSpec); ok {
						groups[groupTypes]++
					}
				}
			case token.CONST, token.VAR:
				// Constants and variables commonly accompany the type or
				// methods they support; they are not an independent concern.
			}
		case *ast.FuncDecl:
			if d.Recv != nil {
				groups[groupMethods]++
			} else if isValidationFunc(d.Name.Name) {
				groups[groupValidation]++
			} else {
				groups[groupUtilities]++
			}
		}
	}
	return groups, lineCount
}

func isValidationFunc(name string) bool {
	for _, p := range []string{"Validate", "validate", "Verify", "verify", "Check", "check"} {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

func groupNames(g map[declGroup]int) []string {
	out := make([]string, 0, len(g))
	for k := range g {
		out = append(out, string(k))
	}
	sort.Strings(out)
	return out
}
