package detect

import (
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

type cohesionMember struct {
	Name                 string `json:"name"`
	Kind                 string `json:"kind"`
	Line                 int    `json:"line"`
	EndLine              int    `json:"end_line"`
	LineCount            int    `json:"line_count"`
	CyclomaticComplexity int    `json:"cyclomatic_complexity,omitempty"`
}

type cohesionComponent struct {
	Members         []cohesionMember `json:"members"`
	MemberCount     int              `json:"member_count"`
	PrimaryCount    int              `json:"primary_member_count"`
	LineCount       int              `json:"line_count"`
	CyclomaticTotal int              `json:"cyclomatic_total,omitempty"`
	CyclomaticMax   int              `json:"cyclomatic_max,omitempty"`
	nodes           []cohesionNode
}

type cohesionAnalysis struct {
	LineCount  int
	Components []cohesionComponent
}

type cohesionNode struct {
	member   cohesionMember
	node     ast.Node
	primary  bool
	receiver *types.Named
}

// analyzeFileCohesion builds an intra-file graph whose nodes are top-level
// declarations. Edges come from direct identifier references, shared
// package-level objects, receiver ownership, and implicit satisfaction of a
// named interface. The last edge is essential in Go: interface families are
// cohesive even when concrete implementations never name the interface.
func analyzeFileCohesion(pkg *packages.Package, file *ast.File) cohesionAnalysis {
	analysis := cohesionAnalysis{}
	if pkg == nil || file == nil {
		return analysis
	}
	analysis.LineCount = physicalLineCount(pkg.Fset, file)
	if pkg.Fset == nil || pkg.Types == nil || pkg.TypesInfo == nil {
		return analysis
	}

	nodes := collectCohesionNodes(pkg, file)
	if len(nodes) == 0 {
		return analysis
	}

	adjacent := make([]map[int]bool, len(nodes))
	for i := range adjacent {
		adjacent[i] = map[int]bool{}
	}
	connect := func(a, b int) {
		if a == b {
			return
		}
		adjacent[a][b] = true
		adjacent[b][a] = true
	}

	definitionNode := map[types.Object]int{}
	for i, node := range nodes {
		for _, obj := range declaredObjects(pkg.TypesInfo, node.node) {
			definitionNode[obj] = i
		}
	}

	// Direct references connect callers, constructors, result types, methods,
	// and the package-level declarations they use. Multiple declarations using
	// the same package object declared in another file are connected too.
	cohesionObjectUsers := map[types.Object][]int{}
	dotImports := dotImportPaths(file)
	for i, node := range nodes {
		seen := map[types.Object]bool{}
		ast.Inspect(node.node, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			obj := pkg.TypesInfo.Uses[id]
			if obj == nil || seen[obj] {
				return true
			}
			seen[obj] = true
			if target, ok := definitionNode[obj]; ok {
				connect(i, target)
			}
			if obj.Pkg() == pkg.Types && obj.Parent() == pkg.Types.Scope() {
				cohesionObjectUsers[obj] = append(cohesionObjectUsers[obj], i)
			} else if obj.Pkg() != nil && dotImports[obj.Pkg().Path()] {
				// Dot imports are primarily used by declarative DSLs. Multiple
				// declarations invoking the same unqualified DSL constructor are
				// one registration family even when they do not reference each
				// other directly.
				cohesionObjectUsers[obj] = append(cohesionObjectUsers[obj], i)
			}
			return true
		})
	}
	for _, users := range cohesionObjectUsers {
		connectAll(users, connect)
	}

	// Every method belongs to its receiver even when the type declaration is
	// in another file. Grouping receiver nodes also supplies the concrete types
	// used by the implicit-interface-family pass below.
	typeNodes := map[*types.Named][]int{}
	for i, node := range nodes {
		if node.receiver != nil {
			typeNodes[node.receiver] = append(typeNodes[node.receiver], i)
		}
		if spec, ok := node.node.(*ast.TypeSpec); ok {
			if typeName, ok := pkg.TypesInfo.Defs[spec.Name].(*types.TypeName); ok {
				if named := asNamed(typeName.Type()); named != nil {
					typeNodes[named] = append(typeNodes[named], i)
				}
			}
		}
	}
	for _, members := range typeNodes {
		connectAll(members, connect)
	}

	for _, iface := range namedMethodSetInterfaces(pkg, file) {
		var implementations []int
		for named, members := range typeNodes {
			if types.Implements(named, iface) || types.Implements(types.NewPointer(named), iface) {
				implementations = append(implementations, members[0])
			}
		}
		if len(implementations) >= 2 {
			connectAll(implementations, connect)
		}
	}

	analysis.Components = graphComponents(nodes, adjacent)
	return analysis
}

func dotImportPaths(file *ast.File) map[string]bool {
	paths := map[string]bool{}
	for _, spec := range file.Imports {
		if spec.Name == nil || spec.Name.Name != "." {
			continue
		}
		path, err := strconv.Unquote(spec.Path.Value)
		if err == nil {
			paths[path] = true
		}
	}
	return paths
}

