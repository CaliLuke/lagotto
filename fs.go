package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/tools/go/packages"
)

var fsCmd = &cobra.Command{
	Use:   "fs [path]",
	Short: "Find filesystem-level smells (prefix cluster, shadow suffix, build-tag pair sprawl, premature package, junk drawer).",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		root := "."
		if len(args) == 1 {
			root = args[0]
		}
		pkgs, err := loadPackages(root)
		if err != nil {
			return err
		}
		return emit(&Report{Root: root, Tags: resolvedTags(), Findings: scanFS(root, pkgs)})
	},
}

// scanFS aggregates every filesystem-level smell (G3, G9, G10, G11,
// G12) for each package directory under root. Filesystem detectors
// reason about directory listings, not AST; the loaded packages are
// only used to determine the directory set and exclusion list.
func scanFS(root string, pkgs []*packages.Package) []Finding {
	dirs := collectPackageDirs(root, pkgs)
	var findings []Finding
	for _, d := range sortedKeys(dirs) {
		files := dirs[d]
		findings = append(findings, prefixClusterFindings(d, files)...)
		findings = append(findings, shadowSuffixFindings(d, files)...)
		findings = append(findings, junkDrawerFindings(d, files)...)
		findings = append(findings, prematurePackageFindings(d, files)...)
		findings = append(findings, buildTagPairFindings(d, files)...)
	}
	return findings
}

// collectPackageDirs walks the loaded packages and groups non-test
// .go file basenames by directory, falling back to a filesystem
// walk if packages.Load returned no packages (e.g., no go.mod).
func collectPackageDirs(root string, pkgs []*packages.Package) map[string][]string {
	out := map[string][]string{}
	if len(pkgs) > 0 {
		for _, p := range pkgs {
			if p.PkgPath == "" || shouldExclude(p.PkgPath) {
				continue
			}
			for _, f := range p.GoFiles {
				if strings.HasSuffix(f, "_test.go") {
					continue
				}
				dir := filepath.Dir(f)
				out[dir] = append(out[dir], filepath.Base(f))
			}
		}
		return out
	}
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if shouldExclude(path) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		out[filepath.Dir(path)] = append(out[filepath.Dir(path)], filepath.Base(path))
		return nil
	})
	return out
}

// prefixClusterFindings flags G9 — three or more files in dir that
// share a leading name segment up to the first separator (`_`, `-`,
// `.`). The cluster typically wants to be a subpackage.
func prefixClusterFindings(dir string, files []string) []Finding {
	type entry struct{ files []string }
	clusters := map[string]*entry{}
	for _, f := range files {
		base := strings.TrimSuffix(f, ".go")
		idx := strings.IndexAny(base, "_-.")
		if idx <= 2 {
			continue
		}
		prefix := base[:idx]
		c, ok := clusters[prefix]
		if !ok {
			c = &entry{}
			clusters[prefix] = c
		}
		c.files = append(c.files, f)
	}
	var findings []Finding
	for _, prefix := range sortedKeys(clusters) {
		c := clusters[prefix]
		if len(c.files) < 3 {
			continue
		}
		sev := SevLow
		if len(c.files) >= 4 {
			sev = SevMedium
		}
		findings = append(findings, Finding{
			Smell:    "Prefix Cluster",
			SmellID:  "G9",
			Severity: sev,
			Location: dir,
			Message: fmt.Sprintf("%d files share prefix %q in %s.",
				len(c.files), prefix, dir),
			Evidence: map[string]any{
				"prefix": prefix,
				"files":  c.files,
				"dir":    dir,
			},
			Suggestion: "Promote the cluster to a subpackage if the files share a domain concern. Otherwise rename to remove the artificial common prefix.",
		})
	}
	return findings
}

var shadowSuffixes = []string{
	"_helpers", "_utils", "_handlers", "_actions", "_responses",
	"_data", "_support", "_extra", "_impl", "_misc",
}

