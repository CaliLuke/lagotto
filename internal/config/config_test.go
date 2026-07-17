package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), Filename)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadRepositoryConfig(t *testing.T) {
	path := writeConfig(t, `version: 1
suppress:
  - G5@tqlgen/parser.go
mixed:
  min_lines: 750
  min_component_members: 3
  min_component_lines: 60
  min_single_component_complexity: 7
  severity: low
  cohesive_min_lines: 1400
layer_policy:
  - name: thin-transport
    paths: [internal/transport/**]
    dependencies: [internal/service/**]
    generated_types: [gen/**]
    max_coordinated_dependencies: 2
    severity: low
`)
	cfg, loaded, err := Load(filepath.Dir(path), "")
	if err != nil {
		t.Fatal(err)
	}
	if loaded != path || len(cfg.Suppress) != 1 || cfg.Mixed.MinLines == nil || *cfg.Mixed.MinLines != 750 || cfg.Mixed.MinSingleComponentComplexity == nil || *cfg.Mixed.MinSingleComponentComplexity != 7 || cfg.Mixed.Severity != "low" || cfg.Mixed.CohesiveMinLines == nil || *cfg.Mixed.CohesiveMinLines != 1400 || len(cfg.LayerPolicy) != 1 || cfg.LayerPolicy[0].Name != "thin-transport" {
		t.Fatalf("unexpected config: path=%q config=%+v", loaded, cfg)
	}
}

func TestLoadMissingAutoConfig(t *testing.T) {
	cfg, loaded, err := Load(t.TempDir(), "")
	if err != nil || loaded != "" || len(cfg.Suppress) != 0 {
		t.Fatalf("missing auto config = path %q, config %+v, error %v", loaded, cfg, err)
	}
}

func TestLoadExplicitMissingConfigFails(t *testing.T) {
	_, _, err := Load(t.TempDir(), "missing.yaml")
	if err == nil || !strings.Contains(err.Error(), "open Lagotto config") {
		t.Fatalf("expected explicit missing config error, got %v", err)
	}
}

func TestLoadRejectsUnknownAndInvalidPolicy(t *testing.T) {
	for name, body := range map[string]string{
		"unknown":        "version: 1\nmixt: {}\n",
		"threshold":      "version: 1\nmixed:\n  min_lines: 0\n",
		"cohesive":       "version: 1\nmixed:\n  cohesive_min_lines: -1\n",
		"complexity":     "version: 1\nmixed:\n  min_single_component_complexity: -1\n",
		"suppress":       "version: 1\nsuppress: ['bad selector']\n",
		"layer fields":   "version: 1\nlayer_policy:\n  - name: thin\n    paths: [internal/**]\n",
		"layer max":      "version: 1\nlayer_policy:\n  - name: thin\n    paths: [internal/**]\n    dependencies: [service/**]\n    generated_types: [gen/**]\n    max_coordinated_dependencies: -1\n",
		"layer severity": "version: 1\nlayer_policy:\n  - name: thin\n    paths: [internal/**]\n    dependencies: [service/**]\n    generated_types: [gen/**]\n    severity: urgent\n",
	} {
		t.Run(name, func(t *testing.T) {
			path := writeConfig(t, body)
			if _, _, err := Load(filepath.Dir(path), ""); err == nil {
				t.Fatal("expected invalid config error")
			}
		})
	}
}
