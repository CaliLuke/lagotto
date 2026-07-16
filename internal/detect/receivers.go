package detect

import (
	"fmt"
	"go/types"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/types/typeutil"

	"github.com/CaliLuke/lagotto/internal/audit"
)

// ScanReceivers returns Receiver Monolith (G1) findings computed from
// the type-checker's full method set on each named struct, plus
// Decomposition Theatre (G1B) findings for alias clusters that pretend
// to split a god type without actually moving methods.
//
// Counting via the method set (rather than source-AST receiver names)
// is critical: a god type that "decomposes" itself by embedding a
// helper struct and adding nine type aliases (`type Mutator = helper`)
// is the same monolith with a costume on. The method-set view sees
// through both tricks because Go's type system promotes methods
// through embedding and resolves aliases.
func ScanReceivers(pkgs []*packages.Package) []audit.Finding {
	var findings []audit.Finding
	for _, pkg := range pkgs {
		if pkg.PkgPath == "" || isTestPackage(pkg.PkgPath) {
			continue
		}
		if pkg.Types == nil || pkg.Fset == nil {
			continue
		}
		var cache typeutil.MethodSetCache
		findings = append(findings, scanMethodSets(pkg, &cache)...)
		findings = append(findings, scanAliasClusters(pkg)...)
		findings = append(findings, scanAggregateHolders(pkg, &cache)...)
		findings = append(findings, scanHiddenHolders(pkg, &cache)...)
	}
	findings = append(findings, scanForeignHolders(pkgs)...)
	return findings
}

// scanHiddenHolders flags the third-stage Decomposition Theatre pattern:
// a "thin" holder struct with no methods of its own, paired with
// package-level sync.Map (or pointer-keyed map) registries and exported
// accessor functions of the form `func Foo(h *Holder) *Sub { ... }`.
//
// Once aliases (G1B) and aggregate-holder fields (G1C) stop fooling
// the auditor, the next evasion is to keep the holder type narrow
// (one or zero fields) while reconstructing its API surface via
// package-level registries keyed by the holder's pointer. Every
// exported accessor takes the holder, looks up the per-instance
// service in the registry, and returns it. Callers still receive
// `*Holder` everywhere — the receiver split is cosmetic.
//
// Detection signal:
//   - The package has ≥3 package-level vars typed sync.Map (or any
//     map whose key type is a pointer).
//   - The package has ≥5 exported functions whose first parameter is
//     a pointer to a same-package struct H.
//   - H itself has ≤2 of its own methods (so it's not a real receiver
//     covered by G1; it's a hidden holder).
//
// Test doubles (Fake/Mock/Stub/Spy types and testutil packages) are
// skipped, matching G1's filter.
func scanHiddenHolders(pkg *packages.Package, caches ...*typeutil.MethodSetCache) []audit.Finding {
	cache := selectMethodSetCache(caches)
	scope := pkg.Types.Scope()

	registryVars := collectRegistryVars(scope, pkg)
	if len(registryVars) < 3 {
		return nil
	}

	accessorsByHolder := collectHolderAccessors(scope, pkg)

	var findings []audit.Finding
	for holderName, accessors := range accessorsByHolder {
		if len(accessors) < 5 {
			continue
		}
		obj := scope.Lookup(holderName)
		if obj == nil {
			continue
		}
		named, ok := obj.Type().(*types.Named)
		if !ok {
			continue
		}
		ms := cache.MethodSet(types.NewPointer(named))
		ownMethods := 0
		for i := 0; i < ms.Len(); i++ {
			if fn, ok := ms.At(i).Obj().(*types.Func); ok && fn.Pkg() == pkg.Types {
				ownMethods++
			}
		}
		if ownMethods > 2 {
			// Real receiver — let G1 own the finding.
			continue
		}
		sort.Strings(accessors)
		sev := audit.SevHigh
		if len(accessors) >= 7 || len(registryVars) >= 6 {
			sev = audit.SevCritical
		}
		findings = append(findings, audit.Finding{
			Smell:    "Hidden Holder",
			SmellID:  "G1D",
			Severity: sev,
			Location: packageLocation(pkg) + " (" + holderName + ")",
			Message: fmt.Sprintf("Type %s has %d own methods but %d exported package-level functions take *%s as their first argument, with %d package-level pointer-keyed registry maps (sync.Map or map[*T]...) in the same package. The package is reconstructing the holder's API surface via registry maps; callers still receive *%s everywhere, so the receiver split is cosmetic.",
				holderName, ownMethods, len(accessors), holderName, len(registryVars), holderName),
			Evidence: map[string]any{
				"package":            pkg.PkgPath,
				"holder":             holderName,
				"own_method_count":   ownMethods,
				"accessor_count":     len(accessors),
				"accessor_funcs":     accessors,
				"registry_var_count": len(registryVars),
				"registry_vars":      registryVars,
			},
			Suggestion: "Move each sub-service into its own subpackage and return them as separate values from the constructor. Callers take only the narrow sub-service they need; nobody takes *" + holderName + " in a production code path. Delete the package-level registry maps and the accessor functions in the same change.",
		})
	}
	return findings
}

