# G13 — Large Cohesive File

A non-generated Go file is at least 1200 lines, but its declaration/reference
graph contains at most one substantial component. The file is substantially
cohesive, though it may contain tiny disconnected declarations. It is large and
may be costly to navigate, yet Lagotto has no structural evidence that it mixes
responsibilities.

## Why this is separate from G5

Large connected transport adapters, parsers, and orchestration files can merit
an architecture review even when their declarations belong together. Treating
size as disconnected concerns recreates G5's old false positives, so G13 is a
separate **LOW** signal.

G13 does not report dot-imported DSL registration files, where generated-style
declarative breadth is expected. It also yields to G5 when the file contains
two substantial disconnected components.

The message reports the actual substantial-component count (zero or one).
Evidence reports zero-primary graph artifacts as `ignored_component_count`, so
the substantial, minor, and ignored categories account for the raw
`component_count`. `complexity_rejected_component_count` identifies the subset
of minor components rejected by the post-candidate complexity gate; it is not a
fourth disjoint category.

Because G13 is already a size/cohesion candidate, Lagotto also reports its
total and maximum cyclomatic complexity and the five highest-complexity named
functions under `prioritization_hotspots`. Complexity helps reviewers locate
hotspots; it does not turn this LOW navigation signal into a correctness claim
or create a finding by itself.

## Configuration

The default floor is 1200 physical lines:

```yaml
version: 1
mixed:
  cohesive_min_lines: 1200
```

Use `lagotto mixed --cohesive-min-lines=N` for a one-off threshold, or set the
value to `0` to disable G13.

## How to respond

Do not split a file based on size alone. Review whether named phases or
layer-specific sections change independently and whether navigation is costly.
If a file-only split helps, move cohesive implementation sections to
content-named files in the same package while preserving the public API.
