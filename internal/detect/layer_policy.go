package detect

import (
	"fmt"
	"go/ast"
	"go/types"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/tools/go/packages"

	"github.com/CaliLuke/lagotto/internal/audit"
)

// LayerPolicyRule is one resolved, repository-owned G14 policy. Patterns may
// be full import paths or module-relative globs using *, **, and ?.
type LayerPolicyRule struct {
	Name                       string
	Paths                      []string
	Dependencies               []string
	GeneratedTypes             []string
	MaxCoordinatedDependencies int
	Severity                   audit.Severity
}

type layerDependencyCall struct {
	Type    string `json:"type"`
	Package string `json:"package"`
	Method  string `json:"method"`
	Kind    string `json:"kind"`
	Line    int    `json:"line"`
}

type layerGeneratedMapping struct {
	Type    string `json:"type"`
	Package string `json:"package"`
	Kind    string `json:"kind"`
	Line    int    `json:"line"`
}

type policyTypeRef struct {
	key     string
	name    string
	pkgPath string
}

// ValidateLayerPolicyRules rejects incomplete policy and malformed severity
// before package loading begins.
func ValidateLayerPolicyRules(rules []LayerPolicyRule) error {
	seen := map[string]bool{}
	for index, rule := range rules {
		prefix := fmt.Sprintf("layer policy %d", index)
		if rule.Name == "" {
			return fmt.Errorf("%s has no name", prefix)
		}
		if seen[rule.Name] {
			return fmt.Errorf("duplicate layer policy name %q", rule.Name)
		}
		seen[rule.Name] = true
		if len(rule.Paths) == 0 || len(rule.Dependencies) == 0 || len(rule.GeneratedTypes) == 0 {
			return fmt.Errorf("layer policy %q must define paths, dependencies, and generated types", rule.Name)
		}
		if rule.MaxCoordinatedDependencies < 0 {
			return fmt.Errorf("layer policy %q max coordinated dependencies cannot be negative", rule.Name)
		}
		if _, ok := audit.ParseSeverity(string(rule.Severity)); !ok {
			return fmt.Errorf("layer policy %q has unknown severity %q", rule.Name, rule.Severity)
		}
		for _, patterns := range [][]string{rule.Paths, rule.Dependencies, rule.GeneratedTypes} {
			for _, pattern := range patterns {
				if pattern == "" {
					return fmt.Errorf("layer policy %q contains an empty pattern", rule.Name)
				}
				if _, err := compilePolicyPattern(pattern); err != nil {
					return fmt.Errorf("layer policy %q pattern %q: %w", rule.Name, pattern, err)
				}
			}
		}
	}
	return nil
}

// ScanLayerPolicy reports configured cross-layer orchestration. It is opt-in:
// no rules means no findings. The detector uses typed call targets so aliases
// and local import names do not affect its result.
func ScanLayerPolicy(pkgs []*packages.Package, rules []LayerPolicyRule) []audit.Finding {
	if len(rules) == 0 {
		return nil
	}
	var findings []audit.Finding
	for _, pkg := range pkgs {
		if pkg.PkgPath == "" || pkg.TypesInfo == nil || pkg.Fset == nil || isTestPackage(pkg.PkgPath) {
			continue
		}
		modulePath := ""
		if pkg.Module != nil {
			modulePath = pkg.Module.Path
		}
		for fileIndex, file := range pkg.Syntax {
			filename := syntaxFilename(pkg, fileIndex, file)
			if skipSourceFile(filename, file) {
				continue
			}
			location := sourceLocation(pkg, filename)
			for _, rule := range rules {
				if !matchesPolicyPatterns(rule.Paths, location, modulePath) && !matchesPolicyPatterns(rule.Paths, packageLocation(pkg), modulePath) {
					continue
				}
				for _, declaration := range file.Decls {
					function, ok := declaration.(*ast.FuncDecl)
					if !ok || function.Body == nil {
						continue
					}
					if finding, ok := scanLayerFunction(pkg, modulePath, location, function, rule); ok {
						findings = append(findings, finding)
					}
				}
			}
		}
	}
	return findings
}