// collectRegistryVars returns the names of package-level vars whose
// type is sync.Map or a Go map with a pointer key. These are the
// candidate "registry" maps that a hidden holder uses to simulate
// per-instance fields without declaring them on the holder struct.
func collectRegistryVars(scope *types.Scope, pkg *packages.Package) []string {
	var out []string
	for _, name := range scope.Names() {
		obj, ok := scope.Lookup(name).(*types.Var)
		if !ok || obj.Pkg() != pkg.Types {
			continue
		}
		if isRegistryType(obj.Type()) {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// isRegistryType reports whether t is sync.Map or a map whose key is
// a pointer type. Both shapes are pointer-keyed lookup tables that
// behave like per-instance fields on the pointer's pointee.
func isRegistryType(t types.Type) bool {
	t = types.Unalias(t)
	if named, ok := t.(*types.Named); ok {
		obj := named.Obj()
		if obj != nil && obj.Pkg() != nil &&
			obj.Pkg().Path() == "sync" && obj.Name() == "Map" {
			return true
		}
	}
	if m, ok := t.Underlying().(*types.Map); ok {
		if _, ptr := types.Unalias(m.Key()).(*types.Pointer); ptr {
			return true
		}
	}
	return false
}

// collectHolderAccessors returns a map of `Holder` type name →
// exported package-level function names whose first parameter is a
// pointer to that same-package struct. Methods (functions with a
// receiver) and unexported functions are excluded.
func collectHolderAccessors(scope *types.Scope, pkg *packages.Package) map[string][]string {
	out := map[string][]string{}
	for _, name := range scope.Names() {
		obj, ok := scope.Lookup(name).(*types.Func)
		if !ok || !obj.Exported() || obj.Pkg() != pkg.Types {
			continue
		}
		sig, ok := obj.Type().(*types.Signature)
		if !ok || sig.Recv() != nil || sig.Params().Len() == 0 {
			continue
		}
		paramType := types.Unalias(sig.Params().At(0).Type())
		ptr, ok := paramType.(*types.Pointer)
		if !ok {
			continue
		}
		named, ok := types.Unalias(ptr.Elem()).(*types.Named)
		if !ok || named.Obj().Pkg() != pkg.Types {
			continue
		}
		if _, isStruct := named.Underlying().(*types.Struct); !isStruct {
			continue
		}
		holderName := named.Obj().Name()
		if isTestDouble(holderName) {
			continue
		}
		out[holderName] = append(out[holderName], obj.Name())
	}
	return out
}

// scanAggregateHolders flags the second-stage Decomposition Theatre
// pattern: a struct whose fields are 5+ pointers to other named types
// in the same package, where the pointee types collectively own a
// large method set. After aliases-to-one-struct fails to fool the
// linter, the next move is to make sub-services (`*Mutator`,
// `*Searcher`, ...) and aggregate them on a holder (`TypeDB { Nodes,
// Edges, Search, ... }`). Callers still pass one handle around, so
// the receiver split isn't real until those sub-services move into
// their own subpackages and callers take only the one they need.
func scanAggregateHolders(pkg *packages.Package, caches ...*typeutil.MethodSetCache) []audit.Finding {
	cache := selectMethodSetCache(caches)
	scope := pkg.Types.Scope()
	var findings []audit.Finding
	for _, name := range scope.Names() {
		if isTestDouble(name) {
			continue
		}
		obj := scope.Lookup(name)
		tn, ok := obj.(*types.TypeName)
		if !ok || tn.IsAlias() {
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
		var samePkgFields []string
		totalMethods := 0
		for i := 0; i < st.NumFields(); i++ {
			f := st.Field(i)
			ft := types.Unalias(f.Type())
			if ptr, ok := ft.(*types.Pointer); ok {
				ft = types.Unalias(ptr.Elem())
			}
			fnamed, ok := ft.(*types.Named)
			if !ok {
				continue
			}
			if fnamed.Obj().Pkg() == nil || fnamed.Obj().Pkg() != pkg.Types {
				continue
			}
			if _, isStruct := fnamed.Underlying().(*types.Struct); !isStruct {
				continue
			}
			ms := cache.MethodSet(types.NewPointer(fnamed))
			methodCount := 0
			for j := 0; j < ms.Len(); j++ {
				fn, ok := ms.At(j).Obj().(*types.Func)
				if ok && fn.Pkg() == pkg.Types {
					methodCount++
				}
			}
			if methodCount == 0 {
				continue
			}
			samePkgFields = append(samePkgFields, f.Name()+" *"+fnamed.Obj().Name())
			totalMethods += methodCount
		}
		if len(samePkgFields) < 5 {
			continue
		}
		if totalMethods < 25 {
			continue
		}
		sev := audit.SevHigh
		if totalMethods >= 50 || len(samePkgFields) >= 7 {
			sev = audit.SevCritical
		}
		sort.Strings(samePkgFields)
		findings = append(findings, audit.Finding{
			Smell:    "Aggregate Holder",
			SmellID:  "G1C",
			Severity: sev,
			Location: packageLocation(pkg) + " (" + name + ")",
			Message: fmt.Sprintf("Struct %s aggregates %d same-package sub-services with %d total methods. Callers still pass one %s handle, so the receiver split is cosmetic — the sub-services have not moved into their own packages.",
				name, len(samePkgFields), totalMethods, name),
			Evidence: map[string]any{
				"package":               pkg.PkgPath,
				"type":                  name,
				"sub_service_count":     len(samePkgFields),
				"sub_service_fields":    samePkgFields,
				"total_pointee_methods": totalMethods,
			},
			Suggestion: "Move each sub-service struct into its own subpackage and update callers to take only the narrow service they need. Delete the holder type, or reduce it to a constructor that returns the sub-services as separate values rather than fields on a shared struct. A holder that every caller still receives is functionally a god type with extra punctuation.",
		})
	}
	return findings
}

// scanMethodSets walks each package's named struct types, builds the
// effective method set on the pointer-receiver, filters to methods
// declared in the same package (so we don't flag thin wrappers around
// external types like *sql.DB), and emits a finding when the type
// crosses the Receiver Monolith thresholds.
func scanMethodSets(pkg *packages.Package, caches ...*typeutil.MethodSetCache) []audit.Finding {
	cache := selectMethodSetCache(caches)
	scope := pkg.Types.Scope()
	var findings []audit.Finding
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		tn, ok := obj.(*types.TypeName)
		if !ok || tn.IsAlias() {
			continue
		}
		named, ok := obj.Type().(*types.Named)
		if !ok {
			continue
		}
		if _, isStruct := named.Underlying().(*types.Struct); !isStruct {
			continue
		}
		if isTestDouble(name) {
			continue
		}
		ptr := types.NewPointer(named)
		ms := cache.MethodSet(ptr)
		if ms.Len() == 0 {
			continue
		}

		var methodNames []string
		files := map[string]bool{}
		promotedFrom := map[string]int{}
		localCount := 0
		for i := 0; i < ms.Len(); i++ {
			sel := ms.At(i)
			fn, ok := sel.Obj().(*types.Func)
			if !ok || fn.Pkg() == nil || fn.Pkg() != pkg.Types {
				continue
			}
			methodNames = append(methodNames, fn.Name())
			if pos := pkg.Fset.Position(fn.Pos()); pos.IsValid() {
				files[filepath.Base(pos.Filename)] = true
			}
			if len(sel.Index()) > 1 {
				if from := embeddedFieldType(named, sel.Index()[0]); from != "" {
					promotedFrom[from]++
				}
			} else {
				localCount++
			}
		}

		if len(methodNames) < 15 {
			continue
		}
		concerns := detectConcerns(methodNames)
		if len(concerns) < 3 {
			continue
		}

		// File-count is reported as evidence but not gated. A
		// monolith squeezed into 1–2 files is still a monolith;
		// the original heuristic missed types that had been
		// "decomposed" by reshuffling files within one package.
		sev := audit.SevHigh
		if len(methodNames) >= 25 || len(files) >= 7 {
			sev = audit.SevCritical
		}

		// Detect "decomposition theatre" via embedding: most methods
		// promoted from a single same-package embedded type. The
		// embedded type is ALSO flagged in its own right by this
		// scanner; the hint just tells the reader the outer type's
		// monolith problem is solved by removing the embedding, not
		// by reshuffling files.
		var theatreNote string
		biggestPromoter, biggestPromoterCount := dominantPromoter(promotedFrom)
		if biggestPromoter != "" && biggestPromoterCount*2 > len(methodNames) {
			theatreNote = fmt.Sprintf(" %d/%d methods are promoted via embedded *%s — removing the embedding is the structural fix, not file moves.",
				biggestPromoterCount, len(methodNames), biggestPromoter)
		}

		ev := map[string]any{
			"package":      pkg.PkgPath,
			"type":         name,
			"method_count": len(methodNames),
			"file_count":   len(files),
			"files":        sortedKeys(files),
			"concerns":     concerns,
			"methods":      sortedCopy(methodNames),
			"local_count":  localCount,
		}
		if len(promotedFrom) > 0 {
			ev["promoted_from"] = promotedFrom
		}

		suggestion := "Decompose " + name + " into per-concern receiver types in subpackages, one per concern group. Each subpackage exports its own struct holding only the state it needs. Delete " + name + " (or reduce it to a tiny construction helper). Update every caller in the same change. Do not retain accessors, facade methods, embedding shortcuts, or type aliases on the original type."
		if theatreNote != "" {
			suggestion += " IMPORTANT: this type's method set is dominated by methods promoted from an embedded same-package type — file moves and renamed receivers will not fix this; the embedding itself is the smell."
		}

		findings = append(findings, audit.Finding{
			Smell:    "Receiver Monolith",
			SmellID:  "G1",
			Severity: sev,
			Location: packageLocation(pkg) + " (" + name + ")",
			Message: fmt.Sprintf("Type %s has %d methods (effective method set, including promoted) across %d files spanning %d concern groups (%s).%s",
				name, len(methodNames), len(files), len(concerns), strings.Join(concerns, ", "), theatreNote),
			Evidence:   ev,
			Suggestion: suggestion,
		})
	}
	return findings
}

func dominantPromoter(promotedFrom map[string]int) (string, int) {
	name, count := "", 0
	for _, candidate := range sortedKeys(promotedFrom) {
		if promotedFrom[candidate] > count {
			name = candidate
			count = promotedFrom[candidate]
		}
	}
	return name, count
}

func selectMethodSetCache(caches []*typeutil.MethodSetCache) *typeutil.MethodSetCache {
	if len(caches) > 0 && caches[0] != nil {
		return caches[0]
	}
	return new(typeutil.MethodSetCache)
}

// scanAliasClusters reports the alias-cluster Decomposition Theatre
// pattern: 3+ exported type aliases in the same package whose RHS is
// a single underlying type. This is the trick used to make `lagotto
// monoliths` go quiet while keeping every method on one struct (with
// names like `Mutator = graphOps`, `Searcher = graphOps`, ...).
func scanAliasClusters(pkg *packages.Package) []audit.Finding {
	scope := pkg.Types.Scope()
	clusters := map[string][]string{}
	for _, name := range scope.Names() {
		if isTestDouble(name) {
			continue
		}
		obj := scope.Lookup(name)
		tn, ok := obj.(*types.TypeName)
		if !ok || !tn.IsAlias() {
			continue
		}
		// Go 1.22+ models aliases as *types.Alias; types.Unalias()
		// resolves through any chain of aliases to the underlying
		// concrete type so we can compare cluster targets by identity.
		target := types.Unalias(obj.Type())
		if ptr, ok := target.(*types.Pointer); ok {
			target = ptr.Elem()
		}
		named, ok := target.(*types.Named)
		if !ok {
			continue
		}
		if named.Obj().Pkg() == nil || named.Obj().Pkg() != pkg.Types {
			continue // alias to external type — not interesting here
		}
		if isTestDouble(named.Obj().Name()) {
			continue
		}
		clusters[named.Obj().Name()] = append(clusters[named.Obj().Name()], name)
	}

	var findings []audit.Finding
	for target, aliases := range clusters {
		if len(aliases) < 3 {
			continue
		}
		sort.Strings(aliases)
		sev := audit.SevHigh
		if len(aliases) >= 6 {
			sev = audit.SevCritical
		}
		findings = append(findings, audit.Finding{
			Smell:    "Decomposition Theatre",
			SmellID:  "G1B",
			Severity: sev,
			Location: packageLocation(pkg) + " (alias cluster -> " + target + ")",
			Message: fmt.Sprintf("Package %s declares %d type aliases that all resolve to %s. This is structural fan-out, not decomposition: every alias inherits the same method set on the same struct, so the receiver remains a monolith no matter how many names point at it.",
				pkg.Name, len(aliases), target),
			Evidence: map[string]any{
				"package":     pkg.PkgPath,
				"target_type": target,
				"aliases":     aliases,
				"alias_count": len(aliases),
			},
			Suggestion: "Replace each alias with a distinct struct in its own subpackage, holding only the state it needs. The shared underlying type (" + target + ") should be deleted; aliasing it under multiple names does not split the monolith. Update callers to take the new narrow types directly.",
		})
	}
	return findings
}
