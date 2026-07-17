package detect

import (
	"strings"
	"testing"

	"github.com/CaliLuke/lagotto/internal/audit"
)

func largeSource(body string) string {
	for i := 0; i < 610; i++ {
		body += "// navigation padding\n"
	}
	return body
}

func TestMixedCmdExplainsComplexityIsPrioritizationOnly(t *testing.T) {
	flags := &Flags{Mixed: DefaultMixedOptions(), MixedSeverity: "medium"}
	if help := MixedCmd(flags).Long; !strings.Contains(help, "never trigger") || !strings.Contains(help, "prioritization only") {
		t.Fatalf("mixed help must explain complexity's post-candidate role, got %q", help)
	}
}

func TestG5_DisconnectedConcernClustersFire(t *testing.T) {
	body := largeSource(`package foo

type Parser struct{}
func (Parser) Parse(input string) string { return normalize(input) }
func normalize(input string) string { return input }

type Renderer struct{}
func (Renderer) Render(value string) string { return decorate(value) }
func decorate(value string) string { return "[" + value + "]" }
`)
	findings := ScanMixedConcern(fakeModule(t, map[string]string{"mixed.go": body}))
	if len(findings) != 1 || findings[0].Severity != "MEDIUM" {
		t.Fatalf("expected one MEDIUM G5 finding, got %+v", findings)
	}
	if !strings.Contains(findings[0].Message, "2 substantial disconnected") {
		t.Fatalf("expected disconnected-cluster evidence, got %q", findings[0].Message)
	}
	components, ok := findings[0].Evidence["substantial_components"].([]cohesionComponent)
	if !ok || len(components) != 2 {
		t.Fatalf("expected two evidence components with members, got %#v", findings[0].Evidence)
	}
	if len(components[0].Members) == 0 || components[0].Members[0].Name == "" {
		t.Fatalf("expected actionable member names, got %+v", components)
	}
}

func TestG5_InterfaceFamilyIsCohesive(t *testing.T) {
	body := largeSource(`package foo

type Filter interface { Validate() error }

type Equal struct{ Value string }
func (Equal) Validate() error { return nil }
func NewEqual(value string) Filter { return Equal{Value: value} }

type Range struct{ Min, Max int }
func (Range) Validate() error { return nil }
func NewRange(min, max int) Filter { return Range{Min: min, Max: max} }
`)
	if findings := ScanMixedConcern(fakeModule(t, map[string]string{"filter.go": body})); containsID(findings, "G5") {
		t.Fatalf("did not expect an interface implementation family to be split, got %+v", findings)
	}
}

func TestG5_DirectReferenceGraphIsCohesive(t *testing.T) {
	body := largeSource(`package foo

type Item struct{ Value string }
func (i Item) Save() error { return validateItem(i) }
func validateItem(i Item) error { return checkValue(i.Value) }
func checkValue(value string) error { return nil }
`)
	if findings := ScanMixedConcern(fakeModule(t, map[string]string{"item.go": body})); containsID(findings, "G5") {
		t.Fatalf("did not expect directly connected declarations to fire, got %+v", findings)
	}
}

func TestG5_SmallOrphanDoesNotManufactureConcern(t *testing.T) {
	body := largeSource(`package foo

type Item struct{ Value string }
func (i Item) Save() string { return i.Value }

func Version() string { return "v1" }
`)
	if findings := ScanMixedConcern(fakeModule(t, map[string]string{"item.go": body})); containsID(findings, "G5") {
		t.Fatalf("did not expect one tiny disconnected helper to manufacture a concern, got %+v", findings)
	}
}

func TestG5_OneLongDeclarationIsValidatedByComplexity(t *testing.T) {
	body := `package foo

type Parser struct{}
func (Parser) Parse(input string) string { return normalize(input) }
func normalize(input string) string { return input }

func ExtractAnnotations(input string) []string {
	if input == "" { return nil }
	if len(input) == 1 { return []string{input} }
	for _, value := range input {
		switch value {
		case 'a':
			continue
		case 'b':
			return []string{"b"}
		}
	}
`
	for i := 0; i < 35; i++ {
		body += "\t// annotation parsing step\n"
	}
	body += "\treturn nil\n}\n"
	body = largeSource(body)
	findings := ScanMixedConcern(fakeModule(t, map[string]string{"parser.go": body}))
	if len(findings) != 1 || findings[0].SmellID != "G5" {
		t.Fatalf("expected a long single declaration to qualify via the line threshold, got %+v", findings)
	}
	components, ok := findings[0].Evidence["substantial_components"].([]cohesionComponent)
	if !ok {
		t.Fatalf("unexpected component evidence: %#v", findings[0].Evidence)
	}
	foundSingleLong := false
	for _, component := range components {
		if component.PrimaryCount == 1 && component.LineCount >= 40 && component.CyclomaticMax >= 5 {
			foundSingleLong = true
		}
	}
	if !foundSingleLong {
		t.Fatalf("expected one-member component to pass post-candidate complexity validation, got %+v", components)
	}
}

