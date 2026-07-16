# Verifying A True Decomposition

The first check below is the spirit of the spec. The rest are
diagnostic confirmations. If the first check fails, the work is
incomplete regardless of how clean the lagotto output looks.

Replace `<GodType>` with the actual type name (e.g. `TypeDB`) in the
shell snippets below.

1. **The god type does not appear in production caller signatures.**
   This is the spec, not a structural rule.

   ```bash
   grep -rnE '\*<GodType>\b' --include='*.go' | grep -v _test.go
   ```

   Matches must be confined to the package that defines the type,
   the constructor, and at most a `Close`/teardown helper. No
   handler, no MCP tool, no service-layer caller takes the god
   type. If this grep returns matches in `transport/`, `service/`,
   `mcp/`, etc., the decomposition is cosmetic regardless of what
   the detectors say.

2. **Effective method set shrinks.** Run lagotto. The original
   monolith's name should not appear under G1, G1B, G1C, G1D, or G1E.
   For a direct verification:

   ```go
   ms := types.NewMethodSet(types.NewPointer(named))
   fmt.Println(ms.Len()) // should be near zero on the old god type
   ```

3. **No aliases to a shared struct.** `grep -nE '^type \w+ = ' pkg/`
   should not show 3+ aliases pointing at one type.

4. **No same-package aggregate holder.** The old type, if it still
   exists, must not have 5+ pointer-fields to other types defined
   in the same package.

5. **No hidden holder via registry.** The package must not contain
   ≥3 package-level `sync.Map` (or pointer-keyed map) variables
   paired with ≥5 exported accessor functions taking the holder's
   pointer as the first argument. That shape is functionally
   equivalent to fields on the holder, but invisible to a struct
   inspection.

6. **Callers migrated.** `grep -rn 'OldType' --include='*.go'`
   should show migration: callers now import the new subpackages
   and take the narrow type. A successful split touches every
   caller; if the diff is suspiciously small, the split is
   cosmetic.

7. **The old interface narrowed.** If the god type satisfied an
   omnibus interface (`graph.GraphStore`), that interface should
   have been split into per-concern interfaces and consumers
   updated. A `//nolint` on the legacy interface that no production
   code uses is fine; one that 20+ call sites still take is not.

If any of these fails, the work is incomplete regardless of what
lagotto reports — push back and request a real decomposition rather
than accepting the green light.

**A different agent must run this checklist than the one that
implemented the refactor.** Self-review reliably misses what
self-review is biased to miss; the separation is the gate.
