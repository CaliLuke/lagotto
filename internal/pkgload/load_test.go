package pkgload

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShouldExcludeMatchesWholeSegments(t *testing.T) {
	defaults := []string{"gen", "vendor", "third_party", "design/generated"}
	cases := []struct {
		path string
		want bool
	}{
		// Segment matches.
		{"example.com/mod/gen", true},
		{"example.com/mod/gen/proto", true},
		{"gen/proto", true},
		{"example.com/mod/vendor/lib", true},
		{"example.com/mod/design/generated", true},
		{"example.com/mod/design/generated/icons", true},
		// Substring-only matches must NOT exclude (issues #7/#10).
		{"example.com/mod/agent", false},
		{"example.com/mod/agents", false},
		{"example.com/mod/engine", false},
		{"example.com/mod/legend", false},
		{"example.com/eugene/tool", false},
		{"example.com/mod/internal/vendorfeed", false},
		{"example.com/mod/redesign/generated", false},
		{"example.com/mod/design/generated2", false},
	}
	for _, c := range cases {
		if got := ShouldExclude(c.path, defaults); got != c.want {
			t.Errorf("ShouldExclude(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestShouldExcludeEmptyAndSlashTrimmedPatterns(t *testing.T) {
	if ShouldExclude("example.com/mod/pkg", []string{""}) {
		t.Error("empty pattern must not exclude anything")
	}
	if !ShouldExclude("example.com/mod/gen/x", []string{"/gen/"}) {
		t.Error("pattern with surrounding slashes should still match the segment")
	}
}

// writeModule lays out a synthetic module and returns its root.
func writeModule(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestLoadKeepsAgentDropsGen(t *testing.T) {
	root := writeModule(t, map[string]string{
		"go.mod":         "module example.com/test\ngo 1.21\n",
		"agent/agent.go": "package agent\n\ntype Agent struct{}\n",
		"gen/gen.go":     "package gen\n\ntype Generated struct{}\n",
	})
	pkgs, loadErrs, err := Load(root, "", []string{"gen", "vendor", "third_party", "design/generated"})
	if err != nil {
		t.Fatal(err)
	}
	if len(loadErrs) != 0 {
		t.Fatalf("unexpected load errors: %v", loadErrs)
	}
	var paths []string
	for _, p := range pkgs {
		paths = append(paths, p.PkgPath)
	}
	got := strings.Join(paths, ",")
	if !strings.Contains(got, "example.com/test/agent") {
		t.Errorf("agent package was excluded; loaded: %s", got)
	}
	if strings.Contains(got, "example.com/test/gen") {
		t.Errorf("gen package was not excluded; loaded: %s", got)
	}
}

func TestLoadReportsPackageErrors(t *testing.T) {
	root := writeModule(t, map[string]string{
		"go.mod":    "module example.com/broken\ngo 1.21\n",
		"broken.go": "package broken\n\nfunc F() int { return \"not an int\" }\n",
	})
	_, loadErrs, err := Load(root, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(loadErrs) == 0 {
		t.Fatal("expected load errors for a type-broken package, got none")
	}
	if !strings.Contains(loadErrs[0], "example.com/broken") {
		t.Errorf("load error should name the package: %q", loadErrs[0])
	}
}
