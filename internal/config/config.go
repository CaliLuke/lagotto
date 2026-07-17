// Package config loads repository-owned Lagotto configuration.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Filename is the repository-root configuration filename Lagotto discovers.
const Filename = ".lagotto.yaml"

// Config is the versioned repository policy loaded from .lagotto.yaml.
type Config struct {
	Version     int                     `yaml:"version"`
	Suppress    []string                `yaml:"suppress"`
	Mixed       MixedConfig             `yaml:"mixed"`
	LayerPolicy []LayerPolicyRuleConfig `yaml:"layer_policy"`
}

// LayerPolicyRuleConfig describes one repository-owned G14 boundary policy.
// Package patterns may be full import paths or module-relative globs.
type LayerPolicyRuleConfig struct {
	Name                       string   `yaml:"name"`
	Paths                      []string `yaml:"paths"`
	Dependencies               []string `yaml:"dependencies"`
	GeneratedTypes             []string `yaml:"generated_types"`
	MaxCoordinatedDependencies *int     `yaml:"max_coordinated_dependencies"`
	Severity                   string   `yaml:"severity"`
}

// MixedConfig contains optional G5 threshold and severity overrides.
type MixedConfig struct {
	MinLines                     *int   `yaml:"min_lines"`
	MinComponentMembers          *int   `yaml:"min_component_members"`
	MinComponentLines            *int   `yaml:"min_component_lines"`
	MinSingleComponentComplexity *int   `yaml:"min_single_component_complexity"`
	Severity                     string `yaml:"severity"`
	CohesiveMinLines             *int   `yaml:"cohesive_min_lines"`
}

// Load reads an explicit config path, or root/.lagotto.yaml when explicit is
// empty. A missing auto-discovered file is not an error. Unknown YAML fields
// are rejected so misspelled policy does not silently stop applying.
func Load(root, explicit string) (Config, string, error) {
	path := explicit
	auto := path == ""
	if auto {
		path = filepath.Join(root, Filename)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		if auto && errors.Is(err, os.ErrNotExist) {
			return Config{}, "", nil
		}
		return Config{}, "", fmt.Errorf("open Lagotto config %q: %w", path, err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(contents))
	decoder.KnownFields(true)
	var cfg Config
	decodeErr := decoder.Decode(&cfg)
	if decodeErr != nil && !errors.Is(decodeErr, io.EOF) {
		return Config{}, "", fmt.Errorf("parse Lagotto config %q: %w", path, decodeErr)
	}
	if decodeErr == nil {
		var extra any
		if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
			if err == nil {
				return Config{}, "", fmt.Errorf("parse Lagotto config %q: multiple YAML documents are not supported", path)
			}
			return Config{}, "", fmt.Errorf("parse Lagotto config %q: %w", path, err)
		}
	}
	if err := Validate(cfg); err != nil {
		return Config{}, "", fmt.Errorf("invalid Lagotto config %q: %w", path, err)
	}
	return cfg, path, nil
}