func TestG5_OneLongStraightLineDeclarationIsRejectedAfterCandidateSelection(t *testing.T) {
	body := `package foo

type Parser struct{}
func (Parser) Parse(input string) string { return normalize(input) }
func normalize(input string) string { return input }

func LongWrapper(input string) string {
`
	for i := 0; i < 43; i++ {
		body += "\t// straight-line explanation\n"
	}
	body += "\treturn input\n}\n"
	body = largeSource(body)
	packages := fakeModule(t, map[string]string{"parser.go": body})
	if findings := ScanMixedConcern(packages); containsID(findings, "G5") {
		t.Fatalf("expected low-complexity one-member island to be rejected after candidate selection, got %+v", findings)
	}
	options := DefaultMixedOptions()
	options.MinSingleComponentComplexity = 0
	if findings := ScanMixedConcernWithOptions(packages, options); !containsID(findings, "G5") {
		t.Fatalf("expected zero complexity threshold to disable post-candidate rejection, got %+v", findings)
	}
}

func TestG5_SharedPackageObjectConnectsDeclarations(t *testing.T) {
	pkgs := fakeModule(t, map[string]string{
		"state.go": "package foo\n\nvar shared = map[string]string{}\n",
		"feature.go": largeSource(`package foo

type Reader struct{}
func (Reader) Read(key string) string { return shared[key] }

type Writer struct{}
func (Writer) Write(key, value string) { shared[key] = value }
`),
	})
	if findings := ScanMixedConcern(pkgs); containsID(findings, "G5") {
		t.Fatalf("did not expect declarations coupled through shared package state to fire, got %+v", findings)
	}
}

func TestG5_DotImportedDSLFamilyIsCohesive(t *testing.T) {
	pkgs := fakeModule(t, map[string]string{
		"dsl/dsl.go": `package dsl

func Type(name string, configure func()) string { configure(); return name }
func Field(name string) {}
`,
		"design/types.go": largeSource(`package design

import . "example.com/test/dsl"

var Account = Type("Account", func() { Field("name") })
var Product = Type("Product", func() { Field("title") })
var Project = Type("Project", func() { Field("status") })
`),
	})
	if findings := ScanMixedConcern(pkgs); containsID(findings, "G5") {
		t.Fatalf("did not expect a dot-imported DSL registration family to fire, got %+v", findings)
	}
}

func TestG5_DisconnectedFileBelowFloorDoesNotFire(t *testing.T) {
	body := `package foo

type Parser struct{}
func (Parser) Parse() {}
type Renderer struct{}
func (Renderer) Render() {}
`
	if findings := ScanMixedConcern(fakeModule(t, map[string]string{"small.go": body})); containsID(findings, "G5") {
		t.Fatalf("did not expect G5 below the 600-line floor, got %+v", findings)
	}
}

func TestG5_GeneratedFileDoesNotFire(t *testing.T) {
	body := largeSource(`// Code generated by schema. DO NOT EDIT.
package foo

type Parser struct{}
func (Parser) Parse() {}
type Renderer struct{}
func (Renderer) Render() {}
`)
	if findings := ScanMixedConcern(fakeModule(t, map[string]string{"generated.go": body})); containsID(findings, "G5") {
		t.Fatalf("did not expect generated code to fire, got %+v", findings)
	}
}

func TestG5_SuggestionNamesCandidateIsland(t *testing.T) {
	body := largeSource(`package foo

type Parser struct{}
func (Parser) Parse() {}
type Renderer struct{}
func (Renderer) Render() {}
`)
	findings := ScanMixedConcern(fakeModule(t, map[string]string{"mixed.go": body}))
	if len(findings) != 1 || !strings.Contains(findings[0].Suggestion, "candidate island") ||
		(!strings.Contains(findings[0].Suggestion, "Parser") && !strings.Contains(findings[0].Suggestion, "Renderer")) {
		t.Fatalf("expected a member-based split candidate, got %+v", findings)
	}
}

