package audit

import "testing"

func TestConsolidateRepositoryPatternsGroupsLargeG12Sets(t *testing.T) {
	findings := []Finding{{SmellID: "G9"}}
	for _, location := range []string{"e", "d", "c", "b", "a"} {
		findings = append(findings, Finding{SmellID: "G12", Location: location, Evidence: map[string]any{"file": location + ".go"}})
	}
	got := ConsolidateRepositoryPatterns(findings)
	if len(got) != 2 || got[1].SmellID != "G12" || got[1].Location != "." {
		t.Fatalf("expected one grouped G12 plus unrelated finding, got %+v", got)
	}
	if got[1].Evidence["package_count"] != 5 || got[1].Severity != SevLow {
		t.Fatalf("unexpected grouped evidence: %+v", got[1])
	}
}

func TestConsolidateRepositoryPatternsKeepsSmallG12Sets(t *testing.T) {
	findings := []Finding{{SmellID: "G12", Location: "a"}, {SmellID: "G12", Location: "b"}}
	got := ConsolidateRepositoryPatterns(findings)
	if len(got) != len(findings) || got[0].Location != "a" {
		t.Fatalf("small sets should remain actionable, got %+v", got)
	}
}
