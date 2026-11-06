package detect

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"sort"

	"golang.org/x/tools/go/packages"

	"github.com/CaliLuke/lagotto/internal/audit"
	"github.com/CaliLuke/lagotto/internal/pkgload"
)

type fileReference struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Symbol string `json:"symbol"`
}

type packageConcern struct {
	Prefix    string          `json:"prefix"`
	Files     []string        `json:"files"`
	LineCount int             `json:"line_count"`
	Inbound   []fileReference `json:"inbound"`
	Outbound  []fileReference `json:"outbound"`
}

type packageFileGraph struct {
	lines map[string]int
	edges map[fileReference]bool
}

// ScanPackageConcerns finds substantial prefix groups with narrow, acyclic
// boundaries. A review may still need to move shared declarations into a core
// package; this signal does not promise that moving files unchanged compiles.
func ScanPackageConcerns(pkgs []*packages.Package) []audit.Finding {
	var findings []audit.Finding
	for _, pkg := range pkgs {
		if pkg == nil || pkg.Name == "main" || isTestPackage(pkg.PkgPath) || pkg.TypesInfo == nil || pkg.Types == nil || pkg.Fset == nil {
			continue
		}
		graph := buildPackageFileGraph(pkg)
		var groups []packageConcern
		for _, cluster := range collectPrefixClusters(sortedKeys(graph.lines)) {
			if group, ok := graph.concern(cluster); ok {
				groups = append(groups, group)
			}
		}
		if len(groups) == 0 {
			continue
		}
		findings = append(findings, audit.Finding{
			Smell: "Disconnected Package Concerns", SmellID: "G16", Severity: audit.SevLow,
			Location:   packageLocation(pkg),
			Message:    fmt.Sprintf("%d substantial file group(s) have narrow, acyclic dependency boundaries.", len(groups)),
			Evidence:   map[string]any{"package": pkg.PkgPath, "groups": groups},
			Suggestion: "Review these groups as possible subpackages. Keep receiver and interface families together; move shared core declarations to a neutral package if needed, and export or invert the remaining boundary before moving files.",
		})
	}
	return findings
}

func buildPackageFileGraph(pkg *packages.Package) packageFileGraph {
	graph := packageFileGraph{lines: map[string]int{}, edges: map[fileReference]bool{}}
	// Retain all compiled source in the dependency graph, including generated
	// code. Only authored production files can nominate a candidate.
	allFiles := map[string]bool{}
	for i, file := range pkg.Syntax {
		filename := syntaxFilename(pkg, i, file)
		base := filepath.Base(filename)
		allFiles[base] = true
		if !skipSourceFile(filename, file) {
			graph.lines[base] = physicalLineCount(pkg.Fset, file)
		}
	}
	fileAt := func(pos token.Pos) string { return filepath.Base(pkg.Fset.PositionFor(pos, false).Filename) }
	add := func(from, to, symbol string) {
		if from != to && allFiles[from] && allFiles[to] {
			graph.edges[fileReference{From: from, To: to, Symbol: symbol}] = true
		}
	}
	for id, obj := range pkg.TypesInfo.Uses {
		if obj.Pkg() != pkg.Types {
			continue
		}
		// Package objects, fields and methods all carry file ownership. Local
		// variables remain in their own file and therefore add no edge.
		add(fileAt(id.Pos()), fileAt(obj.Pos()), obj.Name())
	}
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || len(fn.Recv.List) == 0 {
				continue
			}
			named := asNamed(pkg.TypesInfo.TypeOf(fn.Recv.List[0].Type))
			if named == nil {
				continue
			}
			from, to := fileAt(fn.Pos()), fileAt(named.Obj().Pos())
			// A receiver and its methods cannot be defined in different packages.
			add(from, to, named.Obj().Name())
			add(to, from, fn.Name.Name)
		}
	}
	connectPackageInterfaceFamilies(pkg, fileAt, add)
	return graph
}

