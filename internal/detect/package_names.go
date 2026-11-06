package detect

import (
	"fmt"
	"go/types"
	"path/filepath"

	"golang.org/x/tools/go/packages"

	"github.com/CaliLuke/lagotto/internal/audit"
)

// genericPackageNames excludes models/types: those can accurately describe
// cohesive schemas and DTOs, so a name alone is insufficient evidence.
var genericPackageNames = map[string]bool{
	"util": true, "utils": true, "common": true, "helper": true, "helpers": true,
	"misc": true, "shared": true, "base": true, "lib": true,
}

// ScanGenericPackages extends G11 to loaded package names, independently of
// directory spelling. Production importer counts come from the filesystem pass.
func ScanGenericPackages(root string, pkgs []*packages.Package, dirs map[string]packageDirContents) []audit.Finding {
	var findings []audit.Finding
	for _, pkg := range pkgs {
		if pkg == nil || !genericPackageNames[pkg.Name] || isTestPackage(pkg.PkgPath) || pkg.Types == nil {
			continue
		}
		dir := filepath.Dir(firstFilename(pkg))
		var contents packageDirContents
		for candidate, entry := range dirs {
			if samePath(candidate, dir) {
				contents = entry
				break
			}
		}
		if len(contents.files) == 0 || contents.importsTesting {
			continue
		}
		authored := map[string]bool{}
		for _, file := range contents.files {
			authored[file] = true
		}
		exports := 0
		if pkg.Fset != nil {
			for _, name := range pkg.Types.Scope().Names() {
				obj := pkg.Types.Scope().Lookup(name)
				if obj.Exported() && authored[filepath.Base(pkg.Fset.PositionFor(obj.Pos(), false).Filename)] {
					exports++
				}
			}
			// Exported methods are identifiers callers can use too.
			for _, name := range pkg.Types.Scope().Names() {
				obj, ok := pkg.Types.Scope().Lookup(name).(*types.TypeName)
				if !ok || obj.IsAlias() {
					continue
				}
				named, ok := obj.Type().(*types.Named)
				if !ok {
					continue
				}
				for i := 0; i < named.NumMethods(); i++ {
					method := named.Method(i)
					if method.Exported() && authored[filepath.Base(pkg.Fset.PositionFor(method.Pos(), false).Filename)] {
						exports++
					}
				}
			}
		}
		severity := audit.SevLow
		classification := "naming_nit"
		if exports >= 10 && len(contents.importers) >= 3 {
			severity = audit.SevMedium
			classification = "accumulation_risk"
		}
		findings = append(findings, audit.Finding{
			Smell: "Generic Package Name", SmellID: "G11", Severity: severity, Location: filesystemLocation(root, dir),
			Message:    fmt.Sprintf("Package %q does not describe its responsibility (%d exported identifiers, %d production importers).", pkg.Name, exports, len(contents.importers)),
			Evidence:   map[string]any{"kind": "package", "package": pkg.PkgPath, "name": pkg.Name, "file_count": len(contents.files), "exported_identifier_count": exports, "importer_count": len(contents.importers), "importers": contents.importers, "classification": classification},
			Suggestion: "Name the package for the capability it provides. Split it only when its declarations serve independently changing concerns; cohesive schema and DTO packages may keep descriptive domain names.",
		})
	}
	return findings
}