// shadowSuffixFindings flags G10 — files named by their relationship
// to siblings (`_helpers.go`, `_handlers.go`, `_utils.go`, …) instead
// of by their content.
func shadowSuffixFindings(dir string, files []string) []Finding {
	var hits []string
	for _, f := range files {
		base := strings.TrimSuffix(f, ".go")
		for _, sfx := range shadowSuffixes {
			if strings.HasSuffix(base, sfx) {
				hits = append(hits, f)
				break
			}
		}
	}
	if len(hits) == 0 {
		return nil
	}
	return []Finding{{
		Smell:    "Shadow Suffix",
		SmellID:  "G10",
		Severity: SevLow,
		Location: dir,
		Message: fmt.Sprintf("%d file(s) end in a relationship-naming suffix (_helpers/_handlers/_actions/etc.) rather than a content-naming suffix.",
			len(hits)),
		Evidence: map[string]any{
			"files": hits,
			"dir":   dir,
		},
		Suggestion: "Rename to describe content (what it is) rather than relationship (where it lives). The suffix becomes obsolete once the file's content has a real name.",
	}}
}

var junkDrawerNames = map[string]bool{
	"helpers.go": true,
	"utils.go":   true,
	"common.go":  true,
	"misc.go":    true,
	"shared.go":  true,
	"lib.go":     true,
}

// junkDrawerFindings flags G11 — files with reserved generic names
// (`helpers.go`, `utils.go`, `common.go`, etc.) that signal the file
// is a catch-all rather than a coherent unit.
func junkDrawerFindings(dir string, files []string) []Finding {
	var hits []string
	for _, f := range files {
		if junkDrawerNames[f] {
			hits = append(hits, f)
		}
	}
	if len(hits) == 0 {
		return nil
	}
	return []Finding{{
		Smell:    "Junk Drawer",
		SmellID:  "G11",
		Severity: SevLow,
		Location: dir,
		Message:  "Directory contains a generically-named catch-all file.",
		Evidence: map[string]any{
			"files": hits,
			"dir":   dir,
		},
		Suggestion: "Read the contents and split by actual concern. A file named after where it sits ('helpers') instead of what it contains is a structural smell.",
	}}
}

// prematurePackageFindings flags G12 — directories with exactly one
// non-test source file. The package boundary is providing visibility,
// not grouping; once it grows naturally the smell self-resolves.
// `doc.go`-only directories are exempt.
func prematurePackageFindings(dir string, files []string) []Finding {
	if len(files) != 1 {
		return nil
	}
	// Skip the audit root itself; a single file at the root is a tool.
	if filepath.Clean(dir) == "." {
		return nil
	}
	// Skip leaf packages whose single file is doc.go.
	if files[0] == "doc.go" {
		return nil
	}
	return []Finding{{
		Smell:    "Premature Package",
		SmellID:  "G12",
		Severity: SevLow,
		Location: dir,
		Message:  "Directory contains a single source file; the package is providing visibility, not grouping.",
		Evidence: map[string]any{
			"dir":  dir,
			"file": files[0],
		},
		Suggestion: "Either add more files when the concern grows, or inline the file into the parent package if the visibility boundary isn't load-bearing.",
	}}
}

// buildTagPairFindings flags G3 — three or more `*_stub.go` /
// `*.go` paired files. A single pair is a normal Go pattern; once
// the pattern recurs across many files the conditional surface is
// large enough to warrant a sibling subpackage.
func buildTagPairFindings(dir string, files []string) []Finding {
	pairs := 0
	bases := map[string]bool{}
	for _, f := range files {
		base := strings.TrimSuffix(f, ".go")
		bases[base] = true
	}
	for base := range bases {
		if !strings.HasSuffix(base, "_stub") {
			continue
		}
		partner := strings.TrimSuffix(base, "_stub")
		if bases[partner] {
			pairs++
		}
	}
	if pairs < 3 {
		return nil
	}
	var pairList []string
	for base := range bases {
		if strings.HasSuffix(base, "_stub") {
			pairList = append(pairList, base+".go")
		}
	}
	sort.Strings(pairList)
	return []Finding{{
		Smell:    "Build-Tag Pair Sprawl",
		SmellID:  "G3",
		Severity: SevMedium,
		Location: dir,
		Message: fmt.Sprintf("Directory has %d build-tag pair(s); the conditional code is large enough to warrant a subpackage.",
			pairs),
		Evidence: map[string]any{
			"pairs": pairs,
			"stubs": pairList,
			"dir":   dir,
		},
		Suggestion: "Move the stub branch into a sibling subpackage (e.g., cgo/ + nocgo/) and have the parent depend on a shared interface. Each subpackage owns one branch of the build-tag without polluting the directory listing.",
	}}
}
