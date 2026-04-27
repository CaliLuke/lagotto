package detect

import (
	"github.com/spf13/cobra"

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

// AuditCmd returns the `audit` subcommand: run every detector and
// emit the aggregated report.
func AuditCmd(f *Flags) *cobra.Command {
	return &cobra.Command{
		Use:   "audit [path]",
		Short: "Run all smell detectors and emit findings.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			root := argRoot(args)
			pkgs, err := pkgload.Load(root, f.Tags, f.Exclude)
			if err != nil {
				return err
			}
			report := &audit.Report{Root: root, Tags: audit.ResolvedTags(f.Tags)}
			report.Findings = append(report.Findings, ScanReceivers(pkgs)...)
			report.Findings = append(report.Findings, ScanStutter(pkgs)...)
			report.Findings = append(report.Findings, ScanFacades(pkgs)...)
			report.Findings = append(report.Findings, ScanDepsBag(pkgs)...)
			report.Findings = append(report.Findings, ScanMixedConcern(pkgs)...)
			report.Findings = append(report.Findings, ScanInitCoupling(pkgs)...)
			report.Findings = append(report.Findings, ScanReExportTunnel(pkgs)...)
			report.Findings = append(report.Findings, ScanFS(root, pkgs, f.Exclude)...)
			return audit.Emit(report, f.Format)
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
			root := argRoot(args)
			pkgs, err := pkgload.Load(root, f.Tags, f.Exclude)
			if err != nil {
				return err
			}
			return audit.Emit(&audit.Report{
				Root:     root,
				Tags:     audit.ResolvedTags(f.Tags),
				Findings: ScanReceivers(pkgs),
			}, f.Format)
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
			root := argRoot(args)
			pkgs, err := pkgload.Load(root, f.Tags, f.Exclude)
			if err != nil {
				return err
			}
			return audit.Emit(&audit.Report{
				Root:     root,
				Tags:     audit.ResolvedTags(f.Tags),
				Findings: ScanStutter(pkgs),
			}, f.Format)
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
			root := argRoot(args)
			pkgs, err := pkgload.Load(root, f.Tags, f.Exclude)
			if err != nil {
				return err
			}
			return audit.Emit(&audit.Report{
				Root:     root,
				Tags:     audit.ResolvedTags(f.Tags),
				Findings: ScanFacades(pkgs),
			}, f.Format)
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
			root := argRoot(args)
			pkgs, err := pkgload.Load(root, f.Tags, f.Exclude)
			if err != nil {
				return err
			}
			return audit.Emit(&audit.Report{
				Root:     root,
				Tags:     audit.ResolvedTags(f.Tags),
				Findings: ScanDepsBag(pkgs),
			}, f.Format)
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
			root := argRoot(args)
			pkgs, err := pkgload.Load(root, f.Tags, f.Exclude)
			if err != nil {
				return err
			}
			return audit.Emit(&audit.Report{
				Root:     root,
				Tags:     audit.ResolvedTags(f.Tags),
				Findings: ScanMixedConcern(pkgs),
			}, f.Format)
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
			root := argRoot(args)
			pkgs, err := pkgload.Load(root, f.Tags, f.Exclude)
			if err != nil {
				return err
			}
			return audit.Emit(&audit.Report{
				Root:     root,
				Tags:     audit.ResolvedTags(f.Tags),
				Findings: ScanInitCoupling(pkgs),
			}, f.Format)
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
			root := argRoot(args)
			pkgs, err := pkgload.Load(root, f.Tags, f.Exclude)
			if err != nil {
				return err
			}
			return audit.Emit(&audit.Report{
				Root:     root,
				Tags:     audit.ResolvedTags(f.Tags),
				Findings: ScanReExportTunnel(pkgs),
			}, f.Format)
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
			root := argRoot(args)
			pkgs, err := pkgload.Load(root, f.Tags, f.Exclude)
			if err != nil {
				return err
			}
			return audit.Emit(&audit.Report{
				Root:     root,
				Tags:     audit.ResolvedTags(f.Tags),
				Findings: ScanFS(root, pkgs, f.Exclude),
			}, f.Format)
		},
	}
}
