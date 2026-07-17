package config

import (
	"fmt"

	"github.com/CaliLuke/lagotto/internal/audit"
)

// Validate rejects unsupported versions, invalid suppressions, thresholds,
// and severity names before package loading begins.
func Validate(cfg Config) error {
	if cfg.Version != 1 {
		return fmt.Errorf("unsupported version %d (supported: 1)", cfg.Version)
	}
	if err := audit.ValidateSuppressions(cfg.Suppress); err != nil {
		return err
	}
	if cfg.Mixed.MinLines != nil && *cfg.Mixed.MinLines < 1 {
		return fmt.Errorf("mixed.min_lines must be at least 1")
	}
	if cfg.Mixed.MinComponentMembers != nil && *cfg.Mixed.MinComponentMembers < 1 {
		return fmt.Errorf("mixed.min_component_members must be at least 1")
	}
	if cfg.Mixed.MinComponentLines != nil && *cfg.Mixed.MinComponentLines < 1 {
		return fmt.Errorf("mixed.min_component_lines must be at least 1")
	}
	if cfg.Mixed.MinSingleComponentComplexity != nil && *cfg.Mixed.MinSingleComponentComplexity < 0 {
		return fmt.Errorf("mixed.min_single_component_complexity cannot be negative")
	}
	if cfg.Mixed.CohesiveMinLines != nil && *cfg.Mixed.CohesiveMinLines < 0 {
		return fmt.Errorf("mixed.cohesive_min_lines cannot be negative")
	}
	if cfg.Mixed.Severity != "" {
		if _, ok := audit.ParseSeverity(cfg.Mixed.Severity); !ok {
			return fmt.Errorf("unknown mixed.severity %q (critical|high|medium|low)", cfg.Mixed.Severity)
		}
	}
	seenRules := map[string]bool{}
	for index, rule := range cfg.LayerPolicy {
		prefix := fmt.Sprintf("layer_policy[%d]", index)
		if rule.Name == "" {
			return fmt.Errorf("%s.name is required", prefix)
		}
		if seenRules[rule.Name] {
			return fmt.Errorf("duplicate layer_policy name %q", rule.Name)
		}
		seenRules[rule.Name] = true
		if len(rule.Paths) == 0 || len(rule.Dependencies) == 0 || len(rule.GeneratedTypes) == 0 {
			return fmt.Errorf("%s must define paths, dependencies, and generated_types", prefix)
		}
		for field, patterns := range map[string][]string{
			"paths": rule.Paths, "dependencies": rule.Dependencies, "generated_types": rule.GeneratedTypes,
		} {
			for _, pattern := range patterns {
				if pattern == "" {
					return fmt.Errorf("%s.%s cannot contain an empty pattern", prefix, field)
				}
			}
		}
		if rule.MaxCoordinatedDependencies != nil && *rule.MaxCoordinatedDependencies < 0 {
			return fmt.Errorf("%s.max_coordinated_dependencies cannot be negative", prefix)
		}
		if rule.Severity != "" {
			if _, ok := audit.ParseSeverity(rule.Severity); !ok {
				return fmt.Errorf("unknown %s.severity %q (critical|high|medium|low)", prefix, rule.Severity)
			}
		}
	}
	return nil
}
