package detect

import (
	"fmt"
	"go/types"

	"golang.org/x/tools/go/packages"

	"github.com/CaliLuke/lagotto/internal/audit"
)

const highReceiverCouplingFloor = 3

type receiverCouplingEvidence struct {
	operational map[string]bool
	state       map[string]bool
}

// calibrateG1Severity adds consumer-side evidence to G1. Method count and
// inferred verbs alone remain a MEDIUM review signal. Stored DI/state fields
// are reported separately from operational signatures and cannot elevate a
// finding by themselves. Embedding-theatre findings retain CRITICAL.
func calibrateG1Severity(findings []audit.Finding, pkgs []*packages.Package) []audit.Finding {
	candidates := map[string]bool{}
	for _, finding := range findings {
		if finding.SmellID != "G1" {
			continue
		}
		pkgPath, pkgOK := finding.Evidence["package"].(string)
		typeName, typeOK := finding.Evidence["type"].(string)
		if pkgOK && typeOK {
			candidates[pkgPath+"."+typeName] = true
		}
	}
	if len(candidates) == 0 {
		return findings
	}

	sites := receiverCouplingSites(pkgs, candidates)
	for i := range findings {
		finding := &findings[i]
		if finding.SmellID != "G1" {
			continue
		}
		pkgPath, _ := finding.Evidence["package"].(string)
		typeName, _ := finding.Evidence["type"].(string)
		key := pkgPath + "." + typeName
		coupling := sites[key]
		operationalSites := sortedKeys(coupling.operational)
		stateSites := sortedKeys(coupling.state)
		coupled := map[string]bool{}
		for _, site := range operationalSites {
			coupled[site] = true
		}
		for _, site := range stateSites {
			coupled[site] = true
		}
		coupledSites := sortedKeys(coupled)
		finding.Evidence["cross_package_signature_count"] = len(coupledSites)
		finding.Evidence["cross_package_signature_sites"] = coupledSites
		finding.Evidence["cross_package_operational_count"] = len(operationalSites)
		finding.Evidence["cross_package_operational_sites"] = operationalSites
		finding.Evidence["cross_package_state_count"] = len(stateSites)
		finding.Evidence["cross_package_state_sites"] = stateSites
		if finding.Severity != audit.SevCritical {
			finding.Severity = audit.SevMedium
			if len(operationalSites) >= highReceiverCouplingFloor || (len(operationalSites) >= 1 && len(coupledSites) >= 4) {
				finding.Severity = audit.SevHigh
				finding.Message += fmt.Sprintf(" The concrete type appears in %d operational API site(s) and %d dependency/state field(s) across package boundaries.", len(operationalSites), len(stateSites))
			}
		}
	}
	return findings
}

func receiverCouplingSites(pkgs []*packages.Package, candidates map[string]bool) map[string]receiverCouplingEvidence {
	result := map[string]receiverCouplingEvidence{}
	for _, pkg := range pkgs {
		if pkg.PkgPath == "" || pkg.Types == nil || isTestPackage(pkg.PkgPath) || pkg.Imports["testing"] != nil {
			continue
		}
		scope := pkg.Types.Scope()
		for _, name := range scope.Names() {
			switch object := scope.Lookup(name).(type) {
			case *types.Func:
				recordFunctionReceiverCoupling(result, candidates, pkg.PkgPath, pkg.PkgPath+"."+object.Name(), object)
			case *types.TypeName:
				recordTypeDeclarationCoupling(result, candidates, pkg.PkgPath, object)
			}
		}
	}
	return result
}

func recordFunctionReceiverCoupling(result map[string]receiverCouplingEvidence, candidates map[string]bool, consumer, site string, function *types.Func) {
	signature, ok := function.Type().(*types.Signature)
	if !ok {
		return
	}
	parameterMatches := referencedReceiverCandidates(signature.Params(), candidates)
	for key := range parameterMatches {
		recordReceiverCouplingKey(result, key, consumer, site, false)
	}
	// A result-only occurrence is a producer, not a consumer. Retain exported
	// non-constructor accessors because they deliberately expose the concrete
	// type as an operational package boundary.
	if !function.Exported() || hasConstructorPrefix(function.Name()) {
		return
	}
	for key := range referencedReceiverCandidates(signature.Results(), candidates) {
		recordReceiverCouplingKey(result, key, consumer, site, false)
	}
}