func collectCohesionNodes(pkg *packages.Package, file *ast.File) []cohesionNode {
	var nodes []cohesionNode
	add := func(node ast.Node, name, kind string, primary bool, receiver *types.Named) {
		start := pkg.Fset.Position(node.Pos()).Line
		end := pkg.Fset.Position(node.End()).Line
		nodes = append(nodes, cohesionNode{
			member: cohesionMember{
				Name:      name,
				Kind:      kind,
				Line:      start,
				EndLine:   end,
				LineCount: max(1, end-start+1),
			},
			node: node, primary: primary, receiver: receiver,
		})
	}

	for _, declaration := range file.Decls {
		switch decl := declaration.(type) {
		case *ast.GenDecl:
			for _, rawSpec := range decl.Specs {
				switch spec := rawSpec.(type) {
				case *ast.TypeSpec:
					add(spec, spec.Name.Name, "type", true, nil)
				case *ast.ValueSpec:
					var names []string
					for _, name := range spec.Names {
						names = append(names, name.Name)
					}
					kind := strings.ToLower(decl.Tok.String())
					add(spec, strings.Join(names, ", "), kind, false, nil)
				}
			}
		case *ast.FuncDecl:
			name, kind := decl.Name.Name, "function"
			var receiver *types.Named
			if method, ok := pkg.TypesInfo.Defs[decl.Name].(*types.Func); ok {
				if signature, ok := method.Type().(*types.Signature); ok && signature.Recv() != nil {
					receiver = asNamed(signature.Recv().Type())
				}
			}
			if receiver != nil {
				name = receiver.Obj().Name() + "." + name
				kind = "method"
			}
			add(decl, name, kind, true, receiver)
		}
	}
	return nodes
}

func declaredObjects(info *types.Info, node ast.Node) []types.Object {
	var objects []types.Object
	switch declaration := node.(type) {
	case *ast.TypeSpec:
		if obj := info.Defs[declaration.Name]; obj != nil {
			objects = append(objects, obj)
		}
	case *ast.ValueSpec:
		for _, name := range declaration.Names {
			if obj := info.Defs[name]; obj != nil {
				objects = append(objects, obj)
			}
		}
	case *ast.FuncDecl:
		if obj := info.Defs[declaration.Name]; obj != nil {
			objects = append(objects, obj)
		}
	}
	return objects
}

func asNamed(t types.Type) *types.Named {
	if t == nil {
		return nil
	}
	t = types.Unalias(t)
	if pointer, ok := t.(*types.Pointer); ok {
		t = types.Unalias(pointer.Elem())
	}
	named, _ := t.(*types.Named)
	return named
}

func namedMethodSetInterfaces(pkg *packages.Package, file *ast.File) []*types.Interface {
	seen := map[*types.Interface]bool{}
	var interfaces []*types.Interface
	add := func(t types.Type) {
		t = types.Unalias(t)
		if named, ok := t.(*types.Named); ok {
			t = named.Underlying()
		}
		iface, ok := t.(*types.Interface)
		if !ok {
			return
		}
		iface.Complete()
		if iface.NumMethods() == 0 || !iface.IsMethodSet() || seen[iface] {
			return
		}
		seen[iface] = true
		interfaces = append(interfaces, iface)
	}
	for _, name := range pkg.Types.Scope().Names() {
		if typeName, ok := pkg.Types.Scope().Lookup(name).(*types.TypeName); ok {
			add(typeName.Type())
		}
	}
	ast.Inspect(file, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			if typeName, ok := pkg.TypesInfo.Uses[id].(*types.TypeName); ok {
				add(typeName.Type())
			}
		}
		return true
	})
	return interfaces
}

func connectAll(indices []int, connect func(int, int)) {
	if len(indices) < 2 {
		return
	}
	first := indices[0]
	for _, index := range indices[1:] {
		connect(first, index)
	}
}

func graphComponents(nodes []cohesionNode, adjacent []map[int]bool) []cohesionComponent {
	visited := make([]bool, len(nodes))
	var components []cohesionComponent
	for start := range nodes {
		if visited[start] {
			continue
		}
		visited[start] = true
		stack := []int{start}
		component := cohesionComponent{}
		for len(stack) > 0 {
			index := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			node := nodes[index]
			component.Members = append(component.Members, node.member)
			component.nodes = append(component.nodes, node)
			component.LineCount += node.member.LineCount
			if node.primary {
				component.PrimaryCount++
			}
			for neighbor := range adjacent[index] {
				if !visited[neighbor] {
					visited[neighbor] = true
					stack = append(stack, neighbor)
				}
			}
		}
		sort.Slice(component.Members, func(i, j int) bool {
			if component.Members[i].Line != component.Members[j].Line {
				return component.Members[i].Line < component.Members[j].Line
			}
			return component.Members[i].Name < component.Members[j].Name
		})
		component.MemberCount = len(component.Members)
		components = append(components, component)
	}
	sort.Slice(components, func(i, j int) bool {
		return components[i].Members[0].Line < components[j].Members[0].Line
	})
	return components
}

func physicalLineCount(fset *token.FileSet, file *ast.File) int {
	if fset == nil || file == nil {
		return 0
	}
	if tokenFile := fset.File(file.Pos()); tokenFile != nil {
		return tokenFile.LineCount()
	}
	start := fset.Position(file.Pos()).Line
	end := fset.Position(file.End()).Line
	return max(0, end-start+1)
}
