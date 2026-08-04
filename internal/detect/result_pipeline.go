package detect

import (
	"fmt"
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/packages"

	"github.com/CaliLuke/lagotto/internal/audit"
)

// ScanMaterializedResultPipelines finds functions that accept a fully
// materialized []map[string]any result set and build a second, typed slice by
// transforming and appending every row. The raw collection and typed
// collection coexist, so peak memory grows with the complete result set.
//
// This is a LOW performance-review signal. Materialization may be intentional,
// and only a benchmark can establish whether a streaming producer boundary is
// worth its complexity.
func ScanMaterializedResultPipelines(pkgs []*packages.Package) []audit.Finding {
	var findings []audit.Finding
	for _, pkg := range pkgs {
		for fileIndex, file := range pkg.Syntax {
			filename := syntaxFilename(pkg, fileIndex, file)
			if skipSourceFile(filename, file) {
				continue
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil || !hasTypedSliceResult(pkg, fn) {
					continue
				}
				for _, parameter := range rawResultParameters(pkg, fn) {
					row, transformed, ok := transformsRawRows(fn.Body, parameter)
					if !ok {
						continue
					}
					findings = append(findings, audit.Finding{
						Smell:    "Materialized Result Pipeline",
						SmellID:  "G15",
						Severity: audit.SevLow,
						Location: sourceLocation(pkg, filename) + ":" + fn.Name.Name,
						Message:  fmt.Sprintf("%s transforms fully materialized %q rows into a second result slice, so both collections coexist.", fn.Name.Name, parameter),
						Evidence: map[string]any{
							"function":          fn.Name.Name,
							"raw_parameter":     parameter,
							"row_variable":      row,
							"transformed_value": transformed,
						},
						Suggestion: "Benchmark large, frequent reads and inspect peak memory. If materialization is significant, move the boundary before this function: decode or produce one raw row at a time, hydrate it directly into the destination slice, then discard the raw row. Keep scalar setters unless profiling identifies them separately.",
					})
					break
				}
			}
		}
	}
	return findings
}

func rawResultParameters(pkg *packages.Package, fn *ast.FuncDecl) []string {
	var parameters []string
	if pkg.TypesInfo == nil || fn.Type.Params == nil {
		return parameters
	}
	for _, field := range fn.Type.Params.List {
		for _, name := range field.Names {
			obj, ok := pkg.TypesInfo.Defs[name].(*types.Var)
			if ok && isRawResultSlice(obj.Type()) {
				parameters = append(parameters, name.Name)
			}
		}
	}
	return parameters
}

func isRawResultSlice(t types.Type) bool {
	slice, ok := types.Unalias(t).Underlying().(*types.Slice)
	if !ok {
		return false
	}
	row, ok := types.Unalias(slice.Elem()).Underlying().(*types.Map)
	if !ok {
		return false
	}
	key, ok := types.Unalias(row.Key()).Underlying().(*types.Basic)
	if !ok || key.Kind() != types.String {
		return false
	}
	_, ok = types.Unalias(row.Elem()).Underlying().(*types.Interface)
	return ok
}

func hasTypedSliceResult(pkg *packages.Package, fn *ast.FuncDecl) bool {
	if pkg.TypesInfo == nil {
		return false
	}
	object, ok := pkg.TypesInfo.Defs[fn.Name].(*types.Func)
	if !ok {
		return false
	}
	signature, ok := object.Type().(*types.Signature)
	if !ok {
		return false
	}
	for i := range signature.Results().Len() {
		result := signature.Results().At(i).Type()
		if _, ok := types.Unalias(result).Underlying().(*types.Slice); ok && !isRawResultSlice(result) {
			return true
		}
	}
	return false
}

func transformsRawRows(body *ast.BlockStmt, parameter string) (row, transformed string, found bool) {
	for _, statement := range body.List {
		rangeStatement, ok := statement.(*ast.RangeStmt)
		if !ok || !identNamed(rangeStatement.X, parameter) || rangeStatement.Body == nil {
			continue
		}
		rowIdent, ok := rangeStatement.Value.(*ast.Ident)
		if !ok || rowIdent.Name == "_" {
			continue
		}
		if value, ok := appendedRowTransformation(rangeStatement.Body, rowIdent.Name); ok {
			return rowIdent.Name, value, true
		}
	}
	return "", "", false
}

func appendedRowTransformation(body *ast.BlockStmt, row string) (string, bool) {
	transformed := map[string]bool{}
	for _, statement := range body.List {
		assign, ok := statement.(*ast.AssignStmt)
		if !ok {
			continue
		}
		for rhsIndex, rhs := range assign.Rhs {
			if !isTransformationCall(rhs, row) {
				continue
			}
			lhsIndex := min(rhsIndex, len(assign.Lhs)-1)
			if lhsIndex >= 0 {
				if ident, ok := assign.Lhs[lhsIndex].(*ast.Ident); ok {
					transformed[ident.Name] = true
				}
			}
		}
	}
	if len(transformed) == 0 {
		return "", false
	}
	var appended string
	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) < 2 || !identNamed(call.Fun, "append") {
			return true
		}
		for _, argument := range call.Args[1:] {
			if ident, ok := argument.(*ast.Ident); ok && transformed[ident.Name] {
				appended = ident.Name
				return false
			}
		}
		return true
	})
	return appended, appended != ""
}

func isTransformationCall(expr ast.Expr, row string) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	for _, argument := range call.Args {
		if identNamed(argument, row) {
			return true
		}
	}
	return false
}

func identNamed(expr ast.Expr, name string) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == name
}