func hasConstructorPrefix(name string) bool {
	for _, prefix := range []string{"New", "Open", "Make", "Build", "Create"} {
		if len(name) > len(prefix) && name[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

func recordTypeDeclarationCoupling(result map[string]receiverCouplingEvidence, candidates map[string]bool, consumer string, object *types.TypeName) {
	if object.IsAlias() {
		recordReceiverCoupling(result, candidates, consumer, consumer+"."+object.Name(), object.Type(), false)
		return
	}
	named, ok := object.Type().(*types.Named)
	if !ok {
		return
	}
	switch underlying := named.Underlying().(type) {
	case *types.Struct:
		for i := 0; i < underlying.NumFields(); i++ {
			field := underlying.Field(i)
			recordReceiverCoupling(result, candidates, consumer, consumer+"."+object.Name()+"."+field.Name(), field.Type(), true)
		}
		for i := 0; i < named.NumMethods(); i++ {
			method := named.Method(i)
			recordFunctionReceiverCoupling(result, candidates, consumer, consumer+"."+object.Name()+"."+method.Name(), method)
		}
	case *types.Interface:
		underlying.Complete()
		for i := 0; i < underlying.NumMethods(); i++ {
			method := underlying.Method(i)
			recordReceiverCoupling(result, candidates, consumer, consumer+"."+object.Name()+"."+method.Name(), method.Type(), false)
		}
	}
}

func recordReceiverCoupling(result map[string]receiverCouplingEvidence, candidates map[string]bool, consumer, site string, typ types.Type, state bool) {
	for key := range referencedReceiverCandidates(typ, candidates) {
		recordReceiverCouplingKey(result, key, consumer, site, state)
	}
}

func recordReceiverCouplingKey(result map[string]receiverCouplingEvidence, key, consumer, site string, state bool) {
	origin := key[:len(key)-len(typeNameFromKey(key))-1]
	if origin == consumer {
		return
	}
	evidence := result[key]
	if evidence.operational == nil {
		evidence.operational = map[string]bool{}
	}
	if evidence.state == nil {
		evidence.state = map[string]bool{}
	}
	if state {
		evidence.state[site] = true
	} else {
		evidence.operational[site] = true
	}
	result[key] = evidence
}

func typeNameFromKey(key string) string {
	for i := len(key) - 1; i >= 0; i-- {
		if key[i] == '.' {
			return key[i+1:]
		}
	}
	return key
}

func referencedReceiverCandidates(root types.Type, candidates map[string]bool) map[string]bool {
	matched := map[string]bool{}
	seen := map[types.Type]bool{}
	var visit func(types.Type)
	visit = func(typ types.Type) {
		if typ == nil || seen[typ] {
			return
		}
		seen[typ] = true
		switch current := typ.(type) {
		case *types.Alias:
			visit(types.Unalias(current))
		case *types.Named:
			if object := current.Obj(); object != nil && object.Pkg() != nil {
				key := object.Pkg().Path() + "." + object.Name()
				if candidates[key] {
					matched[key] = true
				}
			}
			if args := current.TypeArgs(); args != nil {
				for i := 0; i < args.Len(); i++ {
					visit(args.At(i))
				}
			}
		case *types.Pointer:
			visit(current.Elem())
		case *types.Slice:
			visit(current.Elem())
		case *types.Array:
			visit(current.Elem())
		case *types.Map:
			visit(current.Key())
			visit(current.Elem())
		case *types.Chan:
			visit(current.Elem())
		case *types.Signature:
			visit(current.Params())
			visit(current.Results())
		case *types.Tuple:
			for i := 0; i < current.Len(); i++ {
				visit(current.At(i).Type())
			}
		case *types.Struct:
			for i := 0; i < current.NumFields(); i++ {
				visit(current.Field(i).Type())
			}
		case *types.Interface:
			current.Complete()
			for i := 0; i < current.NumMethods(); i++ {
				visit(current.Method(i).Type())
			}
		}
	}
	visit(root)
	return matched
}
