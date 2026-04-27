package detect

import "testing"

// TestG4_GodDependencyBag fires when a Deps/Container struct mixes
// dependency types from many packages — a sign that the consumer is
// crossing domain boundaries rather than taking a focused subset.
func TestG4_GodDependencyBag(t *testing.T) {
	files := map[string]string{
		"go.mod": "module example.com/test\ngo 1.21\n",
	}
	// Ten distinct packages, each exporting one type.
	for _, p := range []string{"auth", "events", "graph", "store", "audit", "comments", "search", "version", "source", "project"} {
		files[p+"/"+p+".go"] = "package " + p + "\n\ntype Service struct{}\n"
	}
	files["app/deps.go"] = `package app

import (
	"example.com/test/auth"
	"example.com/test/events"
	"example.com/test/graph"
	"example.com/test/store"
	"example.com/test/audit"
	"example.com/test/comments"
	"example.com/test/search"
	"example.com/test/version"
	"example.com/test/source"
	"example.com/test/project"
)

type Deps struct {
	Auth     *auth.Service
	Events   *events.Service
	Graph    *graph.Service
	Store    *store.Service
	Audit    *audit.Service
	Comments *comments.Service
	Search   *search.Service
	Version  *version.Service
	Source   *source.Service
	Project  *project.Service
}
`
	pkgs := fakeModule(t, files)
	findings := ScanDepsBag(pkgs)
	if !containsID(findings, "G4") {
		t.Fatalf("expected G4, got %v", findingIDs(findings))
	}
}

// TestG4_FocusedDeps_NoFire ensures a struct with all fields drawn
// from one domain (the legitimate aggregator pattern) does NOT fire,
// even when the field count is high.
func TestG4_FocusedDeps_NoFire(t *testing.T) {
	pkgs := fakeModule(t, map[string]string{
		"go.mod":             "module example.com/test\ngo 1.21\n",
		"transport/types.go": "package transport\n\ntype A struct{}\ntype B struct{}\ntype C struct{}\ntype D struct{}\n",
		"transport/deps.go": `package transport

type Deps struct {
	A1 *A
	A2 *A
	B1 *B
	B2 *B
	C1 *C
	C2 *C
	D1 *D
	D2 *D
	D3 *D
}
`,
	})
	findings := ScanDepsBag(pkgs)
	if containsID(findings, "G4") {
		t.Fatalf("did not expect G4 for single-domain Deps, got %+v", findings)
	}
}
