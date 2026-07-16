package detect

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/CaliLuke/lagotto/internal/audit"
)

type holderCandidate struct {
	key              string
	pkgPath          string
	typeName         string
	subpackageFields int
	accessorCount    int
}

type foreignHolderUse struct {
	candidate holderCandidate
	consumer  string
	location  string
	sites     map[string]bool
}

// scanForeignHolders flags G1E: broad holder types that escaped their
// producer package and remain in downstream signatures after the holder's
// services were split into subpackages.
func scanForeignHolders(pkgs []*packages.Package) []audit.Finding {
	candidates := collectHolderCandidates(pkgs)
	if len(candidates) == 0 {
		return nil
	}
	uses := map[string]*foreignHolderUse{}
	for _, pkg := range pkgs {
		if pkg.PkgPath == "" || pkg.TypesInfo == nil || isTestPackage(pkg.PkgPath) {
			continue
		}
		for i, file := range pkg.Syntax {
			filename := syntaxFilename(pkg, i, file)
			if skipSourceFile(filename, file) {
				continue
			}
			for _, decl := range file.Decls {
				switch d := decl.(type) {
				case *ast.FuncDecl:
					recordForeignHolderFunctionUses(uses, candidates, pkg, filename, d)
				case *ast.GenDecl:
					for _, spec := range d.Specs {
						ts, ok := spec.(*ast.TypeSpec)
						if !ok || isTestDouble(ts.Name.Name) {
							continue
						}
						st, ok := ts.Type.(*ast.StructType)
						if !ok || st.Fields == nil {
							continue
						}
						for _, field := range st.Fields.List {
							if len(field.Names) == 0 || !field.Names[0].IsExported() {
								continue
							}
							for _, candidate := range holderCandidatesInType(pkg.TypesInfo.TypeOf(field.Type), candidates) {
								if candidate.pkgPath == pkg.PkgPath {
									continue
								}
								site := fmt.Sprintf("%s:%s.%s", sourceLocation(pkg, filename), ts.Name.Name, field.Names[0].Name)
								addForeignHolderUse(uses, candidate, pkg.PkgPath, packageLocation(pkg), site)
							}
						}
					}
				}
			}
		}
	}

	packageCounts := map[string]map[string]bool{}
	siteCounts := map[string]int{}
	for _, use := range uses {
		if packageCounts[use.candidate.key] == nil {
			packageCounts[use.candidate.key] = map[string]bool{}
		}
		packageCounts[use.candidate.key][use.consumer] = true
		siteCounts[use.candidate.key] += len(use.sites)
	}

	var findings []audit.Finding
	for _, useKey := range sortedKeys(uses) {
		use := uses[useKey]
		sites := sortedKeys(use.sites)
		sev := audit.SevHigh
		if siteCounts[use.candidate.key] >= 5 && len(packageCounts[use.candidate.key]) >= 3 {
			sev = audit.SevCritical
		}
		findings = append(findings, audit.Finding{
			Smell:    "Foreign Holder",
			SmellID:  "G1E",
			Severity: sev,
			Location: use.location + " (*" + use.candidate.typeName + ")",
			Message: fmt.Sprintf("Package %s keeps broad holder *%s from %s in %d production signature site(s), preserving the holder as a cross-package chokepoint.",
				use.consumer, use.candidate.typeName, use.candidate.pkgPath, len(sites)),
			Evidence: map[string]any{
				"holder_package":         use.candidate.pkgPath,
				"holder_type":            use.candidate.typeName,
				"consumer_package":       use.consumer,
				"sites":                  sites,
				"site_count":             len(sites),
				"subpackage_field_count": use.candidate.subpackageFields,
				"accessor_count":         use.candidate.accessorCount,
			},
			Suggestion: "Replace *" + use.candidate.typeName + " in these signatures with the narrow sub-service each consumer actually uses. Constructors may assemble the services, but production callers should not reach through the broad holder.",
		})
	}
	return findings
}

