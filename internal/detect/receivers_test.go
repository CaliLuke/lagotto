package detect

import (
	"fmt"
	"strings"
	"testing"

	"github.com/CaliLuke/lagotto/internal/audit"
)

// TestG1_ReceiverMonolith_Direct verifies the headline detector fires
// when a single named struct owns ≥15 methods spanning ≥3 concerns.
// The synthetic Big type spans create/read/update/delete to cross the
// concern threshold without any embedding tricks.
func TestG1_ReceiverMonolith_Direct(t *testing.T) {
	src := `package foo

type Big struct{}

`
	for _, m := range []string{
		"CreateA", "CreateB", "CreateC", "CreateD",
		"GetA", "GetB", "GetC", "GetD",
		"UpdateA", "UpdateB", "UpdateC",
		"DeleteA", "DeleteB", "DeleteC", "DeleteD",
	} {
		src += "func (b *Big) " + m + "() {}\n"
	}
	pkgs := fakeModule(t, map[string]string{"a.go": src})
	findings := ScanReceivers(pkgs)
	if !containsID(findings, "G1") {
		t.Fatalf("expected G1, got %v", findingIDs(findings))
	}
}

// TestG1_CohesiveLowLevelType_NoFire ensures a Conn-style type with
// many methods at one abstraction level is NOT flagged. Concerns
// gating filters out types that don't span verb groups.
func TestG1_CohesiveLowLevelType_NoFire(t *testing.T) {
	src := `package foo

type Conn struct{}
`
	for _, m := range []string{
		"Read1", "Read2", "Read3", "Read4", "Read5",
		"Read6", "Read7", "Read8", "Read9", "Read10",
		"Read11", "Read12", "Read13", "Read14", "Read15",
		"Read16",
	} {
		src += "func (c *Conn) " + m + "() {}\n"
	}
	pkgs := fakeModule(t, map[string]string{"a.go": src})
	findings := ScanReceivers(pkgs)
	if containsID(findings, "G1") {
		t.Fatalf("did not expect G1 on uniform-concern type; got %+v", findings)
	}
}

// TestG1_PromotedViaEmbedding ensures methods promoted onto an outer
// struct via a same-package embedded pointer count toward the outer
// type's effective method set. This is the "embedding theatre"
// disguise: the outer god type is rebuilt by composition while
// callers still see one wide method set.
func TestG1_PromotedViaEmbedding(t *testing.T) {
	src := `package foo

type inner struct{}

type Outer struct{ *inner }

`
	for _, m := range []string{
		"CreateA", "CreateB", "CreateC", "CreateD",
		"GetA", "GetB", "GetC", "GetD",
		"UpdateA", "UpdateB", "UpdateC",
		"DeleteA", "DeleteB", "DeleteC", "DeleteD",
	} {
		src += "func (i *inner) " + m + "() {}\n"
	}
	pkgs := fakeModule(t, map[string]string{"a.go": src})
	findings := ScanReceivers(pkgs)
	// The detector should report the outer type with a "promoted"
	// hint, since callers see the methods on *Outer.
	var outer *audit.Finding
	for i := range findings {
		if findings[i].SmellID == "G1" && findings[i].Evidence["type"] == "Outer" {
			outer = &findings[i]
			break
		}
	}
	if outer == nil {
		t.Fatalf("expected G1 on Outer, got %+v", findings)
	}
	if !strings.Contains(outer.Message, "promoted") {
		t.Errorf("expected 'promoted' note in message, got %q", outer.Message)
	}
}

// TestG1_TestDoubleSkipped ensures Fake/Mock/Stub-prefixed types are
// excluded — they legitimately implement wide interfaces and should
// not be reported even when they pass the method-count threshold.
func TestG1_TestDoubleSkipped(t *testing.T) {
	src := `package foo

type FakeStore struct{}

`
	for _, m := range []string{
		"CreateA", "CreateB", "GetA", "GetB", "GetC", "GetD",
		"UpdateA", "UpdateB", "UpdateC", "DeleteA", "DeleteB",
		"DeleteC", "DeleteD", "DeleteE", "DeleteF",
	} {
		src += "func (f *FakeStore) " + m + "() {}\n"
	}
	pkgs := fakeModule(t, map[string]string{"a.go": src})
	findings := ScanReceivers(pkgs)
	if containsID(findings, "G1") {
		t.Fatalf("expected test-double Fake* to be skipped, got %+v", findings)
	}
}

