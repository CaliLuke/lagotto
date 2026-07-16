package detect

import "testing"

// TestG7_InitCoupling fires when a package has multiple init()
// functions across separate files. Cross-file init order in Go is
// declaration-order within a file, then alphabetical filename order
// across files — fragile to filename changes, so the smell flags any
// case with non-trivial cross-file ordering surface.
func TestG7_InitCoupling(t *testing.T) {
	pkgs := fakeModule(t, map[string]string{
		"a.go": "package foo\n\nvar step int\n\nfunc init() { step = 1 }\n",
		"b.go": "package foo\n\nimport \"log\"\n\nfunc init() { log.Println(step) }\n",
	})
	findings := ScanInitCoupling(pkgs)
	if !containsID(findings, "G7") {
		t.Fatalf("expected G7, got %v", findingIDs(findings))
	}
}

func TestG7_IndependentRegistrations_NoFire(t *testing.T) {
	pkgs := fakeModule(t, map[string]string{
		"registry/registry.go": "package registry\n\nfunc Register(string) {}\n",
		"a.go":                 "package foo\n\nimport \"example.com/test/registry\"\n\nfunc init() { registry.Register(\"a\") }\n",
		"b.go":                 "package foo\n\nimport \"example.com/test/registry\"\n\nfunc init() { registry.Register(\"b\") }\n",
	})
	if findings := ScanInitCoupling(pkgs); containsID(findings, "G7") {
		t.Fatalf("did not expect G7 for independent registration inits, got %+v", findings)
	}
}

// TestG7_SingleFile_NoFire ensures multiple init() funcs in a single
// file do not fire — within one file the order is unambiguous.
func TestG7_SingleFile_NoFire(t *testing.T) {
	pkgs := fakeModule(t, map[string]string{
		"a.go": "package foo\n\nimport \"log\"\n\nfunc init() { log.Println(\"1\") }\nfunc init() { log.Println(\"2\") }\n",
	})
	findings := ScanInitCoupling(pkgs)
	if containsID(findings, "G7") {
		t.Fatalf("did not expect G7 for single-file inits, got %+v", findings)
	}
}
