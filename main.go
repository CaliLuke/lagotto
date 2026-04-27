// Lagotto sniffs out Go layout smells (Receiver Monolith, stutter,
// facade methods, god dependency bags, mixed-concern files, FS smells)
// using go/packages-based AST analysis.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	flagTags    string
	flagFormat  string
	flagExclude []string
)

var rootCmd = &cobra.Command{
	Use:   "lagotto",
	Short: "Sniff out Go layout smells (named for a truffle dog).",
	Long: `Lagotto audits Go file/package layout for structural problems
that the language's specific rules — methods bound to receiver-defining
packages, package=directory, build tags, internal/ visibility — produce.

It uses go/packages and go/ast for accurate analysis (no regex
heuristics). Designed to be invoked by an audit skill or a human
reviewer, with JSON output for machine consumption.`,
	Version: versionString(),
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagTags, "tags", "", "comma-separated build tags (e.g., cgo,typedb)")
	rootCmd.PersistentFlags().StringVar(&flagFormat, "format", "json", "output format: json | text")
	rootCmd.PersistentFlags().StringSliceVar(&flagExclude, "exclude", []string{"gen", "vendor", "third_party", "design/generated"}, "path substrings to exclude")

	rootCmd.AddCommand(auditCmd)
	rootCmd.AddCommand(monolithsCmd)
	rootCmd.AddCommand(stutterCmd)
	rootCmd.AddCommand(facadesCmd)
	rootCmd.AddCommand(depsCmd)
	rootCmd.AddCommand(mixedCmd)
	rootCmd.AddCommand(fsCmd)
	rootCmd.AddCommand(initsCmd)
	rootCmd.AddCommand(tunnelCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "lagotto:", err)
		os.Exit(1)
	}
}