func connectPackageInterfaceFamilies(pkg *packages.Package, fileAt func(token.Pos) string, add func(string, string, string)) {
	var namedTypes []*types.Named
	for _, name := range pkg.Types.Scope().Names() {
		obj, ok := pkg.Types.Scope().Lookup(name).(*types.TypeName)
		if !ok || obj.IsAlias() {
			continue
		}
		if named, ok := obj.Type().(*types.Named); ok {
			namedTypes = append(namedTypes, named)
		}
	}
	for _, named := range namedTypes {
		iface, ok := named.Underlying().(*types.Interface)
		if !ok || !iface.IsMethodSet() || iface.NumMethods() == 0 {
			continue
		}
		for _, impl := range namedTypes {
			if _, isInterface := impl.Underlying().(*types.Interface); isInterface {
				continue
			}
			if !types.Implements(impl, iface) && !types.Implements(types.NewPointer(impl), iface) {
				continue
			}
			from, to := fileAt(named.Obj().Pos()), fileAt(impl.Obj().Pos())
			add(from, to, impl.Obj().Name())
			add(to, from, named.Obj().Name())
		}
	}
}

func (g packageFileGraph) concern(cluster prefixCluster) (packageConcern, bool) {
	group := packageConcern{Prefix: cluster.Prefix, Files: cluster.Files, Inbound: []fileReference{}, Outbound: []fileReference{}}
	members := map[string]bool{}
	for _, file := range cluster.Files {
		members[file] = true
		group.LineCount += g.lines[file]
	}
	if group.LineCount < 800 || len(members) == len(g.lines) {
		return group, false
	}
	callers := map[string]bool{}
	for edge := range g.edges {
		if !members[edge.From] && members[edge.To] {
			group.Inbound = append(group.Inbound, edge)
			callers[edge.From] = true
		}
		if members[edge.From] && !members[edge.To] {
			group.Outbound = append(group.Outbound, edge)
		}
	}
	if len(callers) > 1 {
		return group, false
	}
	// Walk outbound dependencies transitively: a core -> dispatcher -> group
	// path is as cyclic as a direct core -> group edge.
	adjacent := map[string][]string{}
	for edge := range g.edges {
		adjacent[edge.From] = append(adjacent[edge.From], edge.To)
	}
	seen := map[string]bool{}
	var pending []string
	for _, edge := range group.Outbound {
		pending = append(pending, edge.To)
	}
	for len(pending) > 0 {
		file := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if members[file] {
			return group, false
		}
		if seen[file] {
			continue
		}
		seen[file] = true
		pending = append(pending, adjacent[file]...)
	}
	sortFileReferences(group.Inbound)
	sortFileReferences(group.Outbound)
	return group, true
}

func sortFileReferences(refs []fileReference) {
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].From != refs[j].From {
			return refs[i].From < refs[j].From
		}
		if refs[i].To != refs[j].To {
			return refs[i].To < refs[j].To
		}
		return refs[i].Symbol < refs[j].Symbol
	})
}

// removeCoveredPrefixClusters preserves the filesystem report for prefixes
// without structural evidence. Convert package locations to audit-root-relative
// locations because a scan may start below the module root.
func removeCoveredPrefixClusters(root string, findings []audit.Finding) []audit.Finding {
	covered := map[string]map[string]bool{}
	moduleRoot := pkgload.ModuleRoot(root)
	for _, finding := range findings {
		if finding.SmellID != "G16" {
			continue
		}
		location := filesystemLocation(root, filepath.Join(moduleRoot, filepath.FromSlash(finding.Location)))
		covered[location] = map[string]bool{}
		for _, group := range finding.Evidence["groups"].([]packageConcern) {
			covered[location][group.Prefix] = true
		}
	}
	var result []audit.Finding
	for _, finding := range findings {
		if finding.SmellID != "G9" || len(covered[finding.Location]) == 0 {
			result = append(result, finding)
			continue
		}
		var remaining []prefixCluster
		for _, cluster := range finding.Evidence["clusters"].([]prefixCluster) {
			if !covered[finding.Location][cluster.Prefix] {
				remaining = append(remaining, cluster)
			}
		}
		result = append(result, prefixClusterReport(finding.Location, remaining)...)
	}
	return result
}
