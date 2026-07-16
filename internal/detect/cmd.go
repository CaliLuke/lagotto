package detect

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/tools/go/packages"

	"github.com/CaliLuke/lagotto/internal/audit"
	"github.com/CaliLuke/lagotto/internal/pkgload"
)

// Flags carries the persistent CLI flag values shared by every
// subcommand. cmd/lagotto/main.go binds these to cobra persistent
// flags; the detect package reads them at RunE time.
type Flags struct {
	Tags     string
	Format   string
	Exclude  []string
	Suppress []string
	FailOn   string
}

// argRoot resolves the optional path argument to the directory the
// loader should run in. The idiomatic Go tool pattern `./...` (and
// any `dir/...` suffix) is accepted and reduced to the directory —
// recursion is always implicit.
func argRoot(args []string) string {
	if len(args) != 1 {
		return "."
	}
	root := args[0]
	if strings.HasSuffix(root, "...") {
		root = strings.TrimSuffix(strings.TrimSuffix(root, "..."), "/")
		if root == "" {
			root = "."
		}
	}
	return root
}

// runScan is the shared body of every subcommand: load packages,
// surface load problems on stderr and in the report envelope, run the
// detector(s), emit. Detectors still report on whatever type-checked,
// but an incomplete load returns an error after emission so CI cannot
// mistake a partial audit for a clean run.
func runScan(f *Flags, args []string, scan func(root string, pkgs []*packages.Package) []audit.Finding) error {
	root := argRoot(args)
	if err := prepareScanRoot(root); err != nil {
		return err
	}
	pkgs, loadErrs, err := pkgload.Load(root, f.Tags, f.Exclude)
	if err != nil {
		return err
	}
	if len(pkgs) == 0 {
		fmt.Fprintf(os.Stderr, "lagotto: warning: no Go packages found under %q\n", root)
	}
	return emitScanReport(f, root, loadErrs, scan(root, pkgs))
}

func prepareScanRoot(root string) error {
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("path %q does not exist", root)
		}
		return fmt.Errorf("cannot access %q: %w", root, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%q is not a directory (lagotto takes a directory; subdirectories are always included)", root)
	}
	if err := pkgload.CheckToolchain(root); err != nil {
		return err
	}
	return nil
}

func emitScanReport(f *Flags, root string, loadErrs []string, rawFindings []audit.Finding) error {
	for _, le := range loadErrs {
		fmt.Fprintln(os.Stderr, "lagotto: load error:", le)
	}
	if len(loadErrs) > 0 {
		fmt.Fprintf(os.Stderr, "lagotto: warning: %d package load error(s); findings may be incomplete\n", len(loadErrs))
	}
	findings, suppressed, err := audit.ApplySuppressions(rawFindings, f.Suppress)
	if err != nil {
		return err
	}
	report := &audit.Report{
		Root:               root,
		Tags:               audit.ResolvedTags(f.Tags),
		LoadErrors:         loadErrs,
		SuppressedFindings: suppressed,
		Findings:           findings,
	}
	if err := audit.Emit(report, f.Format); err != nil {
		return err
	}
	return reportOutcome(report, f.FailOn)
}

const auditPackageBatchSize = 24

// runAuditScan avoids retaining module-wide syntax and TypesInfo maps.
// Receiver detectors run from a lightweight type-only load, then the
// syntax-dependent detectors process bounded package batches.
func runAuditScan(f *Flags, args []string) error {
	root := argRoot(args)
	if err := prepareScanRoot(root); err != nil {
		return err
	}
	typedPkgs, loadErrs, err := pkgload.LoadTypes(root, f.Tags, f.Exclude)
	if err != nil {
		return err
	}
	if len(typedPkgs) == 0 {
		fmt.Fprintf(os.Stderr, "lagotto: warning: no Go packages found under %q\n", root)
	}
	findings := ScanReceivers(typedPkgs)
	paths := make([]string, 0, len(typedPkgs))
	for _, pkg := range typedPkgs {
		paths = append(paths, pkg.PkgPath)
	}
	runtime.GC()

	for start := 0; start < len(paths); start += auditPackageBatchSize {
		end := min(start+auditPackageBatchSize, len(paths))
		pkgs, batchErrs, err := pkgload.LoadPatterns(root, f.Tags, f.Exclude, paths[start:end])
		if err != nil {
			return err
		}
		loadErrs = appendUnique(loadErrs, batchErrs...)
		findings = append(findings, ScanStutter(pkgs)...)
		findings = append(findings, ScanFacades(pkgs)...)
		findings = append(findings, ScanDepsBag(pkgs)...)
		findings = append(findings, ScanMixedConcern(pkgs)...)
		findings = append(findings, ScanInitCoupling(pkgs)...)
		findings = append(findings, ScanReExportTunnel(pkgs)...)
	}
	findings = append(findings, ScanFS(root, nil, f.Exclude)...)
	return emitScanReport(f, root, loadErrs, findings)
}

func appendUnique(existing []string, values ...string) []string {
	seen := make(map[string]bool, len(existing)+len(values))
	for _, value := range existing {
		seen[value] = true
	}
	for _, value := range values {
		if !seen[value] {
			existing = append(existing, value)
			seen[value] = true
		}
	}
	return existing
}

// reportOutcome applies exit-code precedence after a report has been
// emitted. Incomplete loading is a run failure (exit 1) even when the
// partial findings also meet --fail-on (exit 2).
func reportOutcome(report *audit.Report, failOn string) error {
	if len(report.LoadErrors) > 0 {
		return &audit.IncompleteLoadError{Count: len(report.LoadErrors)}
	}
	if failOn != "" {
		threshold, ok := audit.ParseSeverity(failOn)
		if !ok {
			return fmt.Errorf("unknown --fail-on severity %q (critical|high|medium|low)", failOn)
		}
		count := 0
		for _, fd := range report.Findings {
			if fd.Severity.AtLeast(threshold) {
				count++
			}
		}
		if count > 0 {
			return &audit.FindingsError{Count: count, Threshold: threshold}
		}
	}
	return nil
}

// AuditCmd returns the `audit` subcommand: run every detector and
// emit the aggregated report.
func AuditCmd(f *Flags) *cobra.Command {
	return &cobra.Command{
		Use:   "audit [path]",
		Short: "Run all smell detectors and emit findings.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runAuditScan(f, args)
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
		Short: "Find Internal Re-Export Tunnels (packages dominated by re-exports from a deeper package).",
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