// TestG1B_AliasCluster fires when 3+ exported aliases in one package
// resolve to a single underlying named type — the canonical
// Decomposition Theatre pattern.
func TestG1B_AliasCluster(t *testing.T) {
	pkgs := fakeModule(t, map[string]string{"a.go": `package foo

type ops struct{}

type Mutator   = ops
type Searcher  = ops
type Threads   = ops
type CheckRunner = ops
`})
	findings := ScanReceivers(pkgs)
	if !containsID(findings, "G1B") {
		t.Fatalf("expected G1B, got %v", findingIDs(findings))
	}
}

// TestG1B_TwoAliases_NoFire ensures the threshold (3+) is enforced.
// Two aliases is a stylistic choice, not the theatre pattern.
func TestG1B_TwoAliases_NoFire(t *testing.T) {
	pkgs := fakeModule(t, map[string]string{"a.go": `package foo

type impl struct{}

type Mutator  = impl
type Searcher = impl
`})
	findings := ScanReceivers(pkgs)
	if containsID(findings, "G1B") {
		t.Fatalf("did not expect G1B with only 2 aliases, got %+v", findings)
	}
}

// TestG1B_AliasToExternalType_NoFire ensures aliases pointing at
// types in a different package (a legitimate re-export shape) do
// not trigger G1B. The detector targets in-package collapsing.
func TestG1B_AliasToExternalType_NoFire(t *testing.T) {
	pkgs := fakeModule(t, map[string]string{
		"go.mod":         "module example.com/test\ngo 1.21\n",
		"inner/inner.go": "package inner\n\ntype Real struct{}\n",
		"outer/outer.go": `package outer

import "example.com/test/inner"

type A = inner.Real
type B = inner.Real
type C = inner.Real
type D = inner.Real
`,
	})
	findings := ScanReceivers(pkgs)
	if containsID(findings, "G1B") {
		t.Fatalf("did not expect G1B for cross-package alias re-export, got %+v", findings)
	}
}

// TestG1C_AggregateHolder fires when a struct aggregates ≥5
// same-package sub-services whose pointee method counts total ≥25.
// This is the second-stage theatre: aliases replaced by named fields,
// but the sub-services still live in the same package as the holder.
func TestG1C_AggregateHolder(t *testing.T) {
	src := `package foo

type Sub1 struct{}
type Sub2 struct{}
type Sub3 struct{}
type Sub4 struct{}
type Sub5 struct{}

type Holder struct {
	A *Sub1
	B *Sub2
	C *Sub3
	D *Sub4
	E *Sub5
}

`
	for _, sub := range []string{"Sub1", "Sub2", "Sub3", "Sub4", "Sub5"} {
		for _, m := range []string{"M1", "M2", "M3", "M4", "M5", "M6"} {
			src += "func (s *" + sub + ") " + m + "() {}\n"
		}
	}
	pkgs := fakeModule(t, map[string]string{"a.go": src})
	findings := ScanReceivers(pkgs)
	if !containsID(findings, "G1C") {
		t.Fatalf("expected G1C, got %v", findingIDs(findings))
	}
}

// TestG1C_CrossPackageHolder_NoFire ensures a holder whose fields
// point at types defined in OTHER packages is treated as the
// legitimate end state (each sub-service already lives elsewhere)
// and is not flagged.
func TestG1C_CrossPackageHolder_NoFire(t *testing.T) {
	files := map[string]string{
		"go.mod": "module example.com/test\ngo 1.21\n",
	}
	for i, name := range []string{"a", "b", "c", "d", "e"} {
		_ = i
		files[name+"/"+name+".go"] = `package ` + name + `

type Sub struct{}

func (s *Sub) M1() {}
func (s *Sub) M2() {}
func (s *Sub) M3() {}
func (s *Sub) M4() {}
func (s *Sub) M5() {}
func (s *Sub) M6() {}
`
	}
	files["holder/holder.go"] = `package holder

import (
	"example.com/test/a"
	"example.com/test/b"
	"example.com/test/c"
	"example.com/test/d"
	"example.com/test/e"
)

type Holder struct {
	A *a.Sub
	B *b.Sub
	C *c.Sub
	D *d.Sub
	E *e.Sub
}
`
	pkgs := fakeModule(t, files)
	findings := ScanReceivers(pkgs)
	if containsID(findings, "G1C") {
		t.Fatalf("did not expect G1C for cross-package holder, got %+v", findings)
	}
}

