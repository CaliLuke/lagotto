# Formal models

Lean 4 models of Lagotto's pure decision logic. Each module states the
documented contract as a theorem. It also keeps a machine-checked
counterexample for the behaviour before the fix, so a regression in
the model is visible.

| Module | Go code | Contract |
| ------ | ------- | -------- |
| `Lagotto/Suppress.lean` | `audit.locationWithin` | A scoped selector suppresses its location and what is nested beneath it, and nothing else. The fix refines plain prefix matching. |
| `Lagotto/Exclude.lean` | `pkgload.ShouldExclude`, `ModuleRelativeDir`, `moduleRelativePath` | The segment loop is exactly "contiguous run of whole segments". Exclusion is independent of the checkout location and the module path. Loader and walker agree. |
| `Lagotto/Mixed.lean` | `qualifyComponents*`, `ScanMixedConcernWithOptions` | Substantial, minor, and ignored partition the components. Complexity never creates a G5. G5 and G13 are mutually exclusive. |
| `Lagotto/Outcome.lean` | `ConsolidateRepositoryPatterns`, `reportOutcome` | Consolidation is idempotent, keeps non-G12 findings, and preserves the `--fail-on` decision while G12 stays LOW. Incomplete loading exits 1. |

The models use core Lean only (no Mathlib). To check them:

```bash
cd formal
lake build
```

The build fails on any proof error. The models do not contain `sorry`.