func scanLayerFunction(pkg *packages.Package, modulePath, location string, function *ast.FuncDecl, rule LayerPolicyRule) (audit.Finding, bool) {
	allDependencyCalls := map[string]layerDependencyCall{}
	generatedMappings := map[string]layerGeneratedMapping{}

	ast.Inspect(function.Body, func(node ast.Node) bool {
		if _, nested := node.(*ast.FuncLit); nested {
			return false
		}
		switch current := node.(type) {
		case *ast.CallExpr:
			for _, call := range policyDependencyCalls(pkg, modulePath, current, rule.Dependencies) {
				key := fmt.Sprintf("%s:%s:%s:%d", call.Type, call.Method, call.Kind, call.Line)
				allDependencyCalls[key] = call
			}
			for _, mapping := range policyGeneratedCallMappings(pkg, modulePath, current, rule.GeneratedTypes) {
				key := fmt.Sprintf("%s:%s:%d", mapping.Type, mapping.Kind, mapping.Line)
				generatedMappings[key] = mapping
			}
		case *ast.CompositeLit:
			for _, ref := range policyTypeRefs(pkg.TypesInfo.TypeOf(current.Type)) {
				if matchesPolicyPatterns(rule.GeneratedTypes, ref.pkgPath, modulePath) {
					mapping := newLayerGeneratedMapping(pkg, current, ref, "composite_literal")
					generatedMappings[fmt.Sprintf("%s:%s:%d", mapping.Type, mapping.Kind, mapping.Line)] = mapping
				}
			}
		case *ast.AssignStmt:
			for _, lhs := range current.Lhs {
				selector, ok := lhs.(*ast.SelectorExpr)
				if !ok {
					continue
				}
				for _, ref := range policyTypeRefs(pkg.TypesInfo.TypeOf(selector.X)) {
					if matchesPolicyPatterns(rule.GeneratedTypes, ref.pkgPath, modulePath) {
						mapping := newLayerGeneratedMapping(pkg, selector, ref, "field_assignment")
						generatedMappings[fmt.Sprintf("%s:%s:%d", mapping.Type, mapping.Kind, mapping.Line)] = mapping
					}
				}
			}
		}
		return true
	})

	dependencyTypes := map[string]bool{}
	dependencyCalls := map[string]layerDependencyCall{}
	receiverPackages := map[string]bool{}
	for _, call := range allDependencyCalls {
		if call.Kind == "receiver" {
			receiverPackages[call.Package] = true
		}
	}
	for key, call := range allDependencyCalls {
		// A package helper accompanying a receiver call from the same package
		// is part of that dependency, not a second coordinated service.
		if call.Kind == "package_function" && receiverPackages[call.Package] {
			continue
		}
		dependencyTypes[call.Type] = true
		dependencyCalls[key] = call
	}

	if len(dependencyTypes) <= rule.MaxCoordinatedDependencies || len(generatedMappings) == 0 {
		return audit.Finding{}, false
	}
	calls := mapValuesSorted(dependencyCalls, func(a, b layerDependencyCall) bool {
		if a.Type != b.Type {
			return a.Type < b.Type
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Method < b.Method
	})
	mappings := mapValuesSorted(generatedMappings, func(a, b layerGeneratedMapping) bool {
		if a.Type != b.Type {
			return a.Type < b.Type
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Kind < b.Kind
	})
	functionName := layerFunctionName(pkg, function)
	return audit.Finding{
		Smell:    "Cross-Layer Orchestration",
		SmellID:  "G14",
		Severity: rule.Severity,
		Location: location + ":" + functionName,
		Message: fmt.Sprintf("Layer policy %q: %s coordinates %d configured dependency types while mapping %d configured generated type use(s).",
			rule.Name, functionName, len(dependencyTypes), len(mappings)),
		Evidence: map[string]any{
			"rule":                         rule.Name,
			"function":                     functionName,
			"dependency_type_count":        len(dependencyTypes),
			"dependency_types":             sortedKeys(dependencyTypes),
			"dependency_calls":             calls,
			"generated_mapping_count":      len(mappings),
			"generated_mappings":           mappings,
			"max_coordinated_dependencies": rule.MaxCoordinatedDependencies,
			"scope_patterns":               rule.Paths,
			"dependency_patterns":          rule.Dependencies,
			"generated_type_patterns":      rule.GeneratedTypes,
		},
		Suggestion: "Keep transport mapping at this boundary, but move multi-service/store coordination into one application service or use-case and call that narrow boundary from transport. If this orchestration is intentional for the configured layer, adjust the repository policy or suppress this exact finding.",
	}, true
}

func policyDependencyCalls(pkg *packages.Package, modulePath string, call *ast.CallExpr, patterns []string) []layerDependencyCall {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	line := pkg.Fset.Position(call.Pos()).Line
	if selection := pkg.TypesInfo.Selections[selector]; selection != nil {
		var calls []layerDependencyCall
		for _, ref := range policyTypeRefs(selection.Recv()) {
			if matchesPolicyPatterns(patterns, ref.pkgPath, modulePath) {
				calls = append(calls, layerDependencyCall{Type: ref.key, Package: ref.pkgPath, Method: selection.Obj().Name(), Kind: "receiver", Line: line})
			}
		}
		return calls
	}
	function, ok := pkg.TypesInfo.Uses[selector.Sel].(*types.Func)
	if !ok || function.Pkg() == nil || !matchesPolicyPatterns(patterns, function.Pkg().Path(), modulePath) {
		return nil
	}
	// Package constructors are wiring, not operational coordination. Counting
	// NewService plus a call on the returned Service as two dependencies turns
	// an ordinary one-service adapter into a false positive.
	if isPolicyConstructor(function) {
		return nil
	}
	return []layerDependencyCall{{Type: function.Pkg().Path(), Package: function.Pkg().Path(), Method: function.Name(), Kind: "package_function", Line: line}}
}

func isPolicyConstructor(function *types.Func) bool {
	if function == nil || function.Type() == nil {
		return false
	}
	signature, ok := function.Type().(*types.Signature)
	if !ok || signature.Recv() != nil {
		return false
	}
	name := function.Name()
	if name != "New" {
		if !strings.HasPrefix(name, "New") || len(name) == len("New") {
			return false
		}
		first, _ := utf8.DecodeRuneInString(strings.TrimPrefix(name, "New"))
		if !unicode.IsUpper(first) {
			return false
		}
	}
	results := signature.Results()
	for index := 0; index < results.Len(); index++ {
		for _, ref := range policyTypeRefs(results.At(index).Type()) {
			if ref.pkgPath == function.Pkg().Path() {
				return true
			}
		}
	}
	return false
}

func policyGeneratedCallMappings(pkg *packages.Package, modulePath string, call *ast.CallExpr, patterns []string) []layerGeneratedMapping {
	line := pkg.Fset.Position(call.Pos()).Line
	if ref, ok := calledTypeRef(pkg.TypesInfo, call.Fun); ok && matchesPolicyPatterns(patterns, ref.pkgPath, modulePath) {
		return []layerGeneratedMapping{{Type: ref.key, Package: ref.pkgPath, Kind: "type_conversion", Line: line}}
	}
	callee := calledFunction(pkg.TypesInfo, call.Fun)
	if callee == nil || callee.Pkg() == nil {
		return nil
	}
	calleeIsLocal := callee.Pkg() == pkg.Types
	calleeIsGenerated := matchesPolicyPatterns(patterns, callee.Pkg().Path(), modulePath)
	if !calleeIsLocal && !calleeIsGenerated {
		return nil
	}
	var mappings []layerGeneratedMapping
	for _, ref := range policyTypeRefs(pkg.TypesInfo.TypeOf(call)) {
		if matchesPolicyPatterns(patterns, ref.pkgPath, modulePath) {
			mappings = append(mappings, layerGeneratedMapping{Type: ref.key, Package: ref.pkgPath, Kind: "generated_result", Line: line})
		}
	}
	return mappings
}

func calledFunction(info *types.Info, expression ast.Expr) *types.Func {
	switch current := expression.(type) {
	case *ast.Ident:
		function, _ := info.Uses[current].(*types.Func)
		return function
	case *ast.SelectorExpr:
		if selection := info.Selections[current]; selection != nil {
			function, _ := selection.Obj().(*types.Func)
			return function
		}
		function, _ := info.Uses[current.Sel].(*types.Func)
		return function
	}
	return nil
}

func calledTypeRef(info *types.Info, expression ast.Expr) (policyTypeRef, bool) {
	var object types.Object
	switch current := expression.(type) {
	case *ast.Ident:
		object = info.Uses[current]
	case *ast.SelectorExpr:
		object = info.Uses[current.Sel]
	}
	typeName, ok := object.(*types.TypeName)
	if !ok {
		return policyTypeRef{}, false
	}
	refs := policyTypeRefs(typeName.Type())
	if len(refs) == 0 {
		return policyTypeRef{}, false
	}
	return refs[0], true
}

func newLayerGeneratedMapping(pkg *packages.Package, node ast.Node, ref policyTypeRef, kind string) layerGeneratedMapping {
	return layerGeneratedMapping{Type: ref.key, Package: ref.pkgPath, Kind: kind, Line: pkg.Fset.Position(node.Pos()).Line}
}

func policyTypeRefs(root types.Type) []policyTypeRef {
	seenTypes := map[types.Type]bool{}
	refs := map[string]policyTypeRef{}
	var visit func(types.Type)
	visit = func(current types.Type) {
		if current == nil || seenTypes[current] {
			return
		}
		seenTypes[current] = true
		switch value := current.(type) {
		case *types.Alias:
			visit(types.Unalias(value))
		case *types.Named:
			object := value.Obj()
			if object != nil && object.Pkg() != nil {
				ref := policyTypeRef{key: object.Pkg().Path() + "." + object.Name(), name: object.Name(), pkgPath: object.Pkg().Path()}
				refs[ref.key] = ref
			}
			if args := value.TypeArgs(); args != nil {
				for i := 0; i < args.Len(); i++ {
					visit(args.At(i))
				}
			}
		case *types.Pointer:
			visit(value.Elem())
		case *types.Slice:
			visit(value.Elem())
		case *types.Array:
			visit(value.Elem())
		case *types.Map:
			visit(value.Key())
			visit(value.Elem())
		case *types.Chan:
			visit(value.Elem())
		case *types.Tuple:
			for i := 0; i < value.Len(); i++ {
				visit(value.At(i).Type())
			}
		}
	}
	visit(root)
	keys := sortedKeys(refs)
	result := make([]policyTypeRef, 0, len(keys))
	for _, key := range keys {
		result = append(result, refs[key])
	}
	return result
}

func layerFunctionName(pkg *packages.Package, function *ast.FuncDecl) string {
	if function.Recv == nil || len(function.Recv.List) == 0 {
		return function.Name.Name
	}
	receiver := receiverTypeName(pkg.TypesInfo, function.Recv.List[0])
	if receiver == "" {
		return function.Name.Name
	}
	return receiver + "." + function.Name.Name
}

func matchesPolicyPatterns(patterns []string, value, modulePath string) bool {
	values := []string{normalizePolicyPath(value)}
	modulePath = normalizePolicyPath(modulePath)
	if modulePath != "" {
		if relative, ok := strings.CutPrefix(values[0], modulePath+"/"); ok {
			values = append(values, relative)
		}
	}
	for _, pattern := range patterns {
		expression, err := compilePolicyPattern(pattern)
		if err != nil {
			continue
		}
		for _, candidate := range values {
			if expression.MatchString(candidate) {
				return true
			}
		}
	}
	return false
}

func compilePolicyPattern(pattern string) (*regexp.Regexp, error) {
	pattern = normalizePolicyPath(pattern)
	if pattern == "" {
		return nil, fmt.Errorf("empty pattern")
	}
	if strings.HasSuffix(pattern, "/**") {
		base := strings.TrimSuffix(pattern, "/**")
		return regexp.Compile("^" + globPolicyRegexp(base) + "(?:/.*)?$")
	}
	return regexp.Compile("^" + globPolicyRegexp(pattern) + "$")
}

func globPolicyRegexp(pattern string) string {
	var expression strings.Builder
	for index := 0; index < len(pattern); index++ {
		switch pattern[index] {
		case '*':
			if index+1 < len(pattern) && pattern[index+1] == '*' {
				expression.WriteString(".*")
				index++
			} else {
				expression.WriteString("[^/]*")
			}
		case '?':
			expression.WriteString("[^/]")
		default:
			expression.WriteString(regexp.QuoteMeta(string(pattern[index])))
		}
	}
	return expression.String()
}

func normalizePolicyPath(value string) string {
	return strings.TrimPrefix(strings.ReplaceAll(value, "\\", "/"), "./")
}

func mapValuesSorted[T any](values map[string]T, less func(T, T) bool) []T {
	result := make([]T, 0, len(values))
	for _, value := range values {
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return less(result[i], result[j]) })
	return result
}
