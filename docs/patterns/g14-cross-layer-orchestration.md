# G14 — Cross-Layer Orchestration

## What it is

A function in a configured boundary layer both coordinates several configured
dependency types and maps configured generated boundary types. A common example
is an HTTP, RPC, or generated transport handler that calls multiple application
services or stores while constructing its generated response model.

G14 is an opt-in repository policy, not a universal Go smell. With no
`layer_policy` entries in `.lagotto.yaml`, it produces no findings.

## Why it matters

Mapping wire types belongs at a transport boundary. Coordinating several
services or stores usually belongs in an application service or use-case. When
one method does both, transport changes and workflow changes become coupled and
the same orchestration is harder to reuse outside that adapter.

## Configuration

```yaml
version: 1

layer_policy:
  - name: thin-transport
    paths:
      - internal/transport/**
    dependencies:
      - internal/service/**
      - internal/storage/**
    generated_types:
      - gen/**
    max_coordinated_dependencies: 1
    severity: medium
```

Patterns may be module-relative or full import paths. They support `*` within
one path segment, `**` across path segments, and `?` for one non-separator
character. The defaults are `max_coordinated_dependencies: 1` and
`severity: medium`.

Use `lagotto layers .` to run only configured G14 policies. `lagotto audit .`
includes them in the existing audit pipeline.

## What triggers it

Within a function or method whose file or package matches `paths`, Lagotto
requires both:

1. calls to more distinct configured dependency receiver types or dependency
   package-function groups than `max_coordinated_dependencies`; and
2. mapping of at least one configured generated type.

Repeated calls to methods on the same receiver type count as one dependency.
Package constructors named `New` or `NewX` are treated as wiring and do not
count as another coordinated dependency. A package function and receiver call
from the same package are also treated as one dependency.
Mapping evidence includes generated composite literals, assignments to fields
of generated values, conversions to generated types, and generated results from
local mapper helpers or functions in the generated package.

The evidence records the rule and threshold, each typed dependency call and
line, and each generated-type mapping and line. Import aliases do not affect
the result because G14 uses type-checker objects rather than source spelling.

## What does not trigger it

- coordinating several dependencies without mapping a configured boundary type;
- mapping generated types while calling no more than the permitted dependency
  count;
- repeated calls to a single dependency type;
- files outside the configured paths;
- generated files, test files, and test utility packages; or
- nested function literals, whose work is not attributed to the outer method.

G14 intentionally does not enforce import boundaries. Use depguard or an
equivalent import-policy linter for strict allow/deny rules.

## How to respond

Prefer moving multi-service or multi-store workflow coordination into one
narrow application service or use-case, leaving the boundary method responsible
for request/response mapping and one application call. If the boundary is
intentionally an orchestration layer, adjust the checked-in policy or suppress
the exact stable finding rather than restructuring code only to satisfy the
detector.

## Limitations

G14 detects typed static calls. Dynamic dispatch through a broad interface
still counts by that interface type, and orchestration hidden behind closures,
reflection, or untyped registries may not be visible. Its output is a review
candidate qualified by repository policy, not proof of an architectural defect.
