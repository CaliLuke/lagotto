package detect

import (
	"go/ast"
	"go/token"
	"sort"
)

type cyclomaticHotspot struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Line       int    `json:"line"`
	Complexity int    `json:"complexity"`
}

type cyclomaticSummary struct {
	FunctionCount          int                 `json:"function_count"`
	Total                  int                 `json:"total"`
	Max                    int                 `json:"max"`
	PrioritizationHotspots []cyclomaticHotspot `json:"prioritization_hotspots,omitempty"`
}

// annotateCyclomaticComplexity runs only after cohesion and size have produced
// a candidate file. It records standard McCabe complexity for named functions
// and methods, excluding nested function literals from the outer declaration.
func annotateCyclomaticComplexity(analysis *cohesionAnalysis) cyclomaticSummary {
	var summary cyclomaticSummary
	for componentIndex := range analysis.Components {
		component := &analysis.Components[componentIndex]
		for _, node := range component.nodes {
			function, ok := node.node.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			complexity := cyclomaticComplexity(function.Body)
			component.CyclomaticTotal += complexity
			component.CyclomaticMax = max(component.CyclomaticMax, complexity)
			summary.FunctionCount++
			summary.Total += complexity
			summary.Max = max(summary.Max, complexity)
			summary.PrioritizationHotspots = append(summary.PrioritizationHotspots, cyclomaticHotspot{
				Name:       node.member.Name,
				Kind:       node.member.Kind,
				Line:       node.member.Line,
				Complexity: complexity,
			})
			for memberIndex := range component.Members {
				member := &component.Members[memberIndex]
				if member.Line == node.member.Line && member.Name == node.member.Name {
					member.CyclomaticComplexity = complexity
					break
				}
			}
		}
	}
	sort.Slice(summary.PrioritizationHotspots, func(i, j int) bool {
		if summary.PrioritizationHotspots[i].Complexity != summary.PrioritizationHotspots[j].Complexity {
			return summary.PrioritizationHotspots[i].Complexity > summary.PrioritizationHotspots[j].Complexity
		}
		if summary.PrioritizationHotspots[i].Line != summary.PrioritizationHotspots[j].Line {
			return summary.PrioritizationHotspots[i].Line < summary.PrioritizationHotspots[j].Line
		}
		return summary.PrioritizationHotspots[i].Name < summary.PrioritizationHotspots[j].Name
	})
	const hotspotLimit = 5
	if len(summary.PrioritizationHotspots) > hotspotLimit {
		summary.PrioritizationHotspots = summary.PrioritizationHotspots[:hotspotLimit]
	}
	return summary
}

func cyclomaticComplexity(body *ast.BlockStmt) int {
	if body == nil {
		return 0
	}
	complexity := 1
	ast.Inspect(body, func(node ast.Node) bool {
		switch current := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt:
			complexity++
		case *ast.CaseClause:
			if current.List != nil {
				complexity++
			}
		case *ast.CommClause:
			if current.Comm != nil {
				complexity++
			}
		case *ast.BinaryExpr:
			if current.Op == token.LAND || current.Op == token.LOR {
				complexity++
			}
		}
		return true
	})
	return complexity
}
