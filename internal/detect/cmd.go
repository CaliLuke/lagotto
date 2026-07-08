package detect

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/tools/go/packages"

	"github.com/CaliLuke/lagotto/internal/audit"
	"github.com/CaliLuke/lagotto/internal/pkgload"
)

// Flags carries the persistent CLI flag values shared by every
// subcommand. cmd/lagotto/main.go binds these to cobra persistent
// flags; the detect package reads them at RunE time.
type Flags struct {
	Tags    string
	Format  string
	Exclude []string
}

func argRoot(args []string) string {
	if len(args) == 1 {
		return args[0]
	}
	return "."
}

// runScan is the shared body of every subcommand: load packages,
// surface load problems on stderr and in the report envelope, run the
// detector(s), emit. Load errors do not abort the run — detectors
// still report on whatever type-checked — but they are never silent:
// a broken package must be distinguishable from a clean one.
func runScan(f *Flags, args []string, scan func(root string, pkgs []*packages.Package) []audit.Finding) error {
	root := argRoot(args)
	pkgs, loadErrs, err := pkgload.Load(root, f.Tags, f.Exclude)
	if err != nil {
		return err
	}
	if len(pkgs) == 0 {
		fmt.Fprintf(os.Stderr, "lagotto: warning: no Go packages found under %q\n", root)
	}
	for _, le := range loadErrs {
		fmt.Fprintln(os.Stderr, "lagotto: load error:", le)
	}
	if len(loadErrs) > 0 {
		fmt.Fprintf(os.Stderr, "lagotto: warning: %d package load error(s); findings may be incomplete\n", len(loadErrs))
	}
	return audit.Emit(&audit.Report{
		Root:       root,
		Tags:       audit.ResolvedTags(f.Tags),
		LoadErrors: loadErrs,
		Findings:   scan(root, pkgs),
	}, f.Format)
}

// AuditCmd returns the `audit` subcommand: run every detector and
// emit the aggregated report.
func AuditCmd(f *Flags) *cobra.Command {
	return &cobra.Command{
		Use:   "audit [path]",
		Short: "Run all smell detectors and emit findings.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runScan(f, args, func(root string, pkgs []*packages.Package) []audit.Finding {
				var findings []audit.Finding
				findings = append(findings, ScanReceivers(pkgs)...)
				findings = append(findings, ScanStutter(pkgs)...)
				findings = append(findings, ScanFacades(pkgs)...)
				findings = append(findings, ScanDepsBag(pkgs)...)
				findings = append(findings, ScanMixedConcern(pkgs)...)
				findings = append(findings, ScanInitCoupling(pkgs)...)
				findings = append(findings, ScanReExportTunnel(pkgs)...)
				findings = append(findings, ScanFS(root, pkgs, f.Exclude)...)
				return findings
			})
		},
	}
}

// MonolithsCmd returns the `monoliths` subcommand: G1/G1B/G1C/G1D.
func MonolithsCmd(f *Flags) *cobra.Command {
	return &cobra.Command{
		Use:   "monoliths [path]",
		Short: "Find Receiver Monoliths and Decomposition Theatre.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runScan(f, args, func(_ string, pkgs []*packages.Package) []audit.Finding {
				return ScanReceivers(pkgs)
			})
		},
	}
}

// StutterCmd returns the `stutter` subcommand: G2.
func StutterCmd(f *Flags) *cobra.Command {
	return &cobra.Command{
		Use:   "stutter [path]",
		Short: "Find exported names that stutter on the package name.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runScan(f, args, func(_ string, pkgs []*packages.Package) []audit.Finding {
				return ScanStutter(pkgs)
			})
		},
	}
}

// FacadesCmd returns the `facades` subcommand: G6.
func FacadesCmd(f *Flags) *cobra.Command {
	return &cobra.Command{
		Use:   "facades [path]",
		Short: "Find Facade Methods (thin pass-throughs to subpackage funcs).",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runScan(f, args, func(_ string, pkgs []*packages.Package) []audit.Finding {
				return ScanFacades(pkgs)
			})
		},
	}
}

// DepsCmd returns the `deps` subcommand: G4.
func DepsCmd(f *Flags) *cobra.Command {
	return &cobra.Command{
		Use:   "deps [path]",
		Short: "Find God Dependency Bag structs.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runScan(f, args, func(_ string, pkgs []*packages.Package) []audit.Finding {
				return ScanDepsBag(pkgs)
			})
		},
	}
}

// MixedCmd returns the `mixed` subcommand: G5.
func MixedCmd(f *Flags) *cobra.Command {
	return &cobra.Command{
		Use:   "mixed [path]",
		Short: "Find Mixed-Concern Files (3+ unrelated decl groups).",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runScan(f, args, func(_ string, pkgs []*packages.Package) []audit.Finding {
				return ScanMixedConcern(pkgs)
			})
		},
	}
}

// InitsCmd returns the `inits` subcommand: G7.
func InitsCmd(f *Flags) *cobra.Command {
	return &cobra.Command{
		Use:   "inits [path]",
		Short: "Find Init Coupling (multiple init() funcs across files).",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runScan(f, args, func(_ string, pkgs []*packages.Package) []audit.Finding {
				return ScanInitCoupling(pkgs)
			})
		},
	}
}

// TunnelCmd returns the `tunnel` subcommand: G8.
func TunnelCmd(f *Flags) *cobra.Command {
	return &cobra.Command{
		Use:   "tunnel [path]",
		Short: "Find Internal Re-Export Tunnels (packages whose only role is re-exporting from a deeper package).",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runScan(f, args, func(_ string, pkgs []*packages.Package) []audit.Finding {
				return ScanReExportTunnel(pkgs)
			})
		},
	}
}

// FSCmd returns the `fs` subcommand: G3, G9, G10, G11, G12.
func FSCmd(f *Flags) *cobra.Command {
	return &cobra.Command{
		Use:   "fs [path]",
		Short: "Find filesystem-level smells (prefix cluster, shadow suffix, build-tag pair sprawl, premature package, junk drawer).",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runScan(f, args, func(root string, pkgs []*packages.Package) []audit.Finding {
				return ScanFS(root, pkgs, f.Exclude)
			})
		},
	}
}
