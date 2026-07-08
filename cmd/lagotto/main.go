package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/CaliLuke/lagotto/internal/audit"
	"github.com/CaliLuke/lagotto/internal/detect"
	"github.com/CaliLuke/lagotto/internal/pkgload"
	"github.com/CaliLuke/lagotto/internal/version"
)

func main() {
	flags := &detect.Flags{}

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
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			if err := audit.ValidateFormat(flags.Format); err != nil {
				return err
			}
			if err := pkgload.ValidateTags(flags.Tags); err != nil {
				return err
			}
			if flags.FailOn != "" {
				if _, ok := audit.ParseSeverity(flags.FailOn); !ok {
					return fmt.Errorf("unknown --fail-on severity %q (critical|high|medium|low)", flags.FailOn)
				}
			}
			return nil
		},
	}

	rootCmd.PersistentFlags().StringVar(&flags.Tags, "tags", "", "comma-separated build tags (e.g., cgo,typedb)")
	rootCmd.PersistentFlags().StringVar(&flags.Format, "format", "json", "output format: json | text")
	rootCmd.PersistentFlags().StringSliceVar(&flags.Exclude, "exclude", []string{"gen", "vendor", "third_party", "design/generated"}, "path segments to exclude (matches whole segments: \"gen\" skips a/gen/b but not a/agent/b)")
	rootCmd.PersistentFlags().StringVar(&flags.FailOn, "fail-on", "", "exit 2 if any finding is at or above this severity: critical | high | medium | low")

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
