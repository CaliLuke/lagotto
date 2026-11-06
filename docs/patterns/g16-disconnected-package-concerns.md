# G16 — Disconnected Package Concerns

A substantial filename-prefix family has a narrow, acyclic dependency boundary
inside a package. This is a LOW review signal for a potential subpackage.

Candidates use G9's prefix rules and must contain at least three authored,
non-test files totaling 800 physical lines. A candidate must be a proper subset
of the package. Main and test-helper packages are excluded.

The file graph records references to same-package declarations, including
fields and methods. Receiver declarations and their methods stay together;
local named interface families are conservatively connected in both directions.
Generated files cannot nominate candidates but their references still constrain
extraction. At most one outside file may reference the candidate, and no
outbound dependency may reach the candidate, even transitively.

For example, `lang_parse.go`, `lang_resolve.go` and `lang_emit.go` may form a
candidate when only `runner.go` calls them and they use independent declarations
in `core.go`. A group called by `handler.go`, `service.go` and `store.go` does
not qualify. Neither does a group whose core calls back into it, or one splitting
a type from its methods.

One finding per package lists `evidence.groups`: each group's prefix, files,
line count, inbound references and outbound dependencies. Each reference gives
`from`, `to` and `symbol`. Evidence is sorted for repeatable reports.

Run `lagotto packages [path]` to inspect G16 alone. In a full audit G16 replaces
only the matching G9 prefixes, preserving other prefix clusters and directory
suppressions. Suppressing G16 does not restore its redundant G9 prefixes.

This is not a claim that moving files unchanged will compile. Shared core
objects may need their own neutral package, unexported boundary identifiers may
need an API, and runtime registration and side effects still need human review.
Only active build-tagged files are structurally analyzed. Graph communities
without filename prefixes are deliberately deferred until corpus evidence
justifies their cost and precision.
