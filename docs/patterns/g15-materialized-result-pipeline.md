# G15 — Materialized Result Pipeline

A function accepts a complete `[]map[string]any` result set, transforms every
row, and appends the transformed values into a second typed slice. The raw and
typed collections coexist, so peak memory grows with both representations.

## Why this matters

This shape often appears at decoding, FFI, ORM, and hydration boundaries. A
producer first materializes generic rows, then a consumer walks them again to
construct the values the caller actually wanted. For large reads, retaining
both collections can dominate the cost even when per-field conversion is
already efficient.

G15 is a LOW performance-review signal. It does not claim materialization is
wrong or slow; benchmark representative large reads before changing the API.

## What lagotto checks

The detector reports a function or method only when it has all of these
properties:

- it accepts a `[]map[string]any` parameter;
- it returns a different slice type;
- it ranges over the raw parameter;
- it transforms each row with a function call; and
- it appends the transformed value to a result slice.

Test and generated files are excluded. Raw-to-raw normalization and
row-at-a-time producer APIs do not fire.

## Example

```go
func hydrateResults(rows []map[string]any) ([]*Model, error) {
    models := make([]*Model, 0, len(rows))
    for _, row := range rows {
        model, err := hydrate(row)
        if err != nil { return nil, err }
        models = append(models, model)
    }
    return models, nil
}
```

## How to investigate

Benchmark peak memory and throughput for large, frequent reads. If the raw
collection is material, move the boundary before the reported function:
decode or produce one row, hydrate it directly into the destination slice,
then discard the raw row. If an upstream FFI layer also collects every row,
streaming must begin there to remove the full pipeline.

Do not rewrite scalar setters merely because this detector fired. Profile
field-level hydration separately; the finding is specifically about the
whole-result intermediate representation.
