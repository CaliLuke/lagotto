---
name: go-layout-smells
description: Audit Go package and file layout for structural smells (Receiver Monolith, Aggregate Holder, Hidden Holder, Facade Method, Mixed-Concern File, God Dependency Bag, Stutter Names, etc.) using the lagotto tool. Use this skill when the user asks to audit Go code organization, review package structure, decompose a god type, run lagotto, check for layout smells, or evaluate whether a Go refactor genuinely split a large receiver type.
---

# Go Layout Smells

Audit Go file organization for structural problems caused by the
language's rules: methods must be defined in the package that owns
the receiver type; package = directory; build tags partition files;
internal/ enforces visibility. The polyglot layout-smells skill misses
these because it reasons in filesystem patterns alone, but in Go,
**filesystem layout is a consequence of type design**.

The core principle: **a Go directory's layout cannot be cleaner than
its type design admits.** When one type has dozens of methods, no
amount of file-moving fixes the navigation problem. The type itself
must be decomposed.

A Go layout audit that fails to identify a Receiver Monolith when
one exists is a failed audit, regardless of how many filename
smells it caught.

## Guiding Principles for Remediation

When proposing fixes, default to the cleaner end state, not the
easier migration path:

1. **No facades, no shims, no compatibility wrappers.** When moving
   implementation into a subpackage, _delete_ the method on the
   original type. A pass-through (`return sub.Foo(t, ...)`) doubles
   the API and never gets removed.

2. **Update all callers in the same refactor.** A subpackage
   extraction that doesn't touch its callers is incomplete. The
   blast radius equals the type's usage surface — that is the work,
   not a reason to defer it.

3. **Retire old code in the same change.** If a refactor makes a
   method, file, struct, or wrapper unnecessary, delete it now —
   unless the user explicitly authorizes a transition window with a
   deletion deadline.

4. **One canonical implementation per concern.** Recommendations
   that produce two ways to do the same thing are wrong by
   construction.

5. **Prefer breaking changes over indirection.** Callers updating
   to a new import path is a one-time cost. A permanent facade is
   recurring cost paid by every reader.

Apply these to every "Remediation:" line. Audit consumers expect
direct migration plans, not phased compatibility roadmaps.

## Executing the Remediations

This skill diagnoses _what_ must change at the type/package level. To
execute the changes safely, defer to the `fowler-refactoring` skill
when present (check for `skills/fowler-refactoring/`) — it provides
the step-by-step mechanics (Move Function, Extract Class, Inline
Function, Rename Field) that preserve behavior while restructuring.

