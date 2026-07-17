package detect

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CaliLuke/lagotto/internal/audit"
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
	for _, finding := range findings {
		if finding.SmellID == "G9" && finding.Severity != audit.SevLow {
			t.Fatalf("prefix shape alone should remain LOW, got %+v", finding)
		}
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

func TestG9_ShortPrefixAndPlatformSuites(t *testing.T) {
	if findings := prefixClusterFindings("/tmp/fake", []string{"db_conn.go", "db_query.go", "db_tx.go"}); !containsID(findings, "G9") {
		t.Fatalf("expected G9 for a two-character prefix, got %v", findingIDs(findings))
	}
	platformFiles := []string{"socket_linux.go", "socket_darwin.go", "socket_windows.go"}
	if findings := prefixClusterFindings("/tmp/fake", platformFiles); containsID(findings, "G9") {
		t.Fatalf("did not expect G9 for a GOOS file suite, got %+v", findings)
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

func TestG11_RecommendsRenameWithoutAssumingSplit(t *testing.T) {
	findings := junkDrawerFindings("driver", []string{"helpers.go"}, map[string]sourceFileStats{
		"helpers.go": {DeclarationCount: 1, LineCount: 17},
	})
	if len(findings) != 1 {
		t.Fatalf("expected one G11 finding, got %+v", findings)
	}
	if findings[0].Smell != "Generic Filename" ||
		!strings.Contains(findings[0].Suggestion, "Rename") ||
		!strings.Contains(findings[0].Suggestion, "Split only") ||
		!strings.Contains(findings[0].Message, "1 top-level declaration(s) over 17 lines") ||
		findings[0].Severity != audit.SevLow {
		t.Fatalf("expected rename-first guidance without assumed decomposition, got %+v", findings[0])
	}
}

func TestG11_LargeGenericFileShowsAccumulationRisk(t *testing.T) {
	findings := junkDrawerFindings("driver", []string{"helpers.go"}, map[string]sourceFileStats{
		"helpers.go": {DeclarationCount: 20, LineCount: 500},
	})
	if len(findings) != 1 || findings[0].Severity != audit.SevMedium ||
		!strings.Contains(findings[0].Message, "accumulation risk") {
		t.Fatalf("expected a content-weighted accumulation warning, got %+v", findings)
	}
}

// TestG12_PrematurePackage fires on directories with exactly one
// source file — the package boundary is providing visibility, not
// grouping. doc.go-only packages are exempt (legitimate godoc home).
func TestG12_PrematurePackage(t *testing.T) {
	findings := prematurePackageFindings("/tmp/fake", "/tmp/fake/sub", "sub", packageDirContents{files: []string{"only.go"}, packageName: "foo"})
	if !containsID(findings, "G12") {
		t.Fatalf("expected G12, got %v", findingIDs(findings))
	}
}

// TestG12_DocOnly_NoFire confirms doc.go-only packages are exempt.
func TestG12_DocOnly_NoFire(t *testing.T) {
	findings := prematurePackageFindings("/tmp/fake", "/tmp/fake/sub", "sub", packageDirContents{})
	if containsID(findings, "G12") {
		t.Fatalf("did not expect G12 for doc-only package, got %+v", findings)
	}
}

// TestG12_TwoFiles_NoFire ensures multi-file packages are clean.
func TestG12_TwoFiles_NoFire(t *testing.T) {
	findings := prematurePackageFindings("/tmp/fake", "/tmp/fake/sub", "sub", packageDirContents{files: []string{"a.go", "b.go"}, packageName: "foo"})
	if containsID(findings, "G12") {
		t.Fatalf("did not expect G12 for 2-file package, got %+v", findings)
	}
}

// TestG12_SharedTestingSupport_NoFire covers a legitimate one-file package
// boundary: reusable assertions imported only by tests in sibling packages.
// Importing testing from production-shaped source is direct evidence of that
// role, so G12 should not ask callers to inline it.
func TestG12_SharedTestingSupport_NoFire(t *testing.T) {
	root := fakeDir(t, map[string]string{
		"internal/check/check.go": `package check

import "testing"

func Valid(t testing.TB) { t.Helper() }
`,
	})
	dir := filepath.Join(root, "internal", "check")
	contents := collectPackageDirs(root, nil, nil)[dir]
	if !contents.importsTesting {
		t.Fatal("expected collector to recognize the testing import")
	}
	findings := prematurePackageFindings(root, dir, "internal/check", contents)
	if containsID(findings, "G12") {
		t.Fatalf("did not expect G12 for shared testing support, got %+v", findings)
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
	tagged := map[string]bool{}
	for _, file := range files {
		tagged[file] = true
	}
	findings := buildTagPairFindings("/tmp/fake", packageDirContents{files: files, buildTagged: tagged})
	if !containsID(findings, "G3") {
		t.Fatalf("expected G3, got %v", findingIDs(findings))
	}
}

// TestG3_OnePair_NoFire ensures a single stub/real pair (a common
// and reasonable pattern) does NOT fire.
func TestG3_OnePair_NoFire(t *testing.T) {
	files := []string{"connect.go", "connect_stub.go", "main.go"}
	findings := buildTagPairFindings("/tmp/fake", packageDirContents{files: files, buildTagged: map[string]bool{"connect_stub.go": true}})
	if containsID(findings, "G3") {
		t.Fatalf("did not expect G3 for one pair, got %+v", findings)
	}
}

func TestScanFSReadsIgnoredBuildFilesAndFiltersGeneratedDocAndMain(t *testing.T) {
	root := fakeDir(t, map[string]string{
		"go.mod":              "module example.com/test\ngo 1.23\n",
		"pairs/a.go":          "//go:build feature\n\npackage pairs\n",
		"pairs/a_stub.go":     "//go:build !feature\n\npackage pairs\n",
		"pairs/b.go":          "//go:build feature\n\npackage pairs\n",
		"pairs/b_cgo.go":      "//go:build !feature\n\npackage pairs\n",
		"pairs/c.go":          "//go:build feature\n\npackage pairs\n",
		"pairs/c_stub.go":     "//go:build !feature\n\npackage pairs\n",
		"single/real.go":      "package single\n",
		"single/doc.go":       "package single\n",
		"generated/api.pb.go": "// Code generated by protoc. DO NOT EDIT.\npackage generated\n",
		"cmd/tool/main.go":    "package main\n\nfunc main() {}\n",
	})
	findings := ScanFS(root, nil, nil)
	if !containsID(findings, "G3") {
		t.Fatalf("expected G3 from mutually exclusive files on disk, got %v", findingIDs(findings))
	}
	g12Count := 0
	for _, finding := range findings {
		if finding.SmellID == "G12" {
			g12Count++
			if finding.Evidence["file"] != "real.go" {
				t.Fatalf("unexpected G12 target: %+v", finding)
			}
		}
	}
	if g12Count != 1 {
		t.Fatalf("expected only real.go + doc.go to produce G12, got %d in %+v", g12Count, findings)
	}
}
