package main

import (
	"strings"
	"testing"
)

// TestG5_MixedConcernFile fires on a file mixing 3+ unrelated decl
// groups (types + methods + utilities + validation, etc.) over the
// 100-line threshold. The smell predicts the file is a junk drawer
// disguised as a module.
func TestG5_MixedConcernFile(t *testing.T) {
	body := "package foo\n\n"
	body += "type A struct{ N int }\n\ntype B struct{ M int }\n\n"
	body += "func (a A) Method() int {\n\treturn a.N\n}\n\n"
	body += "func ValidateInput(s string) bool {\n\treturn s != \"\"\n}\n\n"
	for i := 0; i < 60; i++ {
		body += "func Helper" + string(rune('A'+i%26)) + string(rune('a'+i/26)) + "(n int) int {\n\treturn n + 1\n}\n\n"
	}
	pkgs := fakeModule(t, map[string]string{"a.go": body})
	findings := scanMixedConcern(pkgs)
	if !containsID(findings, "G5") {
		t.Fatalf("expected G5, got %v", findingIDs(findings))
	}
	if !strings.Contains(findings[0].Message, "decl groups") {
		t.Errorf("expected message to mention decl groups, got %q", findings[0].Message)
	}
}

// TestG5_FocusedFile_NoFire ensures a file with one concern (just
// types, or just methods) does not trigger even if it's long.
func TestG5_FocusedFile_NoFire(t *testing.T) {
	body := "package foo\n\n"
	for i := 0; i < 80; i++ {
		body += "type T" + string(rune('A'+i%26)) + string(rune('a'+i/26)) + " struct {\n\tN int\n}\n\n"
	}
	pkgs := fakeModule(t, map[string]string{"a.go": body})
	findings := scanMixedConcern(pkgs)
	if containsID(findings, "G5") {
		t.Fatalf("did not expect G5 for single-concern file, got %+v", findings)
	}
}
