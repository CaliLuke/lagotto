# G17 — Fat Main

A large `package main` with its own types and helper functions can hide reusable
logic behind an unimportable entry point. G17 is always LOW: self-contained
commands may reasonably choose this layout.

The detector requires at least 1000 physical lines of authored production code
and either three declared types with methods or fifteen top-level functions
other than `main` and `init`. Comments contribute to physical lines, but size
alone cannot trigger a finding. Complexity ranks the five reported hotspots;
it is not an independent trigger.

Tests, generated files, test-helper packages, and path components `example`,
`examples` and `testdata` are excluded. An example should demonstrate an API,
not be forced into a production package architecture. Types declared only in
generated files do not count toward the structural threshold.

A large command with store/server/handler types and many helper functions
qualifies. A small flag parser plus `run`, or a large declaration table with
only `main`, does not. Single-package tools receive the same LOW review signal
as commands in a larger module; neither module size nor an `internal/` tree
proves that extraction is worthwhile.

Run `lagotto mains [path]` or a full audit. Evidence includes physical lines,
file and type counts, types with methods, helper-function count, the top five
complexity hotspots (with file/name/line), and `has_internal`, indicating an
existing module-root `internal/` directory.

Keep command-line parsing and wiring in main. Move a capability only when it
has a useful importable API or benefits from independent testing and reuse.
