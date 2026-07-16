package detect

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/packages"

	"github.com/CaliLuke/lagotto/internal/audit"
)

// fakeModule writes a synthetic Go module to a tmp dir and returns
// loaded *packages.Package values with full type-check info. Files
// keyed by basename land at the module root; nested paths (e.g.
// "inner/inner.go") create subdirectories. A default go.mod is
// supplied if the caller does not provide one.
//
// Test fixtures use this helper so every detector can be exercised
// against a tiny, focused codebase rather than a real repo.
func fakeModule(t *testing.T, files map[string]string) []*packages.Package {
	t.Helper()
	dir := t.TempDir()
	if _, ok := files["go.mod"]; !ok {
		files["go.mod"] = "module example.com/test\ngo 1.21\n"
	}
	for name, content := range files {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedTypes |
			packages.NeedSyntax | packages.NeedTypesInfo,
		Dir: dir,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		t.Fatalf("packages.Load: %v", err)
	}
	return pkgs
}

// findingIDs extracts smell IDs in argument order. Useful for
// assertion error messages that show what fired vs. what was
// expected without dumping the full Finding struct.
func findingIDs(findings []audit.Finding) []string {
	out := make([]string, 0, len(findings))
	for _, f := range findings {
		out = append(out, f.SmellID)
	}
	return out
}

// containsID reports whether at least one finding has the given
// smell ID. Tests use this when they care that a detector fired,
// not how many times.
func containsID(findings []audit.Finding, id string) bool {
	for _, f := range findings {
		if f.SmellID == id {
			return true
		}
	}
	return false
}
