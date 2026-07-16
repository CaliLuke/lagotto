package detect

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/packages"

	"github.com/CaliLuke/lagotto/internal/audit"
)

func TestArgRootAcceptsPatternSuffix(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{nil, "."},
		{[]string{"."}, "."},
		{[]string{"./..."}, "."},
		{[]string{"..."}, "."},
		{[]string{"internal/..."}, "internal"},
		{[]string{"./internal/..."}, "./internal"},
		{[]string{"internal"}, "internal"},
	}
	for _, c := range cases {
		if got := argRoot(c.args); got != c.want {
			t.Errorf("argRoot(%v) = %q, want %q", c.args, got, c.want)
		}
	}
}

func TestReportOutcomeLoadErrorsFailBeforeFindingThreshold(t *testing.T) {
	report := &audit.Report{
		LoadErrors: []string{"example.com/broken: type error"},
		Findings: []audit.Finding{{
			Severity: audit.SevCritical,
		}},
	}
	err := reportOutcome(report, "high")
	var incomplete *audit.IncompleteLoadError
	if !errors.As(err, &incomplete) {
		t.Fatalf("expected IncompleteLoadError, got %T: %v", err, err)
	}
	if incomplete.Count != 1 {
		t.Fatalf("load error count = %d, want 1", incomplete.Count)
	}
}

func TestReportOutcomeFindingsStillUseExitTwoError(t *testing.T) {
	report := &audit.Report{Findings: []audit.Finding{{Severity: audit.SevHigh}}}
	err := reportOutcome(report, "high")
	var findings *audit.FindingsError
	if !errors.As(err, &findings) {
		t.Fatalf("expected FindingsError, got %T: %v", err, err)
	}
}

func TestRunScanReturnsIncompleteErrorAfterPackageFailure(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"go.mod":    "module example.com/broken\ngo 1.26\n",
		"broken.go": "package broken\n\nfunc F() int { return \"wrong\" }\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	err := runScan(&Flags{Format: "json"}, []string{root}, func(string, []*packages.Package) []audit.Finding {
		return nil
	})
	var incomplete *audit.IncompleteLoadError
	if !errors.As(err, &incomplete) {
		t.Fatalf("expected IncompleteLoadError, got %T: %v", err, err)
	}
}