func TestG5_OptionsTuneThresholdsAndSeverity(t *testing.T) {
	body := largeSource(`package foo

type Parser struct{}
func (Parser) Parse() {}
type Renderer struct{}
func (Renderer) Render() {}
`)
	pkgs := fakeModule(t, map[string]string{"mixed.go": body})
	options := DefaultMixedOptions()
	options.Severity = audit.SevLow
	findings := ScanMixedConcernWithOptions(pkgs, options)
	if len(findings) != 1 || findings[0].Severity != audit.SevLow {
		t.Fatalf("expected configured LOW finding, got %+v", findings)
	}

	options.MinComponentMembers = 3
	options.MinComponentLines = 100
	if findings := ScanMixedConcernWithOptions(pkgs, options); containsID(findings, "G5") {
		t.Fatalf("expected component thresholds to suppress small islands, got %+v", findings)
	}

	options = DefaultMixedOptions()
	options.MinLines = 1000
	if findings := ScanMixedConcernWithOptions(pkgs, options); containsID(findings, "G5") {
		t.Fatalf("expected min-lines to suppress the file, got %+v", findings)
	}
}

func TestValidateMixedOptions(t *testing.T) {
	for name, mutate := range map[string]func(*MixedOptions){
		"lines":      func(options *MixedOptions) { options.MinLines = 0 },
		"members":    func(options *MixedOptions) { options.MinComponentMembers = 0 },
		"component":  func(options *MixedOptions) { options.MinComponentLines = 0 },
		"complexity": func(options *MixedOptions) { options.MinSingleComponentComplexity = -1 },
		"severity":   func(options *MixedOptions) { options.Severity = "UNKNOWN" },
		"cohesive":   func(options *MixedOptions) { options.CohesiveMinLines = -1 },
	} {
		t.Run(name, func(t *testing.T) {
			options := DefaultMixedOptions()
			mutate(&options)
			if err := ValidateMixedOptions(options); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestG13_LargeConnectedFileIsLowNavigationSignal(t *testing.T) {
	body := `package foo

type Item struct{ Value string }
func (i Item) Save() error { return validateItem(i) }
func validateItem(i Item) error { return checkValue(i.Value) }
func checkValue(value string) error { return nil }
`
	for i := 0; i < 1210; i++ {
		body += "// navigation padding\n"
	}
	findings := ScanMixedConcern(fakeModule(t, map[string]string{"item.go": body}))
	if len(findings) != 1 || findings[0].SmellID != "G13" || findings[0].Severity != audit.SevLow {
		t.Fatalf("expected one LOW G13 rather than G5, got %+v", findings)
	}
	if !strings.Contains(findings[0].Message, "1 substantial declaration/reference component") {
		t.Fatalf("expected G13 to report the actual substantial count, got %q", findings[0].Message)
	}
}

func TestG13_AccountsForZeroPrimaryGraphArtifacts(t *testing.T) {
	body := `package foo

type Item struct{ Value string }
func (i Item) Save() string { return i.Value }

var first = 1
var second = 2
`
	for i := 0; i < 1210; i++ {
		body += "// navigation padding\n"
	}
	findings := ScanMixedConcern(fakeModule(t, map[string]string{"item.go": body}))
	if len(findings) != 1 || findings[0].SmellID != "G13" {
		t.Fatalf("expected G13, got %+v", findings)
	}
	if findings[0].Evidence["component_count"] != 3 || findings[0].Evidence["ignored_component_count"] != 2 || findings[0].Evidence["minor_component_count"] != 0 {
		t.Fatalf("expected every graph component to be accounted for, got %#v", findings[0].Evidence)
	}
}

func TestG13_CanBeDisabled(t *testing.T) {
	body := "package foo\n\ntype Item struct{}\n"
	for i := 0; i < 1210; i++ {
		body += "// navigation padding\n"
	}
	options := DefaultMixedOptions()
	options.CohesiveMinLines = 0
	if findings := ScanMixedConcernWithOptions(fakeModule(t, map[string]string{"item.go": body}), options); containsID(findings, "G13") {
		t.Fatalf("expected G13 to be disabled, got %+v", findings)
	}
}

func TestG13_DotImportedDSLIsExcluded(t *testing.T) {
	body := `package design

import . "example.com/test/dsl"

var Account = Type("Account", func() { Field("name") })
`
	for i := 0; i < 1210; i++ {
		body += "// declarative padding\n"
	}
	pkgs := fakeModule(t, map[string]string{
		"dsl/dsl.go":      "package dsl\nfunc Type(name string, configure func()) string { configure(); return name }\nfunc Field(name string) {}\n",
		"design/types.go": body,
	})
	if findings := ScanMixedConcern(pkgs); containsID(findings, "G13") {
		t.Fatalf("did not expect a dot-imported DSL file to fire G13, got %+v", findings)
	}
}
