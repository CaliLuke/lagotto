package detect

import (
	"testing"

	"github.com/CaliLuke/lagotto/internal/audit"
)

func TestG15_MaterializedRawRowsBecomeTypedSlice(t *testing.T) {
	pkgs := fakeModule(t, map[string]string{
		"hydrate.go": `package result

type Model struct{ Name string }
func hydrate(row map[string]any) (*Model, error) { return &Model{}, nil }

func hydrateResults(results []map[string]any) ([]*Model, error) {
	models := make([]*Model, 0, len(results))
	for _, row := range results {
		model, err := hydrate(row)
		if err != nil { return nil, err }
		models = append(models, model)
	}
	return models, nil
}
`,
	})
	findings := ScanMaterializedResultPipelines(pkgs)
	if len(findings) != 1 || findings[0].SmellID != "G15" || findings[0].Severity != audit.SevLow {
		t.Fatalf("expected one LOW G15, got %+v", findings)
	}
	if findings[0].Evidence["raw_parameter"] != "results" || findings[0].Evidence["transformed_value"] != "model" {
		t.Fatalf("unexpected G15 evidence: %+v", findings[0])
	}
}

func TestG15_RowProducerHydratesOneAtATime_NoFire(t *testing.T) {
	pkgs := fakeModule(t, map[string]string{
		"hydrate.go": `package result

type Model struct{ Name string }
type Rows interface { Each(func(map[string]any) error) error }
func collect(rows Rows) ([]*Model, error) {
	var models []*Model
	err := rows.Each(func(row map[string]any) error {
		models = append(models, &Model{Name: row["name"].(string)})
		return nil
	})
	return models, err
}
`,
	})
	if findings := ScanMaterializedResultPipelines(pkgs); containsID(findings, "G15") {
		t.Fatalf("did not expect G15 for row-at-a-time production, got %+v", findings)
	}
}

func TestG15_RawToRawNormalization_NoFire(t *testing.T) {
	pkgs := fakeModule(t, map[string]string{
		"normalize.go": `package result

func normalize(results []map[string]any) []map[string]any {
	normalized := make([]map[string]any, 0, len(results))
	for _, row := range results {
		normalized = append(normalized, clean(row))
	}
	return normalized
}
func clean(row map[string]any) map[string]any { return row }
`,
	})
	if findings := ScanMaterializedResultPipelines(pkgs); containsID(findings, "G15") {
		t.Fatalf("did not expect G15 when the result remains raw rows, got %+v", findings)
	}
}
