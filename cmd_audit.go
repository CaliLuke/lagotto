package main

import (
	"github.com/spf13/cobra"
)

var auditCmd = &cobra.Command{
	Use:   "audit [path]",
	Short: "Run all smell detectors and emit findings.",
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
		report := &Report{Root: root, Tags: resolvedTags()}
		report.Findings = append(report.Findings, scanReceivers(pkgs)...)
		report.Findings = append(report.Findings, scanStutter(pkgs)...)
		report.Findings = append(report.Findings, scanFacades(pkgs)...)
		report.Findings = append(report.Findings, scanDepsBag(pkgs)...)
		report.Findings = append(report.Findings, scanMixedConcern(pkgs)...)
		report.Findings = append(report.Findings, scanInitCoupling(pkgs)...)
		report.Findings = append(report.Findings, scanReExportTunnel(pkgs)...)
		report.Findings = append(report.Findings, scanFS(root, pkgs)...)
		return emit(report)
	},
}
