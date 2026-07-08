package detect

import "testing"

// TestG6_FacadeMethod_PassThrough fires on a method whose body is a
// thin pass-through to a function in another package. The classic
// shape — `return other.Foo(args...)` — produces no value beyond
// indirection and should be removed in favor of direct calls.
func TestG6_FacadeMethod_PassThrough(t *testing.T) {
	pkgs := fakeModule(t, map[string]string{
		"go.mod":         "module example.com/test\ngo 1.21\n",
		"inner/inner.go": "package inner\n\nfunc Compute(n int) int { return n + 1 }\n",
		"outer/outer.go": `package outer

import "example.com/test/inner"

type Wrap struct{}

func (w *Wrap) Compute(n int) int { return inner.Compute(n) }
`,
	})
	findings := ScanFacades(pkgs)
	if !containsID(findings, "G6") {
		t.Fatalf("expected G6, got %v", findingIDs(findings))
	}
}

// TestG6_LocalCall_NoFire ensures methods that delegate to a function
// in their own package do NOT fire — those are legitimate internal
// composition and the smell only targets cross-package facades.
func TestG6_LocalCall_NoFire(t *testing.T) {
	pkgs := fakeModule(t, map[string]string{"a.go": `package foo

type Wrap struct{}

func compute(n int) int { return n + 1 }

func (w *Wrap) Compute(n int) int { return compute(n) }
`})
	findings := ScanFacades(pkgs)
	if containsID(findings, "G6") {
		t.Fatalf("did not expect G6 for in-package call, got %+v", findings)
	}
}

// TestG6_LongBody_NoFire ensures a method with substantive logic
// (more than the pass-through threshold) is not flagged even if
// it ultimately calls another package.
func TestG6_LongBody_NoFire(t *testing.T) {
	pkgs := fakeModule(t, map[string]string{
		"go.mod":         "module example.com/test\ngo 1.21\n",
		"inner/inner.go": "package inner\n\nfunc Compute(n int) int { return n + 1 }\n",
		"outer/outer.go": `package outer

import "example.com/test/inner"

type Wrap struct{}

func (w *Wrap) Compute(n int) int {
	if n < 0 {
		return 0
	}
	doubled := n * 2
	result := inner.Compute(doubled)
	return result + 1
}
`,
	})
	findings := ScanFacades(pkgs)
	if containsID(findings, "G6") {
		t.Fatalf("did not expect G6 for substantive method, got %+v", findings)
	}
}

// TestG6_EmptyReceiverList_NoPanic guards against the crash from
// issue #11: go/parser accepts `func () F() ...` (Recv != nil but
// len(Recv.List) == 0) with only a soft type error, and the scanner
// must skip it instead of panicking on Recv.List[0].
func TestG6_EmptyReceiverList_NoPanic(t *testing.T) {
	pkgs := fakeModule(t, map[string]string{
		"go.mod":         "module example.com/test\ngo 1.21\n",
		"inner/inner.go": "package inner\n\nfunc Compute(n int) int { return n + 1 }\n",
		"outer/outer.go": `package outer

import "example.com/test/inner"

func () Compute(n int) int { return inner.Compute(n) }
`,
	})
	findings := ScanFacades(pkgs)
	if containsID(findings, "G6") {
		t.Fatalf("did not expect G6 for receiverless method, got %v", findingIDs(findings))
	}
}
