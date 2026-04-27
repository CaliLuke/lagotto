package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/tools/go/packages"
)

var mixedCmd = &cobra.Command{
	Use:   "mixed [path]",
	Short: "Find Mixed-Concern Files (3+ unrelated decl groups).",
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
		return emit(&Report{Root: root, Tags: resolvedTags(), Findings: scanMixedConcern(pkgs)})
	},
}

func scanMixedConcern(pkgs []*packages.Package) []Finding {
	var findings []Finding
	for _, pkg := range pkgs {
		if pkg.PkgPath == "" || shouldExclude(pkg.PkgPath) {
			continue
		}
		for i, file := range pkg.Syntax {
			fname := pkg.GoFiles[i]
			if strings.HasSuffix(fname, "_test.go") {
				continue
			}
			groups, lineCount := classifyFile(pkg.Fset, file)
			if len(groups) < 3 {
				continue
			}
			if lineCount < 100 {
				continue
			}
			sev := SevMedium
			if lineCount >= 300 {
				sev = SevHigh
			}
			if lineCount >= 600 {
				sev = SevCritical
			}
			findings = append(findings, Finding{
				Smell:    "Mixed-Concern File",
				SmellID:  "G5",
				Severity: sev,
				Location: filepath.Base(fname),
				Message: fmt.Sprintf("File %s mixes %d decl groups (%s) over %d lines.",
					filepath.Base(fname), len(groups), strings.Join(groupNames(groups), ", "), lineCount),
				Evidence: map[string]any{
					"file":       fname,
					"package":    pkg.PkgPath,
					"groups":     groupNames(groups),
					"line_count": lineCount,
				},
				Suggestion: "Split the file by concern: types in one file, methods in another, validation/utilities in their own files. Avoid mixing pure types with arbitrary helpers in the same file.",
			})
		}
	}
	return findings
}
