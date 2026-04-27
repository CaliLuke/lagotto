package detect

import (
	"os"
	"path/filepath"
	"testing"
)

// fakeDir writes a flat directory of files (basename → content) and
// returns the temp directory path. Use it to exercise filesystem
// detectors that examine file names rather than AST contents.
func fakeDir(t *testing.T, files map[string]string) string {
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

// TestG9_PrefixCluster fires when 3+ files in a flat directory share
// a name prefix, hinting that the cluster wants to be a subpackage.
func TestG9_PrefixCluster(t *testing.T) {
	dir := fakeDir(t, map[string]string{
		"node_create.go": "package foo\n",
		"node_delete.go": "package foo\n",
		"node_update.go": "package foo\n",
		"node_search.go": "package foo\n",
		"unrelated.go":   "package foo\n",
	})
	files := []string{"node_create.go", "node_delete.go", "node_update.go", "node_search.go", "unrelated.go"}
	findings := prefixClusterFindings(dir, files)
	if !containsID(findings, "G9") {
		t.Fatalf("expected G9, got %v", findingIDs(findings))
	}
	_ = dir
}

// TestG9_TwoFiles_NoFire ensures the threshold (3+) is enforced.
func TestG9_TwoFiles_NoFire(t *testing.T) {
	files := []string{"node_create.go", "node_delete.go", "unrelated.go"}
	findings := prefixClusterFindings("/tmp/fake", files)
	if containsID(findings, "G9") {
		t.Fatalf("did not expect G9 with 2 prefixed files, got %+v", findings)
	}
}

// TestG10_ShadowSuffix fires on files named by relationship rather
// than content (`*_helpers.go`, `*_utils.go`, `*_handlers.go`, etc.).
func TestG10_ShadowSuffix(t *testing.T) {
	files := []string{"user_helpers.go", "graph_handlers.go", "main.go"}
	findings := shadowSuffixFindings("/tmp/fake", files)
	if !containsID(findings, "G10") {
		t.Fatalf("expected G10, got %v", findingIDs(findings))
	}
}

// TestG10_ContentNames_NoFire ensures content-named files are clean.
func TestG10_ContentNames_NoFire(t *testing.T) {
	files := []string{"router.go", "session.go", "main.go"}
	findings := shadowSuffixFindings("/tmp/fake", files)
	if containsID(findings, "G10") {
		t.Fatalf("did not expect G10, got %+v", findings)
	}
}

// TestG11_JunkDrawer fires when a directory has a `helpers.go` /
// `utils.go` / `common.go` / `misc.go` / `shared.go` / `lib.go` file —
// names that describe location instead of content.
func TestG11_JunkDrawer(t *testing.T) {
	for _, name := range []string{"helpers.go", "utils.go", "common.go", "misc.go", "shared.go", "lib.go"} {
		findings := junkDrawerFindings("/tmp/fake", []string{name, "real.go"})
		if !containsID(findings, "G11") {
			t.Errorf("expected G11 for %s, got %v", name, findingIDs(findings))
		}
	}
}

// TestG11_RealNames_NoFire ensures named files don't trigger.
func TestG11_RealNames_NoFire(t *testing.T) {
	findings := junkDrawerFindings("/tmp/fake", []string{"router.go", "session.go"})
	if containsID(findings, "G11") {
		t.Fatalf("did not expect G11, got %+v", findings)
	}
}

// TestG12_PrematurePackage fires on directories with exactly one
// source file — the package boundary is providing visibility, not
// grouping. doc.go-only packages are exempt (legitimate godoc home).
func TestG12_PrematurePackage(t *testing.T) {
	findings := prematurePackageFindings("/tmp/fake/sub", []string{"only.go"})
	if !containsID(findings, "G12") {
		t.Fatalf("expected G12, got %v", findingIDs(findings))
	}
}

// TestG12_DocOnly_NoFire confirms doc.go-only packages are exempt.
func TestG12_DocOnly_NoFire(t *testing.T) {
	findings := prematurePackageFindings("/tmp/fake/sub", []string{"doc.go"})
	if containsID(findings, "G12") {
		t.Fatalf("did not expect G12 for doc-only package, got %+v", findings)
	}
}

// TestG12_TwoFiles_NoFire ensures multi-file packages are clean.
func TestG12_TwoFiles_NoFire(t *testing.T) {
	findings := prematurePackageFindings("/tmp/fake/sub", []string{"a.go", "b.go"})
	if containsID(findings, "G12") {
		t.Fatalf("did not expect G12 for 2-file package, got %+v", findings)
	}
}

// TestG3_BuildTagPairSprawl fires when a directory has 3+ paired
// `*_stub.go` / `*.go` files — the conditional surface is wide
// enough to deserve its own subpackage.
func TestG3_BuildTagPairSprawl(t *testing.T) {
	files := []string{
		"connect.go", "connect_stub.go",
		"reconnect.go", "reconnect_stub.go",
		"resilience.go", "resilience_stub.go",
		"unrelated.go",
	}
	findings := buildTagPairFindings("/tmp/fake", files)
	if !containsID(findings, "G3") {
		t.Fatalf("expected G3, got %v", findingIDs(findings))
	}
}

// TestG3_OnePair_NoFire ensures a single stub/real pair (a common
// and reasonable pattern) does NOT fire.
func TestG3_OnePair_NoFire(t *testing.T) {
	files := []string{"connect.go", "connect_stub.go", "main.go"}
	findings := buildTagPairFindings("/tmp/fake", files)
	if containsID(findings, "G3") {
		t.Fatalf("did not expect G3 for one pair, got %+v", findings)
	}
}