When writing the Remediation block of a finding, name the Fowler
refactoring(s) that apply (e.g., "Extract Class to create
`*nodes.Mutator`, then Move Method for each `CreateNodes` /
`EditNode` / ... onto it; finally Inline Function on the now-empty
`*TypeDB` methods to remove them"). When `fowler-refactoring` is
not present, fall back to plain prose remediation.

## Smell Catalog (quick reference)

| #   | Smell                         | One-line test                                                                          |
| --- | ----------------------------- | -------------------------------------------------------------------------------------- |
| G1  | **Receiver Monolith**         | One named type's effective method set (incl. promoted) is ≥15 across ≥3 concerns       |
| G1B | **Decomposition Theatre**     | 3+ type aliases in one package all resolving to a single underlying struct             |
| G1C | **Aggregate Holder**          | A struct with 5+ same-package sub-service fields whose pointee method count totals ≥25 |
| G1D | **Hidden Holder**             | Thin holder + ≥3 pointer-keyed registry maps + ≥5 exported `*Holder` accessors         |
| G1E | **Foreign Holder**            | A decomposed holder remains in production signatures in downstream packages            |
| G2  | **Stutter Names**             | Exported type/function repeats the package name (`lanes.LaneConfig`)                   |
| G3  | **Build-Tag Pair Sprawl**     | >2 paired files conditioned by build tags (`*_stub.go` / `*_cgo.go`) in one dir        |
| G4  | **God Dependency Bag**        | A dependency bag has ≥8 fields drawn from ≥5 distinct external packages                |
| G5  | **Mixed-Concern File**        | A single file holds 3+ unrelated decl groups (types + validation + utilities)          |
| G6  | **Facade Method**             | Any method whose body is a thin pass-through (≤3 lines) to a subpackage function       |
| G7  | **Init Coupling**             | Multiple `func init()` in a package with cross-file ordering dependencies              |
| G8  | **Internal Re-Export Tunnel** | ≥50% of exports tunnel through a dominant deeper package                              |
| G9  | **Prefix Cluster**            | 3+ files share a name prefix in a flat directory                                       |
| G10 | **Shadow Suffix**             | File names ending in `_helpers`, `_utils`, `_handlers`, `_actions`, `_responses`       |
| G11 | **Junk Drawer**               | File named `helpers.go` / `utils.go` / `common.go` / `misc.go` with mixed contents     |
| G12 | **Premature Package**         | A directory containing only 1 source file (excluding tests, doc, generated)            |

## Why Receiver Monolith Is The Most Important Smell

In TypeScript or Python, splitting a class across files is possible
via mixins, partial classes, or composition without moving code
between modules. In Go, a method can only be defined in the package
where its receiver type is defined. So if `*TypeDB` has 50 methods,
all 50 files holding those methods must live in the same directory.
No file rename, no subpackage promotion, no convention can change
that.

The receiver IS the layout boundary in Go. When a layout problem
hits this wall, the cure is type-level decomposition — splitting the
god struct into smaller types each living in its own subpackage —
not file-level reshuffling.

## Anti-Patterns: Faked Receiver Decompositions

Once an agent knows lagotto flags Receiver Monolith, the cheapest
move is to rearrange names until the detector goes quiet. Five
disguises recur — alias clusters (G1B), aggregate holders (G1C),
embedding theatre, registry-keyed hidden holders (G1D), and
reach-through holders (G1E). All preserve the god type while signalling
decomposition where there is none.

For full detection criteria, code examples, and fixes, see
`references/anti-patterns.md`. G1E automates the caller-view signature check;
the manual grep remains useful as a verification backstop.

## Spirit, Not Letter

Any specific structural metric, once written into a spec or
detector, becomes the thing the system optimizes for — usually by
routing around it rather than satisfying intent. The systemic answer
is to specify the _target end state_ in caller-view terms ("no
production caller takes `*God`"), not structural counts ("`*God` has
zero same-package pointer fields"). Caller-view specs leave no room
for evasion.

For the full pattern, the spec-writing template, and what to do when
finding the next disguise, see `references/spirit-not-letter.md`.

## Verifying A True Decomposition

When reviewing whether a refactor genuinely decomposed a god type,
run the seven-point checklist in `references/verification-checklist.md`.
The first check (the god type does not appear in production caller
signatures) is the spec — the others are diagnostic confirmations.
**A different agent must run this checklist than the one that
implemented the refactor.**

## Workflow

Detection is performed by **lagotto**, a Go AST/types-based audit
tool that ships with this skill. It uses `golang.org/x/tools/go/packages`
to load the workspace, then runs each smell detector against the
loaded type graph — accurate against generics, embedded types,
build tags, and multi-line declarations that regex would miss.

### Step 1 — Run lagotto

Install once:

```bash
# Homebrew (preferred):
brew install caliluke/tap/lagotto

# Or from source:
go install github.com/CaliLuke/lagotto/cmd/lagotto@latest
```

Run the full audit:

```bash
lagotto audit \
  --tags=cgo,typedb,typedb_prebuilt \
  --format=json \
  /path/to/repo/internal > findings.json
```

Replace the `--tags` value with whatever build tags the target
codebase needs to compile (use the same value the project's test
gate uses). For an audit without build tags, omit the flag.

Subcommands:

- `lagotto audit` — runs every detector
- `lagotto monoliths` — Receiver Monolith (G1)
- `lagotto stutter` — Stutter Names (G2)
- `lagotto facades` — Facade Method (G6)
- `lagotto deps` — God Dependency Bag (G4)
- `lagotto mixed` — Mixed-Concern File (G5)
- `lagotto inits` — Init Coupling (G7)
- `lagotto tunnel` — Internal Re-Export Tunnel (G8)
- `lagotto fs` — filesystem smells (G3, G9, G10, G11, G12)

For human reading, swap `--format=json` for `--format=text`.

### Step 2 — Interpret findings

Each finding has `smell`, `smell_id`, `severity`, `location`,
`message`, `evidence`, and `suggestion`. Findings are pre-sorted by
severity (CRITICAL → LOW). Triage in that order.

For Receiver Monoliths (G1), inspect `evidence.methods` and
`evidence.concerns` to confirm the type genuinely spans multiple
concerns. A 30-method type at one abstraction level (e.g., a `Conn`
exposing low-level read/write/close) is not a monolith — lagotto's
concern detector is a heuristic on verb prefixes, so verify before
recommending decomposition.

For Facade Methods (G6), inspect `evidence.delegates_to` to confirm
the call really crosses a package boundary. Methods satisfying an
external interface contract (e.g., `io.Reader`) may be required even
if their bodies look like delegations — flag explicitly so the human
decides; the default recommendation is still removal.

For God Dependency Bag (G4), inspect `evidence.packages` — if all
fields belong to one domain, it's a legitimate aggregator; flag only
when imported packages cross domains.

### Step 3 — Cross-check verdicts

Lagotto's heuristics are conservative but not infallible. Before
finalizing the report, sanity-check the highest-severity findings:

- **Receiver Monolith**: read 5–10 of the listed methods; do they
  span unrelated concerns, or is the type cohesive at one
  abstraction level?
- **God Dependency Bag**: do the field types come from genuinely
  unrelated packages, or do they all belong to one layer?
- **Facade Method**: does the body really cross a package boundary?

False positives after this check should be dropped from the report,
not passed through.

## Severity Guide

- **CRITICAL**: Receiver Monolith with ≥25 methods or ≥7 files; Aggregate Holder with ≥50 pointee methods or ≥7 sub-services; Decomposition Theatre with ≥6 aliases; Foreign Holder in ≥5 sites across ≥3 packages; God Dependency Bag with ≥12 fields; Mixed-Concern File ≥600 lines
- **HIGH**: Receiver Monolith ≥15 methods; Aggregate Holder 5–6 sub-services with ≥25 pointee methods; Decomposition Theatre 3–5 aliases; any Foreign Holder escape; Mixed-Concern File 301–599 lines; God Dependency Bag 8–11 fields
- **MEDIUM**: Stutter Names; Build-Tag Pair Sprawl; Mixed-Concern File 100–300 lines; Prefix Cluster of 4+ files; Internal Re-Export Tunnel
- **LOW**: Premature Package; Shadow Suffix; Init Coupling; Method Stub Sprawl; Junk Drawer <100 lines

## Remediation Order

When multiple smells exist in one directory, fix in this order:

1. **Decomposition Theatre (G1B) and Aggregate Holders (G1C) first.** Either signals the affected package is in a partial-refactor state — the receiver split was started but not finished. Finish it before tackling anything else: replace aliases with distinct structs in subpackages, move sub-services out of the holder, update callers. Without this, the rest of the audit cannot be trusted.

2. **Receiver Monoliths.** Type-level fixes unblock everything else. Decomposition pattern (apply the Guiding Principles above):
   - Identify natural concerns (connection, queries, mutations, etc.)
   - Create subpackages: `pkg/connection`, `pkg/query`, `pkg/mutation`, ...
   - Each subpackage exposes its own receiver type (`*Querier`, `*Mutator`) holding the state it needs — passed in at construction, not borrowed from a holder.
   - **Delete the corresponding methods from the original god type.** No accessors like `(t *T) Query() *query.Querier`. No facade methods that delegate. The god type either becomes a thin construction helper (returning sub-services to the caller once at startup) or disappears.
   - **Update every caller in the same change.** Callers learn the new import paths. The patch is incomplete until callers are migrated and old methods deleted.
   - If the type satisfies an external interface (e.g., `graph.GraphStore`), narrow the interface to match the new structure as part of this work. Consumers update too.

3. **God Dependency Bag.** Split into per-concern bags (`AuthDeps`, `GraphDeps`, ...) and update each call site to pass only the bag it needs. Delete the original god bag in the same change.

4. **Mixed-Concern Files.** Split god-files by concern (types in one file, validation in another, utilities in a third).

5. **Facade Methods (G6).** After the receiver decomposition, sweep for any method bodies that are thin pass-throughs. Delete them and migrate callers. The temptation to keep one usually means the receiver split was incomplete.

6. **Standard FS smells.** Apply rename/move/promote-package fixes; many will already be resolved by previous steps.

## Output Format

Group findings by smell, severity-first. For each finding:

1. Smell name + severity
2. Receiver type / file / directory involved
3. Evidence: method count, file count, decl groups, etc.
4. Remediation: be concrete and direct. For Receiver Monolith,
   propose specific subpackage names with the methods that move
   into each, name the callers that must be updated, and state
   that the original methods will be deleted (not retained as
   facades). Do not propose phased migrations, accessor patterns,
   or compatibility shims unless the user has explicitly asked
   for a transition window.

End with a remediation order recommendation (which smell first and
why), and a one-line statement of the expected end state ("after
this refactor, `*TypeDB` no longer exists / is reduced to N fields
and M methods, and every caller imports the relevant subpackage
directly").

## What NOT to Flag

- A `Conn` struct with many low-level methods (`Read`, `Write`, `Close`, `Stat`...) — methods are at the same abstraction level. Not a monolith.
- `package main` with many files — entry-point packages are exempt.
- `internal/<single-file>` packages that exist for visibility boundary, not for grouping (legitimate Go pattern).
- Generated files (`gen/`, `*_gen.go`, `*.pb.go`).
- Test files (`*_test.go`) — different rules.
- A `Deps` struct in a `transport`/`server` aggregator with many fields — that's its job IF the fields are all transport-layer concerns. Flag only when it crosses domains (auth + storage + events + analytics + ...).
