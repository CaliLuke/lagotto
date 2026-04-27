package main

import "testing"

// TestG7_InitCoupling fires when a package has multiple init()
// functions across separate files. Cross-file init order in Go is
// declaration-order within a file, then alphabetical filename order
// across files — fragile to filename changes, so the smell flags any
// case with non-trivial cross-file ordering surface.
func TestG7_InitCoupling(t *testing.T) {
	pkgs := fakeModule(t, map[string]string{
		"a.go": "package foo\n\nimport \"log\"\n\nfunc init() { log.Println(\"a\") }\n",
		"b.go": "package foo\n\nimport \"log\"\n\nfunc init() { log.Println(\"b\") }\n",
	})
	findings := scanInitCoupling(pkgs)
	if !containsID(findings, "G7") {
		t.Fatalf("expected G7, got %v", findingIDs(findings))
	}
}

// TestG7_SingleFile_NoFire ensures multiple init() funcs in a single
// file do not fire — within one file the order is unambiguous.
func TestG7_SingleFile_NoFire(t *testing.T) {
	pkgs := fakeModule(t, map[string]string{
		"a.go": "package foo\n\nimport \"log\"\n\nfunc init() { log.Println(\"1\") }\nfunc init() { log.Println(\"2\") }\n",
	})
	findings := scanInitCoupling(pkgs)
	if containsID(findings, "G7") {
		t.Fatalf("did not expect G7 for single-file inits, got %+v", findings)
	}
}