func collectHolderCandidates(pkgs []*packages.Package) map[string]holderCandidate {
	candidates := map[string]holderCandidate{}
	for _, pkg := range pkgs {
		if pkg.PkgPath == "" || pkg.Types == nil || isTestPackage(pkg.PkgPath) {
			continue
		}
		scope := pkg.Types.Scope()
		accessors := map[string]int{}
		for _, name := range scope.Names() {
			fn, ok := scope.Lookup(name).(*types.Func)
			if !ok || !fn.Exported() {
				continue
			}
			sig, ok := fn.Type().(*types.Signature)
			if !ok || sig.Recv() != nil || sig.Params().Len() == 0 || sig.Results().Len() == 0 {
				continue
			}
			holder := pointerNamed(sig.Params().At(0).Type())
			result := pointerNamed(sig.Results().At(0).Type())
			if holder == nil || result == nil || holder.Obj().Pkg() != pkg.Types || result.Obj().Pkg() == nil {
				continue
			}
			if strings.HasPrefix(result.Obj().Pkg().Path(), pkg.PkgPath+"/") {
				accessors[holder.Obj().Name()]++
			}
		}
		for _, name := range scope.Names() {
			if isTestDouble(name) {
				continue
			}
			obj, ok := scope.Lookup(name).(*types.TypeName)
			if !ok || obj.IsAlias() {
				continue
			}
			named, ok := obj.Type().(*types.Named)
			if !ok {
				continue
			}
			st, ok := named.Underlying().(*types.Struct)
			if !ok {
				continue
			}
			fieldCount := 0
			for i := 0; i < st.NumFields(); i++ {
				fieldType := pointerNamed(st.Field(i).Type())
				if fieldType != nil && fieldType.Obj().Pkg() != nil && strings.HasPrefix(fieldType.Obj().Pkg().Path(), pkg.PkgPath+"/") {
					fieldCount++
				}
			}
			if fieldCount < 3 && accessors[name] < 3 {
				continue
			}
			key := pkg.PkgPath + "." + name
			candidates[key] = holderCandidate{key: key, pkgPath: pkg.PkgPath, typeName: name, subpackageFields: fieldCount, accessorCount: accessors[name]}
		}
	}
	return candidates
}

func recordForeignHolderFunctionUses(uses map[string]*foreignHolderUse, candidates map[string]holderCandidate, pkg *packages.Package, filename string, fn *ast.FuncDecl) {
	symbol := fn.Name.Name
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		symbol = receiverTypeName(pkg.TypesInfo, fn.Recv.List[0]) + "." + symbol
	}
	for _, fieldList := range []*ast.FieldList{fn.Type.Params} {
		if fieldList == nil {
			continue
		}
		for _, field := range fieldList.List {
			for _, candidate := range holderCandidatesInType(pkg.TypesInfo.TypeOf(field.Type), candidates) {
				if candidate.pkgPath != pkg.PkgPath {
					addForeignHolderUse(uses, candidate, pkg.PkgPath, packageLocation(pkg), sourceLocation(pkg, filename)+":"+symbol)
				}
			}
		}
	}
	if fn.Type.Results == nil {
		return
	}
	for _, field := range fn.Type.Results.List {
		for _, candidate := range holderCandidatesInType(pkg.TypesInfo.TypeOf(field.Type), candidates) {
			if candidate.pkgPath == pkg.PkgPath || isHolderConstructor(fn.Name.Name, candidate.typeName) {
				continue
			}
			addForeignHolderUse(uses, candidate, pkg.PkgPath, packageLocation(pkg), sourceLocation(pkg, filename)+":"+symbol)
		}
	}
}

func holderCandidatesInType(t types.Type, candidates map[string]holderCandidate) []holderCandidate {
	named := pointerNamed(t)
	if named == nil || named.Obj().Pkg() == nil {
		return nil
	}
	if candidate, ok := candidates[named.Obj().Pkg().Path()+"."+named.Obj().Name()]; ok {
		return []holderCandidate{candidate}
	}
	return nil
}

func pointerNamed(t types.Type) *types.Named {
	if t == nil {
		return nil
	}
	t = types.Unalias(t)
	ptr, ok := t.(*types.Pointer)
	if !ok {
		return nil
	}
	named, _ := types.Unalias(ptr.Elem()).(*types.Named)
	return named
}

func isHolderConstructor(functionName, typeName string) bool {
	for _, prefix := range []string{"New", "Open", "Make", "Build", "Create"} {
		if functionName == prefix+typeName {
			return true
		}
	}
	return false
}

func addForeignHolderUse(uses map[string]*foreignHolderUse, candidate holderCandidate, consumer, location, site string) {
	key := candidate.key + "->" + consumer
	if uses[key] == nil {
		uses[key] = &foreignHolderUse{candidate: candidate, consumer: consumer, location: location, sites: map[string]bool{}}
	}
	uses[key].sites[site] = true
}
