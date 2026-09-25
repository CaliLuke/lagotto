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

func TestApplySuppressionsMatchesWholeLocationComponents(t *testing.T) {
	findings := []Finding{
		{SmellID: "G5", Location: "cmd/server/main.go"},
		{SmellID: "G5", Location: "cmd/serverless/main.go"},
		{SmellID: "G14", Location: "internal/api/handler.go:Server.Get"},
		{SmellID: "G14", Location: "internal/apiv2/handler.go:Server.Get"},
		{SmellID: "G1", Location: "internal/store (Store)"},
		{SmellID: "G1", Location: "internal/storefront (Store)"},
		{SmellID: "G12", Location: "internal/a.go2"},
	}
	kept, count, err := ApplySuppressions(findings, []string{
		"G5@cmd/server/",
		"G14@internal/api",
		"G1@internal/store",
		"G12@internal/a.go",
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("suppressed count = %d, want 3", count)
	}
	want := []string{"cmd/serverless/main.go", "internal/apiv2/handler.go:Server.Get", "internal/storefront (Store)", "internal/a.go2"}
	if len(kept) != len(want) {
		t.Fatalf("kept %+v, want locations %v", kept, want)
	}
	for i, finding := range kept {
		if finding.Location != want[i] {
			t.Errorf("kept[%d] = %q, want %q", i, finding.Location, want[i])
		}
	}
}
