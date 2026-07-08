package audit

import "testing"

func TestParseSeverity(t *testing.T) {
	for _, name := range []string{"low", "LOW", "Medium", "high", "critical"} {
		if _, ok := ParseSeverity(name); !ok {
			t.Errorf("ParseSeverity(%q) not ok", name)
		}
	}
	for _, name := range []string{"", "warn", "sev1"} {
		if _, ok := ParseSeverity(name); ok {
			t.Errorf("ParseSeverity(%q) unexpectedly ok", name)
		}
	}
}

func TestSeverityAtLeast(t *testing.T) {
	if !SevCritical.AtLeast(SevLow) || !SevHigh.AtLeast(SevHigh) {
		t.Error("more/equally severe values must satisfy AtLeast")
	}
	if SevLow.AtLeast(SevMedium) {
		t.Error("LOW must not be at least MEDIUM")
	}
}
