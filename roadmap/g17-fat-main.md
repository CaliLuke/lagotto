# G17 — Fat Main

Status: implementing (#51)

Large command packages can hide reusable logic. Require 1000 production physical
lines plus either 3 declared types with methods or 15 top-level functions other
than main/init. Complexity ranks the top five functions but never creates a
finding alone. Keep LOW even in multi-package modules: this is a boundary review,
not evidence that a small standalone utility needs a framework.

Skip generated/test files, test helper packages, examples/example and testdata
path components. Report file/line/type/function counts, complexity hotspots,
and whether an internal directory exists in the module.

Fixtures: large command with types/methods and helpers; small wiring main,
generated/test/example code, large declaration-only main. Done: corpus review,
audit and mains subcommand, docs/catalog/changelog, tests and check.sh green.
