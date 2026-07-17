package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/CaliLuke/lagotto/internal/detect"
)

func TestApplyRepositoryConfigMergesSuppressionsAndHonorsCLIOverrides(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".lagotto.yaml")
	if err := os.WriteFile(path, []byte(`version: 1
suppress:
  - G5@tqlgen/parser.go
mixed:
  min_lines: 700
  min_component_members: 3
  min_component_lines: 55
  min_single_component_complexity: 7
  severity: low
  cohesive_min_lines: 1400
layer_policy:
  - name: thin-transport
    paths: [internal/transport/**]
    dependencies: [internal/service/**]
    generated_types: [gen/**]
`), 0o644); err != nil {
		t.Fatal(err)
	}

	defaults := detect.DefaultMixedOptions()
	flags := &detect.Flags{
		Suppress:      []string{"G11@driver"},
		Mixed:         defaults,
		MixedSeverity: "medium",
	}
	cmd := &cobra.Command{}
	cmd.Flags().IntVar(&flags.Mixed.MinLines, "min-lines", defaults.MinLines, "")
	cmd.Flags().IntVar(&flags.Mixed.MinComponentMembers, "min-component-members", defaults.MinComponentMembers, "")
	cmd.Flags().IntVar(&flags.Mixed.MinComponentLines, "min-component-lines", defaults.MinComponentLines, "")
	cmd.Flags().IntVar(&flags.Mixed.MinSingleComponentComplexity, "min-single-component-complexity", defaults.MinSingleComponentComplexity, "")
	cmd.Flags().IntVar(&flags.Mixed.CohesiveMinLines, "cohesive-min-lines", defaults.CohesiveMinLines, "")
	cmd.Flags().StringVar(&flags.MixedSeverity, "severity", "medium", "")
	if err := cmd.Flags().Set("min-lines", "900"); err != nil {
		t.Fatal(err)
	}

	if err := applyRepositoryConfig(cmd, []string{root}, flags); err != nil {
		t.Fatal(err)
	}
	if flags.LoadedConfigPath != path {
		t.Fatalf("loaded config = %q, want %q", flags.LoadedConfigPath, path)
	}
	if len(flags.Suppress) != 2 || flags.Suppress[0] != "G5@tqlgen/parser.go" || flags.Suppress[1] != "G11@driver" {
		t.Fatalf("merged suppressions = %#v", flags.Suppress)
	}
	if flags.Mixed.MinLines != 900 {
		t.Fatalf("CLI min-lines override lost: %+v", flags.Mixed)
	}
	if flags.Mixed.MinComponentMembers != 3 || flags.Mixed.MinComponentLines != 55 || flags.Mixed.MinSingleComponentComplexity != 7 || flags.Mixed.CohesiveMinLines != 1400 || flags.MixedSeverity != "low" {
		t.Fatalf("config thresholds not applied: options=%+v severity=%q", flags.Mixed, flags.MixedSeverity)
	}
	if len(flags.LayerPolicy) != 1 || flags.LayerPolicy[0].Name != "thin-transport" || flags.LayerPolicy[0].MaxCoordinatedDependencies != 1 || flags.LayerPolicy[0].Severity != "MEDIUM" {
		t.Fatalf("layer policy defaults not resolved: %+v", flags.LayerPolicy)
	}
}
