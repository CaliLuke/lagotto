package detect

import (
	"fmt"
	"go/types"
	"path/filepath"
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
		if pkg.PkgPath == "" || pkg.Types == nil || isTestPackage(pkg.PkgPath) {
			continue
		}
		scope := pkg.Types.Scope()
		for _, name := range scope.Names() {
			obj := scope.Lookup(name)
			switch object := obj.(type) {
			case *types.Func:
				recordForeignHolderSignatureUses(uses, candidates, pkg, object, object.Name())
			case *types.TypeName:
				if isTestDouble(object.Name()) {
					continue
				}
				named, ok := types.Unalias(object.Type()).(*types.Named)
				if !ok {
					continue
				}
				if st, ok := named.Underlying().(*types.Struct); ok {
					for i := 0; i < st.NumFields(); i++ {
						field := st.Field(i)
						if !field.Exported() {
							continue
						}
						for _, candidate := range holderCandidatesInType(field.Type(), candidates) {
							if candidate.pkgPath == pkg.PkgPath {
								continue
							}
							site := fmt.Sprintf("%s:%s.%s", objectSourceLocation(pkg, field), object.Name(), field.Name())
							addForeignHolderUse(uses, candidate, pkg.PkgPath, packageLocation(pkg), site)
						}
					}
				}
				for i := 0; i < named.NumMethods(); i++ {
					method := named.Method(i)
					recordForeignHolderSignatureUses(uses, candidates, pkg, method, object.Name()+"."+method.Name())
				}
			}
		}
	}

	var findings []audit.Finding
	for _, useKey := range sortedKeys(uses) {
		use := uses[useKey]
		sites := sortedKeys(use.sites)
		findings = append(findings, audit.Finding{
			Smell:    "Foreign Holder",
			SmellID:  "G1E",
			Severity: audit.SevMedium,
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

func recordForeignHolderSignatureUses(uses map[string]*foreignHolderUse, candidates map[string]holderCandidate, pkg *packages.Package, fn *types.Func, symbol string) {
	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return
	}
	site := objectSourceLocation(pkg, fn) + ":" + symbol
	for i := 0; i < sig.Params().Len(); i++ {
		for _, candidate := range holderCandidatesInType(sig.Params().At(i).Type(), candidates) {
			if candidate.pkgPath != pkg.PkgPath {
				addForeignHolderUse(uses, candidate, pkg.PkgPath, packageLocation(pkg), site)
			}
		}
	}
	for i := 0; i < sig.Results().Len(); i++ {
		for _, candidate := range holderCandidatesInType(sig.Results().At(i).Type(), candidates) {
			if candidate.pkgPath == pkg.PkgPath || isHolderConstructor(fn.Name(), candidate.typeName) {
				continue
			}
			addForeignHolderUse(uses, candidate, pkg.PkgPath, packageLocation(pkg), site)
		}
	}
}

func objectSourceLocation(pkg *packages.Package, object types.Object) string {
	if pkg.Fset != nil {
		if pos := pkg.Fset.Position(object.Pos()); pos.IsValid() && pos.Filename != "" {
			return sourceLocation(pkg, filepath.Base(pos.Filename))
		}
	}
	return packageLocation(pkg)
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