func TestG1C_ForeignPromotedMethodsDoNotCount(t *testing.T) {
	src := `package foo

import "sync"

type Sub1 struct{ sync.RWMutex }
type Sub2 struct{ sync.RWMutex }
type Sub3 struct{ sync.RWMutex }
type Sub4 struct{ sync.RWMutex }
type Sub5 struct{ sync.RWMutex }

type Holder struct {
	A *Sub1
	B *Sub2
	C *Sub3
	D *Sub4
	E *Sub5
}
`
	if findings := ScanReceivers(fakeModule(t, map[string]string{"a.go": src})); containsID(findings, "G1C") {
		t.Fatalf("did not expect embedded sync.RWMutex methods to count toward G1C, got %+v", findings)
	}
}

func TestG1C_AliasTypedFieldsStillCount(t *testing.T) {
	src := `package foo

type Sub1 struct{}
type Sub2 struct{}
type Sub3 struct{}
type Sub4 struct{}
type Sub5 struct{}
type Alias1 = Sub1
type Alias2 = Sub2
type Alias3 = Sub3
type Alias4 = Sub4
type Alias5 = Sub5
type Holder struct { A *Alias1; B *Alias2; C *Alias3; D *Alias4; E *Alias5 }
`
	for i := 1; i <= 5; i++ {
		for j := 1; j <= 5; j++ {
			src += fmt.Sprintf("func (*Sub%d) M%d() {}\n", i, j)
		}
	}
	if findings := ScanReceivers(fakeModule(t, map[string]string{"a.go": src})); !containsID(findings, "G1C") {
		t.Fatalf("expected alias-typed fields to count toward G1C, got %v", findingIDs(findings))
	}
}

func TestG1C_TestDoubleHolderSkipped(t *testing.T) {
	src := `package foo
type Sub1 struct{}
type Sub2 struct{}
type Sub3 struct{}
type Sub4 struct{}
type Sub5 struct{}
type MockHolder struct { A *Sub1; B *Sub2; C *Sub3; D *Sub4; E *Sub5 }
`
	for i := 1; i <= 5; i++ {
		for j := 1; j <= 5; j++ {
			src += fmt.Sprintf("func (*Sub%d) M%d() {}\n", i, j)
		}
	}
	if findings := ScanReceivers(fakeModule(t, map[string]string{"a.go": src})); containsID(findings, "G1C") {
		t.Fatalf("did not expect a Mock holder to fire G1C, got %+v", findings)
	}
}

func TestG1_PromoterTieIsDeterministic(t *testing.T) {
	src := "package foo\n\ntype alpha struct{}\ntype beta struct{}\ntype Outer struct { *alpha; *beta }\n"
	methods := []string{"CreateA", "CreateB", "GetA", "GetB", "UpdateA", "UpdateB", "DeleteA", "DeleteB"}
	for _, receiver := range []string{"alpha", "beta"} {
		for _, method := range methods {
			src += "func (*" + receiver + ") " + method + receiver + "() {}\n"
		}
	}
	var message string
	for i := 0; i < 10; i++ {
		findings := ScanReceivers(fakeModule(t, map[string]string{"a.go": src}))
		for _, finding := range findings {
			if finding.SmellID == "G1" && finding.Evidence["type"] == "Outer" {
				if message == "" {
					message = finding.Message
				} else if finding.Message != message {
					t.Fatalf("promoter tie changed message: %q vs %q", message, finding.Message)
				}
			}
		}
	}
	if message == "" {
		t.Fatal("expected Outer finding")
	}
	if name, count := dominantPromoter(map[string]int{"beta": 8, "alpha": 8}); name != "alpha" || count != 8 {
		t.Fatalf("dominantPromoter tie = %q, %d; want alpha, 8", name, count)
	}
}
