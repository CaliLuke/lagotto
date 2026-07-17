package detect

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/CaliLuke/lagotto/internal/audit"
	"github.com/CaliLuke/lagotto/internal/pkgload"
)

// ScanFS aggregates every filesystem-level smell (G3, G9, G10, G11,
// G12) for each package directory under root. It walks the filesystem
// directly because a loaded package only exposes files selected for
// the current build tags. The pkgs parameter remains for API stability.
func ScanFS(root string, pkgs []*packages.Package, exclude []string) []audit.Finding {
	dirs := collectPackageDirs(root, pkgs, exclude)
	var findings []audit.Finding
	for _, d := range sortedKeys(dirs) {
		contents := dirs[d]
		files := contents.files
		location := filesystemLocation(root, d)
		findings = append(findings, prefixClusterFindings(location, files)...)
		findings = append(findings, shadowSuffixFindings(location, files)...)
		findings = append(findings, junkDrawerFindings(location, files, contents.fileStats)...)
		findings = append(findings, prematurePackageFindings(root, d, location, contents)...)
		findings = append(findings, buildTagPairFindings(location, contents)...)
	}
	return findings
}

// collectPackageDirs walks the filesystem so mutually exclusive
// build-tagged files are visible together. Loaded packages are not a
// reliable directory listing because go/packages filters by build tags.
type packageDirContents struct {
	files          []string
	packageName    string
	buildTagged    map[string]bool
	importsTesting bool
	fileStats      map[string]sourceFileStats
}

type sourceFileStats struct {
	DeclarationCount int `json:"declaration_count"`
	LineCount        int `json:"line_count"`
}

func collectPackageDirs(root string, pkgs []*packages.Package, exclude []string) map[string]packageDirContents {
	_ = pkgs // Package loading selects files by build tags; this detector must not.
	out := map[string]packageDirContents{}
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if pkgload.ShouldExclude(path, exclude) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		file, parseErr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if parseErr != nil || file == nil || ast.IsGenerated(file) {
			return nil
		}
		base := filepath.Base(path)
		if base == "doc.go" {
			return nil
		}
		dir := filepath.Dir(path)
		contents := out[dir]
		contents.files = append(contents.files, base)
		if contents.packageName == "" && file.Name != nil {
			contents.packageName = file.Name.Name
		}
		for _, spec := range file.Imports {
			importPath, unquoteErr := strconv.Unquote(spec.Path.Value)
			if unquoteErr == nil && importPath == "testing" {
				contents.importsTesting = true
			}
		}
		if contents.buildTagged == nil {
			contents.buildTagged = map[string]bool{}
		}
		if contents.fileStats == nil {
			contents.fileStats = map[string]sourceFileStats{}
		}
		contents.fileStats[base] = sourceFileStatistics(fset, file)
		contents.buildTagged[base] = hasBuildConstraint(file)
		out[dir] = contents
		return nil
	})
	for dir, contents := range out {
		sort.Strings(contents.files)
		out[dir] = contents
	}
	return out
}

func sourceFileStatistics(fset *token.FileSet, file *ast.File) sourceFileStats {
	stats := sourceFileStats{LineCount: physicalLineCount(fset, file)}
	for _, declaration := range file.Decls {
		switch decl := declaration.(type) {
		case *ast.FuncDecl:
			stats.DeclarationCount++
		case *ast.GenDecl:
			if decl.Tok == token.IMPORT {
				continue
			}
			for range decl.Specs {
				stats.DeclarationCount++
			}
		}
	}
	return stats
}

func hasBuildConstraint(file *ast.File) bool {
	if file == nil {
		return false
	}
	for _, group := range file.Comments {
		if group.End() > file.Package {
			continue
		}
		for _, comment := range group.List {
			text := strings.TrimSpace(comment.Text)
			if strings.HasPrefix(text, "//go:build ") || strings.HasPrefix(text, "// +build ") {
				return true
			}
		}
	}
	return false
}

