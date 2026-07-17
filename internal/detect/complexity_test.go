package detect

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestCyclomaticSummaryNamesHotspotsAsPrioritization(t *testing.T) {
	encoded, err := json.Marshal(cyclomaticSummary{
		FunctionCount: 1,
		Total:         2,
		Max:           2,
		PrioritizationHotspots: []cyclomaticHotspot{
			{Name: "reviewMe", Kind: "function", Line: 10, Complexity: 2},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	if !strings.Contains(text, `"prioritization_hotspots"`) || strings.Contains(text, `"hotspots"`) {
		t.Fatalf("complexity JSON should make prioritization intent explicit: %s", text)
	}
}

func TestCyclomaticComplexity(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "complex.go", `package test
func inspect(values []int, ready, valid bool) int {
	if ready && valid { return 1 }
	for _, value := range values {
		switch value {
		case 1:
			return value
		default:
			continue
		}
	}
	return 0
}
`, 0)
	if err != nil {
		t.Fatal(err)
	}
	function := file.Decls[0].(*ast.FuncDecl)
	// Base 1 + if + && + range + one non-default case.
	if got := cyclomaticComplexity(function.Body); got != 5 {
		t.Fatalf("cyclomatic complexity = %d, want 5", got)
	}
}

func TestCyclomaticComplexityExcludesNestedFunctionLiteral(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "complex.go", `package test
func outer() {
	callback := func(value int) bool {
		if value > 0 { return true }
		return false
	}
	_ = callback
}
`, 0)
	if err != nil {
		t.Fatal(err)
	}
	function := file.Decls[0].(*ast.FuncDecl)
	if got := cyclomaticComplexity(function.Body); got != 1 {
		t.Fatalf("outer complexity = %d, want 1", got)
	}
}
