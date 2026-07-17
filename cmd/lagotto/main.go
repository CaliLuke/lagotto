package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/CaliLuke/lagotto/internal/audit"
	"github.com/CaliLuke/lagotto/internal/config"
	"github.com/CaliLuke/lagotto/internal/detect"
	"github.com/CaliLuke/lagotto/internal/pkgload"
	"github.com/CaliLuke/lagotto/internal/version"
)

func main() {
	flags := &detect.Flags{
		Mixed:         detect.DefaultMixedOptions(),
		MixedSeverity: "medium",
	}

	rootCmd := &cobra.Command{
		Use:   "lagotto",
		Short: "Sniff out Go layout smells (named for a truffle dog).",
		Long: `Lagotto audits Go file/package layout for structural problems
that the language's specific rules — methods bound to receiver-defining
packages, package=directory, build tags, internal/ visibility — produce.

It uses go/packages and go/ast for accurate analysis (no regex
heuristics). Designed to be invoked by an audit skill or a human
reviewer, with JSON output for machine consumption.

Exit codes: 0 clean run, 1 run failed, 2 findings met --fail-on.`,
		Version: version.String(),
		// main prints the single "lagotto: <err>" line; without these
		// cobra would print the error a second time plus a full usage
		// dump for runtime failures that have nothing to do with usage.
		SilenceErrors: true,
		SilenceUsage:  true,
		// Validate flag values before any subcommand loads packages, so
		// a typo'd --format fails in milliseconds, not after a full
		// typecheck of the target module.
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := applyRepositoryConfig(cmd, args, flags); err != nil {
				return err
			}
			if err := audit.ValidateFormat(flags.Format); err != nil {
				return err
			}
			if err := pkgload.ValidateTags(flags.Tags); err != nil {
				return err
			}
			if err := audit.ValidateSuppressions(flags.Suppress); err != nil {
				return err
			}
			if flags.FailOn != "" {
				if _, ok := audit.ParseSeverity(flags.FailOn); !ok {
					return fmt.Errorf("unknown --fail-on severity %q (critical|high|medium|low)", flags.FailOn)
				}
			}
			if err := detect.ValidateMixedOptions(flags.Mixed); err != nil {
				return err
			}
			if _, ok := audit.ParseSeverity(flags.MixedSeverity); !ok {
				return fmt.Errorf("unknown mixed severity %q (critical|high|medium|low)", flags.MixedSeverity)
			}
			if err := detect.ValidateLayerPolicyRules(flags.LayerPolicy); err != nil {
				return err
			}
			return nil
		},
	}

	rootCmd.PersistentFlags().StringVar(&flags.Tags, "tags", "", "comma-separated build tags (e.g., cgo,typedb)")
	rootCmd.PersistentFlags().StringVar(&flags.Format, "format", "json", "output format: json | text")
	rootCmd.PersistentFlags().StringSliceVar(&flags.Exclude, "exclude", []string{"gen", "vendor", "third_party", "design/generated"}, "path segments to exclude (matches whole segments: \"gen\" skips a/gen/b but not a/agent/b)")
	rootCmd.PersistentFlags().StringSliceVar(&flags.Suppress, "suppress", nil, "suppress findings by SMELL_ID or SMELL_ID@LOCATION prefix (repeatable)")
	rootCmd.PersistentFlags().StringVar(&flags.FailOn, "fail-on", "", "exit 2 if any finding is at or above this severity: critical | high | medium | low")
	rootCmd.PersistentFlags().StringVar(&flags.ConfigPath, "config", "", "config file (default: <audit-root>/.lagotto.yaml)")

	rootCmd.AddCommand(
		detect.AuditCmd(flags),
		detect.MonolithsCmd(flags),
		detect.StutterCmd(flags),
		detect.FacadesCmd(flags),
		detect.DepsCmd(flags),
		detect.MixedCmd(flags),
		detect.FSCmd(flags),
		detect.InitsCmd(flags),
		detect.TunnelCmd(flags),
		detect.LayersCmd(flags),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "lagotto:", err)
		var fe *audit.FindingsError
		if errors.As(err, &fe) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

func applyRepositoryConfig(cmd *cobra.Command, args []string, flags *detect.Flags) error {
	cfg, path, err := config.Load(detect.ScanRoot(args), flags.ConfigPath)
	if err != nil {
		return err
	}
	flags.LoadedConfigPath = path
	cliSuppressions := append([]string(nil), flags.Suppress...)
	flags.Suppress = append(append([]string(nil), cfg.Suppress...), cliSuppressions...)

	if cfg.Mixed.MinLines != nil && !flagChanged(cmd, "min-lines") {
		flags.Mixed.MinLines = *cfg.Mixed.MinLines
	}
	if cfg.Mixed.MinComponentMembers != nil && !flagChanged(cmd, "min-component-members") {
		flags.Mixed.MinComponentMembers = *cfg.Mixed.MinComponentMembers
	}
	if cfg.Mixed.MinComponentLines != nil && !flagChanged(cmd, "min-component-lines") {
		flags.Mixed.MinComponentLines = *cfg.Mixed.MinComponentLines
	}
	if cfg.Mixed.MinSingleComponentComplexity != nil && !flagChanged(cmd, "min-single-component-complexity") {
		flags.Mixed.MinSingleComponentComplexity = *cfg.Mixed.MinSingleComponentComplexity
	}
	if cfg.Mixed.CohesiveMinLines != nil && !flagChanged(cmd, "cohesive-min-lines") {
		flags.Mixed.CohesiveMinLines = *cfg.Mixed.CohesiveMinLines
	}
	if cfg.Mixed.Severity != "" && !flagChanged(cmd, "severity") {
		flags.MixedSeverity = cfg.Mixed.Severity
	}
	if severity, ok := audit.ParseSeverity(flags.MixedSeverity); ok {
		flags.Mixed.Severity = severity
	}
	flags.LayerPolicy = make([]detect.LayerPolicyRule, 0, len(cfg.LayerPolicy))
	for _, configured := range cfg.LayerPolicy {
		maxDependencies := 1
		if configured.MaxCoordinatedDependencies != nil {
			maxDependencies = *configured.MaxCoordinatedDependencies
		}
		severity := audit.SevMedium
		if configuredSeverity, ok := audit.ParseSeverity(configured.Severity); ok {
			severity = configuredSeverity
		}
		flags.LayerPolicy = append(flags.LayerPolicy, detect.LayerPolicyRule{
			Name:                       configured.Name,
			Paths:                      append([]string(nil), configured.Paths...),
			Dependencies:               append([]string(nil), configured.Dependencies...),
			GeneratedTypes:             append([]string(nil), configured.GeneratedTypes...),
			MaxCoordinatedDependencies: maxDependencies,
			Severity:                   severity,
		})
	}
	return nil
}

func flagChanged(cmd *cobra.Command, name string) bool {
	flag := cmd.Flags().Lookup(name)
	return flag != nil && flag.Changed
}