// prefixClusterFindings flags G9 — three or more files in dir that
// share a leading name segment up to the first separator (`_`, `-`,
// `.`). The cluster typically wants to be a subpackage.
func prefixClusterFindings(dir string, files []string) []audit.Finding {
	type entry struct{ files []string }
	clusters := map[string]*entry{}
	for _, f := range files {
		base := strings.TrimSuffix(f, ".go")
		idx := strings.IndexAny(base, "_-.")
		if idx < 2 || hasPlatformSuffix(base) {
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
	var findings []audit.Finding
	for _, prefix := range sortedKeys(clusters) {
		c := clusters[prefix]
		if len(c.files) < 3 {
			continue
		}
		findings = append(findings, audit.Finding{
			Smell:    "Prefix Cluster",
			SmellID:  "G9",
			Severity: audit.SevLow,
			Location: dir,
			Message: fmt.Sprintf("%d files share prefix %q in %s.",
				len(c.files), prefix, dir),
			Evidence: map[string]any{
				"prefix": prefix,
				"files":  c.files,
				"dir":    dir,
			},
			Suggestion: "A shared prefix may be healthy organization. Promote the cluster to a subpackage only if it forms an independently changing component with a useful boundary; otherwise keep it or rename files when clearer content names exist.",
		})
	}
	return findings
}

var platformSuffixes = map[string]bool{
	"aix": true, "android": true, "darwin": true, "dragonfly": true,
	"freebsd": true, "illumos": true, "ios": true, "js": true,
	"linux": true, "netbsd": true, "openbsd": true, "plan9": true,
	"solaris": true, "wasip1": true, "windows": true,
	"386": true, "amd64": true, "arm": true, "arm64": true,
	"loong64": true, "mips": true, "mips64": true, "mips64le": true,
	"mipsle": true, "ppc64": true, "ppc64le": true, "riscv64": true,
	"s390x": true, "wasm": true,
}

func hasPlatformSuffix(base string) bool {
	i := strings.LastIndexAny(base, "_-.")
	return i >= 0 && platformSuffixes[base[i+1:]]
}

var shadowSuffixes = []string{
	"_helpers", "_utils", "_handlers", "_actions", "_responses",
	"_data", "_support", "_extra", "_impl", "_misc",
}

// shadowSuffixFindings flags G10 — files named by their relationship
// to siblings (`_helpers.go`, `_handlers.go`, `_utils.go`, …) instead
// of by their content.
func shadowSuffixFindings(dir string, files []string) []audit.Finding {
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
	return []audit.Finding{{
		Smell:    "Shadow Suffix",
		SmellID:  "G10",
		Severity: audit.SevLow,
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
// (`helpers.go`, `utils.go`, `common.go`, etc.) that obscure their content.
// The filename alone does not establish that the file contains mixed concerns.
func junkDrawerFindings(dir string, files []string, statsByFile ...map[string]sourceFileStats) []audit.Finding {
	var hits []string
	for _, f := range files {
		if junkDrawerNames[f] {
			hits = append(hits, f)
		}
	}
	if len(hits) == 0 {
		return nil
	}
	stats := map[string]sourceFileStats{}
	if len(statsByFile) > 0 && statsByFile[0] != nil {
		stats = statsByFile[0]
	}
	type fileEvidence struct {
		File             string `json:"file"`
		DeclarationCount int    `json:"declaration_count"`
		LineCount        int    `json:"line_count"`
		Classification   string `json:"classification"`
	}
	evidenceFiles := make([]fileEvidence, 0, len(hits))
	accumulationRisk := false
	for _, file := range hits {
		fileStats := stats[file]
		classification := "naming_nit"
		if fileStats.DeclarationCount >= 10 && fileStats.LineCount >= 200 {
			classification = "accumulation_risk"
			accumulationRisk = true
		}
		evidenceFiles = append(evidenceFiles, fileEvidence{
			File: file, DeclarationCount: fileStats.DeclarationCount,
			LineCount: fileStats.LineCount, Classification: classification,
		})
	}
	severity := audit.SevLow
	message := fmt.Sprintf("%d generic filename(s) do not describe their content; inspect the declaration and line counts before deciding whether anything beyond a rename is warranted.", len(hits))
	if len(hits) == 1 && len(stats) > 0 {
		fileStats := stats[hits[0]]
		message = fmt.Sprintf("%s is a generic filename with %d top-level declaration(s) over %d lines.", hits[0], fileStats.DeclarationCount, fileStats.LineCount)
	}
	if accumulationRisk {
		severity = audit.SevMedium
		message += " At least one file is large enough to show accumulation risk."
	} else {
		message += " This is a naming signal, not evidence of mixed concerns."
	}
	return []audit.Finding{{
		Smell:    "Generic Filename",
		SmellID:  "G11",
		Severity: severity,
		Location: dir,
		Message:  message,
		Evidence: map[string]any{
			"files": evidenceFiles,
			"dir":   dir,
		},
		Suggestion: "Rename each file after its specific content (for example, error.go for error conversion). Split only if the file actually contains independently changing concerns; a single cohesive helper needs only a rename.",
	}}
}

// prematurePackageFindings flags G12 — directories with exactly one
// non-test, non-generated, non-doc source file. Package main and the
// audit root are exempt.
func prematurePackageFindings(root, dir, location string, contents packageDirContents) []audit.Finding {
	if len(contents.files) != 1 {
		return nil
	}
	if samePath(root, dir) || contents.packageName == "main" || contents.importsTesting {
		return nil
	}
	return []audit.Finding{{
		Smell:    "Premature Package",
		SmellID:  "G12",
		Severity: audit.SevLow,
		Location: location,
		Message:  "Directory contains a single source file; the package is providing visibility, not grouping.",
		Evidence: map[string]any{
			"dir":  location,
			"file": contents.files[0],
		},
		Suggestion: "If this package enforces an intentional visibility boundary, keep it and suppress this finding with --suppress G12@" + location + ". Otherwise add cohesive siblings as the concern grows or inline the file into the parent package.",
	}}
}

func filesystemLocation(root, dir string) string {
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return filepath.ToSlash(dir)
	}
	return filepath.ToSlash(rel)
}

func samePath(a, b string) bool {
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	return errA == nil && errB == nil && filepath.Clean(absA) == filepath.Clean(absB)
}

// buildTagPairFindings flags G3 — three or more `*_stub.go` or
// `*_cgo.go` variants paired with an unsuffixed file. A single pair is a normal Go pattern; once
// the pattern recurs across many files the conditional surface is
// large enough to warrant a sibling subpackage.
func buildTagPairFindings(dir string, contents packageDirContents) []audit.Finding {
	bases := map[string]bool{}
	for _, f := range contents.files {
		base := strings.TrimSuffix(f, ".go")
		bases[base] = true
	}
	var pairList []string
	for _, base := range sortedKeys(bases) {
		for _, suffix := range []string{"_stub", "_cgo"} {
			if !strings.HasSuffix(base, suffix) {
				continue
			}
			partner := strings.TrimSuffix(base, suffix)
			variantFile, partnerFile := base+".go", partner+".go"
			if bases[partner] && (contents.buildTagged[variantFile] || contents.buildTagged[partnerFile]) {
				pairList = append(pairList, partnerFile+" + "+variantFile)
			}
		}
	}
	pairs := len(pairList)
	if pairs < 3 {
		return nil
	}
	sort.Strings(pairList)
	return []audit.Finding{{
		Smell:    "Build-Tag Pair Sprawl",
		SmellID:  "G3",
		Severity: audit.SevMedium,
		Location: dir,
		Message: fmt.Sprintf("Directory has %d build-tag pair(s); the conditional code is large enough to warrant a subpackage.",
			pairs),
		Evidence: map[string]any{
			"pairs": pairs,
			"files": pairList,
			"dir":   dir,
		},
		Suggestion: "Move the stub branch into a sibling subpackage (e.g., cgo/ + nocgo/) and have the parent depend on a shared interface. Each subpackage owns one branch of the build-tag without polluting the directory listing.",
	}}
}
