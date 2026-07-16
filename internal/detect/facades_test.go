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

func TestG6_TypeConversion_NoFire(t *testing.T) {
	pkgs := fakeModule(t, map[string]string{
		"inner/inner.go": "package inner\n\ntype ID int\n",
		"outer/outer.go": `package outer

import "example.com/test/inner"

type Service struct{}
func (Service) ID(n int) inner.ID { return inner.ID(n) }
`,
	})
	if findings := ScanFacades(pkgs); containsID(findings, "G6") {
		t.Fatalf("did not expect G6 for a type conversion, got %+v", findings)
	}
}

func TestG6_MultilineIfPrefix_NoFire(t *testing.T) {
	pkgs := fakeModule(t, map[string]string{
		"inner/inner.go": "package inner\n\nfunc Compute(n int) int { return n }\n",
		"outer/outer.go": `package outer

import "example.com/test/inner"

type Service struct{}
func (Service) Compute(n int) int {
	if n < 0 {
		return 0
	}
	return inner.Compute(n)
}
`,
	})
	if findings := ScanFacades(pkgs); containsID(findings, "G6") {
		t.Fatalf("did not expect G6 for substantive multiline logic, got %+v", findings)
	}
}

func TestG6_StateBindingAndStdlibBoundaryAreLow(t *testing.T) {
	pkgs := fakeModule(t, map[string]string{
		"inner/inner.go": "package inner\n\nfunc Fetch(base, id string) string { return base + id }\n",
		"outer/outer.go": `package outer

import (
	"example.com/test/inner"
	"time"
)

type Client struct{ base string }
func (c Client) Fetch(id string) string { return inner.Fetch(c.base, id) }

type Clock struct{}
func (Clock) Now() time.Time { return time.Now() }
`,
	})
	findings := ScanFacades(pkgs)
	if len(findings) != 2 {
		t.Fatalf("expected two contextual facade findings, got %+v", findings)
	}
	for _, finding := range findings {
		if finding.Severity != "LOW" {
			t.Fatalf("expected contextual facade to be LOW, got %+v", finding)
		}
	}
}

func TestG6_AliasReceiverAndEmbeddedInterface(t *testing.T) {
	pkgs := fakeModule(t, map[string]string{
		"inner/inner.go": `package inner

type Contract interface{ Marker() }
func Compute(n int) int { return n }
`,
		"outer/outer.go": `package outer

import "example.com/test/inner"

type real struct{}
type Alias = real
func (*Alias) Compute(n int) int { return inner.Compute(n) }

type Adapter struct { inner.Contract }
func (*Adapter) Compute(n int) int { return inner.Compute(n) }
`,
	})
	findings := ScanFacades(pkgs)
	if len(findings) != 2 {
		t.Fatalf("expected alias receiver and interface adapter findings, got %+v", findings)
	}
	seenReal, seenInterface := false, false
	for _, finding := range findings {
		if finding.Evidence["receiver"] == "real" {
			seenReal = true
		}
		if finding.Evidence["classification"] == "interface_dispatch" && finding.Severity == "LOW" {
			seenInterface = true
		}
	}
	if !seenReal || !seenInterface {
		t.Fatalf("alias or embedded-interface classification missing: %+v", findings)
	}
}

func TestG6_StandardInterfaceContractsDoNotFire(t *testing.T) {
	pkgs := fakeModule(t, map[string]string{
		"outer/outer.go": `package outer

import (
	"errors"
	"fmt"
)

type Problem struct{ cause error }

func (p Problem) Error() string { return fmt.Sprintf("problem: %v", p.cause) }
func (p Problem) String() string { return fmt.Sprintf("problem: %v", p.cause) }
func (p Problem) Unwrap() error { return errors.Unwrap(p.cause) }
`,
	})
	if findings := ScanFacades(pkgs); containsID(findings, "G6") {
		t.Fatalf("did not expect interface-contract methods to be facades, got %+v", findings)
	}
}

func TestG6_ContractNameWithDifferentSignatureStillFires(t *testing.T) {
	pkgs := fakeModule(t, map[string]string{
		"inner/inner.go": "package inner\n\nfunc Render(prefix string) string { return prefix }\n",
		"outer/outer.go": `package outer

import "example.com/test/inner"

type Value struct{}
func (Value) String(prefix string) string { return inner.Render(prefix) }
`,
	})
	if findings := ScanFacades(pkgs); !containsID(findings, "G6") {
		t.Fatalf("expected non-contract String signature to remain detectable, got %+v", findings)
	}
}
