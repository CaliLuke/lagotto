package audit

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

// captureStdout runs fn with os.Stdout redirected to a pipe and
// returns what it wrote.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()
	fn()
	w.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestEmitJSONEmptyFindingsIsArray(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Emit(&Report{Root: "."}, "json"); err != nil {
			t.Error(err)
		}
	})
	var decoded struct {
		Findings []Finding `json:"findings"`
	}
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if !bytes.Contains([]byte(out), []byte(`"findings": []`)) {
		t.Errorf("empty report must emit \"findings\": [], got:\n%s", out)
	}
}

func TestEmitJSONIncludesLoadErrors(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Emit(&Report{Root: ".", LoadErrors: []string{"pkg: boom"}}, "json"); err != nil {
			t.Error(err)
		}
	})
	var decoded Report
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if len(decoded.LoadErrors) != 1 || decoded.LoadErrors[0] != "pkg: boom" {
		t.Errorf("load_errors not round-tripped: %#v", decoded.LoadErrors)
	}
}

func TestEmitJSONIncludesVersionAndEffectiveConfiguration(t *testing.T) {
	report := &Report{
		Version: "0.2.4-dev",
		Root:    ".",
		Configuration: EffectiveConfiguration{
			Exclude: []string{"vendor"},
			Mixed: MixedConfiguration{
				MinLines:                     600,
				MinComponentMembers:          2,
				MinComponentLines:            40,
				MinSingleComponentComplexity: 5,
				Severity:                     SevMedium,
				CohesiveMinLines:             1200,
			},
			LayerPolicy: []LayerPolicyConfiguration{{
				Name:                       "thin-transport",
				Paths:                      []string{"internal/transport/**"},
				Dependencies:               []string{"internal/service/**"},
				GeneratedTypes:             []string{"gen/**"},
				MaxCoordinatedDependencies: 1,
				Severity:                   SevMedium,
			}},
		},
	}
	out := captureStdout(t, func() {
		if err := Emit(report, "json"); err != nil {
			t.Error(err)
		}
	})
	var decoded Report
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Version != "0.2.4-dev" || decoded.Configuration.Mixed.CohesiveMinLines != 1200 || decoded.Configuration.Mixed.MinSingleComponentComplexity != 5 || len(decoded.Configuration.LayerPolicy) != 1 {
		t.Fatalf("reproducibility metadata did not round-trip: %+v", decoded)
	}
}

func TestValidateFormat(t *testing.T) {
	if err := ValidateFormat("json"); err != nil {
		t.Error(err)
	}
	if err := ValidateFormat("text"); err != nil {
		t.Error(err)
	}
	if err := ValidateFormat("jsn"); err == nil {
		t.Error("expected error for unknown format")
	}
}

// failWriter errors after n bytes, simulating a full disk or closed
// pipe mid-report.
type failWriter struct{ left int }

func (w *failWriter) Write(p []byte) (int, error) {
	if len(p) > w.left {
		n := w.left
		w.left = 0
		return n, errWriteFailed
	}
	w.left -= len(p)
	return len(p), nil
}

var errWriteFailed = errors.New("write failed")

func TestEmitTextPropagatesWriteErrors(t *testing.T) {
	r := &Report{Root: ".", Findings: []Finding{{
		Smell: "X", SmellID: "G0", Severity: SevLow,
		Location: "loc", Message: "msg", Suggestion: "do things",
	}}}
	if err := emitText(&failWriter{left: 10}, r); !errors.Is(err, errWriteFailed) {
		t.Errorf("expected write error to propagate, got %v", err)
	}
	if err := emitText(&bytes.Buffer{}, r); err != nil {
		t.Errorf("unexpected error on healthy writer: %v", err)
	}
}
