package audit

import (
	"bytes"
	"encoding/json"
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
