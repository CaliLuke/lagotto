package detect

import "testing"

// TestG2_StutterNames fires when a package has exported identifiers
// that repeat the package name — the canonical `lanes.LaneConfig`
// shape that should be `lanes.Config` per Go style.
func TestG2_StutterNames(t *testing.T) {
	pkgs := fakeModule(t, map[string]string{"lanes/lanes.go": `package lanes

type LaneConfig struct{}
type LaneState int
func LaneNew() *LaneConfig { return nil }
`,
		"go.mod": "module example.com/test\ngo 1.21\n",
	})
	findings := ScanStutter(pkgs)
	if !containsID(findings, "G2") {
		t.Fatalf("expected G2, got %v", findingIDs(findings))
	}
}

// TestG2_NonStutter_NoFire ensures clean naming does not trigger.
func TestG2_NonStutter_NoFire(t *testing.T) {
	pkgs := fakeModule(t, map[string]string{"lanes/lanes.go": `package lanes

type Config struct{}
type State int
func New() *Config { return nil }
`,
		"go.mod": "module example.com/test\ngo 1.21\n",
	})
	findings := ScanStutter(pkgs)
	if containsID(findings, "G2") {
		t.Fatalf("did not expect G2 for clean names, got %+v", findings)
	}
}
