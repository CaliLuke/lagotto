package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/CaliLuke/lagotto/internal/detect"
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
reviewer, with JSON output for machine consumption.`,
		Version: version.String(),
	}

	rootCmd.PersistentFlags().StringVar(&flags.Tags, "tags", "", "comma-separated build tags (e.g., cgo,typedb)")
	rootCmd.PersistentFlags().StringVar(&flags.Format, "format", "json", "output format: json | text")
	rootCmd.PersistentFlags().StringSliceVar(&flags.Exclude, "exclude", []string{"gen", "vendor", "third_party", "design/generated"}, "path substrings to exclude")

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
		os.Exit(1)
	}
}
