package audit

import "testing"

func TestApplySuppressionsSupportsFindingAndDetectorSelectors(t *testing.T) {
	findings := []Finding{
		{SmellID: "G5", Location: "cmd/server/cli_perf.go"},
		{SmellID: "G5", Location: "internal/chat/runtime.go"},
		{SmellID: "G6", Location: "internal/chat/tool_errors.go:ToolError.Error"},
		{SmellID: "G12", Location: "internal/intentional"},
	}
	kept, count, err := ApplySuppressions(findings, []string{
		"G5@cmd/server/cli_perf.go",
		"G6@internal/chat/tool_errors.go:ToolError.Error",
		"G12",
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("suppressed count = %d, want 3", count)
	}
	if len(kept) != 1 || kept[0].Location != "internal/chat/runtime.go" {
		t.Fatalf("unexpected kept findings: %+v", kept)
	}
}

func TestValidateSuppressionsRejectsMalformedSelectors(t *testing.T) {
	for _, selector := range []string{"", "@path", "G5@", "G 5@path"} {
		if err := ValidateSuppressions([]string{selector}); err == nil {
			t.Errorf("ValidateSuppressions(%q): expected error", selector)
		}
	}
}
