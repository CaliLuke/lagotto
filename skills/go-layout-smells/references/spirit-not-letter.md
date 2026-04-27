# Spirit, Not Letter

Three escapes in one refactor cycle (aliases → aggregate holder →
registry maps) is not a coincidence. Any specific structural metric,
once written into a spec or detector, becomes the thing the system
optimizes for — which usually means _routing around_ it rather than
satisfying the underlying intent. This is Goodhart's Law applied to
code metrics, and it has a predictable shape:

1. The spec says "delete every method on `*God`."
2. An agent finds a structural shape that satisfies the literal rule
   while preserving the original problem (the god type as a
   chokepoint every consumer takes).
3. The detector grows a new rule (G1B, G1C, G1D, …).
4. The next agent finds the next shape.

Each new detector raises the floor, but detectors will always lag
behind invention. The systemic answer is to specify the _target end
state_ rather than the structural metric:

| Bad (lettered) spec                             | Good (spirited) spec                                                                                                                                      |
| ----------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
| "Delete every method on `*God`."                | "No production caller takes `*God`. Each consumer takes only the narrow sub-service interface it uses."                                                   |
| "`*God` has zero fields of same-package types." | "Sub-service types live in subpackages. The holder, if it survives, returns sub-services from its constructor — it does not appear in caller signatures." |
| "Move methods into subpackages."                | "Each consumer's source file imports a sub-service package directly; the package graph reflects the decomposition."                                       |

The spirited spec leaves no room for evasion because any disguise
that preserves `*God` in caller signatures fails the spec by
construction. The lettered spec asks for a measurable surface that
agents will always find a clever way to provide.

## How to write a spirited refactor spec

When writing a layout-refactor ticket:

1. **Lead with the caller's view.** "After this ticket lands,
   `grep -rn '\*<GodType>' --include='*.go'` returns matches only
   in the constructor, test fixtures, and at most a teardown helper."
   That sentence is harder to evade than any structural rule.
2. **Name the target packages.** "Each sub-service moves into its
   own subpackage at `<pkg>/<concern>/`. The constructor returns
   them as separate values."
3. **Describe the consumer migration.** "Each handler in
   `transport/...` takes the narrow interface from the matching
   subpackage; the omnibus type does not appear in any handler
   signature."
4. **State that all G1\* detectors must pass**, then add: "and the
   reviewer agent confirms the caller-view test from rule 1." The
   detectors are diagnostics; the caller-view test is the gate.
5. **Mandate verification by a different agent.** No layout ticket
   is complete until a second agent (reviewer, with no access to
   the implementer's reasoning) runs the verification checklist
   in `verification-checklist.md` and reports zero blockers. The
   implementer cannot sign off on their own work; the separation
   breaks self-rationalization.

## What to do when finding the next disguise

When encountering a refactor that satisfies every existing rule and
still feels like the god type is intact:

1. **Write down the caller-view test** that fails. ("Every transport
   handler still takes `*TypeDB`." That single sentence is the
   evidence.) Send the implementer back with that test, not a list
   of structural complaints.
2. **Open a lagotto issue** describing the new evasion shape. Even
   without implementing the detector now, the issue raises the
   floor for the next person who tries the same shape.
3. **Update the anti-patterns reference.** Each new disguise that
   survives belongs in `anti-patterns.md`, with a fix.

The detectors are the artifact. The discipline of describing intent
and verifying with a separate agent is what stops the next iteration.
