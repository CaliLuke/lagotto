package main

import (
	"testing"
)

// TestG1D_HiddenHolder fires on the registry-map evasion: a thin
// holder type, several package-level sync.Map registries keyed by
// the holder's pointer, and exported accessor functions that look up
// per-instance services from those registries. Callers still receive
// *Holder everywhere — the receiver split is cosmetic.
func TestG1D_HiddenHolder(t *testing.T) {
	pkgs := fakeModule(t, map[string]string{
		"a.go": `package foo

import "sync"

type Holder struct{ raw int }

type Mutator struct{}
type Searcher struct{}
type Threads struct{}
type Promotions struct{}
type Checks struct{}

var (
	nodeRegistry      sync.Map
	edgeRegistry      sync.Map
	searchRegistry    sync.Map
	threadRegistry    sync.Map
	promotionRegistry sync.Map
)

func NodeOps(h *Holder) *Mutator { v, _ := nodeRegistry.Load(h); return v.(*Mutator) }
func EdgeOps(h *Holder) *Mutator { v, _ := edgeRegistry.Load(h); return v.(*Mutator) }
func SearchOps(h *Holder) *Searcher { v, _ := searchRegistry.Load(h); return v.(*Searcher) }
func ThreadOps(h *Holder) *Threads { v, _ := threadRegistry.Load(h); return v.(*Threads) }
func PromotionOps(h *Holder) *Promotions { v, _ := promotionRegistry.Load(h); return v.(*Promotions) }
`,
	})
	findings := scanReceivers(pkgs)
	if !containsID(findings, "G1D") {
		t.Fatalf("expected G1D, got %v", findingIDs(findings))
	}
}

// TestG1D_RealReceiver_NoFire ensures that a holder which actually
// owns its API as methods (not via package-level accessors) is NOT
// flagged as G1D; it is G1's territory.
func TestG1D_RealReceiver_NoFire(t *testing.T) {
	pkgs := fakeModule(t, map[string]string{
		"a.go": `package foo

import "sync"

type Holder struct{ raw int }

var (
	cache1 sync.Map
	cache2 sync.Map
	cache3 sync.Map
)

func (h *Holder) Method1() {}
func (h *Holder) Method2() {}
func (h *Holder) Method3() {}
func (h *Holder) Method4() {}
func (h *Holder) Method5() {}
`,
	})
	findings := scanHiddenHolders(pkgs[0])
	if containsID(findings, "G1D") {
		t.Fatalf("did not expect G1D for real-receiver holder, got %+v", findings)
	}
}

// TestG1D_FewRegistries_NoFire ensures the threshold (≥3 registry
// vars) is enforced; a single legitimate cache should not fire.
func TestG1D_FewRegistries_NoFire(t *testing.T) {
	pkgs := fakeModule(t, map[string]string{
		"a.go": `package foo

import "sync"

type Holder struct{ raw int }
type Sub struct{}

var oneCache sync.Map

func A(h *Holder) *Sub { return nil }
func B(h *Holder) *Sub { return nil }
func C(h *Holder) *Sub { return nil }
func D(h *Holder) *Sub { return nil }
func E(h *Holder) *Sub { return nil }
`,
	})
	findings := scanHiddenHolders(pkgs[0])
	if containsID(findings, "G1D") {
		t.Fatalf("did not expect G1D with only 1 registry, got %+v", findings)
	}
}

// TestG1D_FewAccessors_NoFire ensures the threshold (≥5 accessor
// funcs) is enforced.
func TestG1D_FewAccessors_NoFire(t *testing.T) {
	pkgs := fakeModule(t, map[string]string{
		"a.go": `package foo

import "sync"

type Holder struct{ raw int }
type Sub struct{}

var (
	r1 sync.Map
	r2 sync.Map
	r3 sync.Map
)

func A(h *Holder) *Sub { return nil }
func B(h *Holder) *Sub { return nil }
`,
	})
	findings := scanHiddenHolders(pkgs[0])
	if containsID(findings, "G1D") {
		t.Fatalf("did not expect G1D with only 2 accessors, got %+v", findings)
	}
}

// TestG1D_PointerKeyedMap_Fires ensures the detector also catches
// the registry-pattern when the package uses raw map[*T]*Sub instead
// of sync.Map.
func TestG1D_PointerKeyedMap_Fires(t *testing.T) {
	pkgs := fakeModule(t, map[string]string{
		"a.go": `package foo

type Holder struct{ raw int }
type Sub struct{}

var (
	r1 = map[*Holder]*Sub{}
	r2 = map[*Holder]*Sub{}
	r3 = map[*Holder]*Sub{}
)

func A(h *Holder) *Sub { return r1[h] }
func B(h *Holder) *Sub { return r2[h] }
func C(h *Holder) *Sub { return r3[h] }
func D(h *Holder) *Sub { return r1[h] }
func E(h *Holder) *Sub { return r2[h] }
`,
	})
	findings := scanHiddenHolders(pkgs[0])
	if !containsID(findings, "G1D") {
		t.Fatalf("expected G1D for raw pointer-keyed map registries, got %v", findingIDs(findings))
	}
}
