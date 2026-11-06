# G16 — Disconnected Package Concerns

Status: implementing (#50)

A large prefix family deserves structural review only when its dependency
boundary can be separated. Start with filename candidates, not graph communities.
Require 3 production files and 800 physical lines, a proper subset of the package,
and at most one inbound file. Reject outbound dependencies that transitively
reach the group. Receiver ownership stays together (bidirectional edges);
implicit local interface families stay together. Exclude generated/test files,
test helper packages, and main packages. Evidence lists boundary files and
symbols, counts and prefixes. Emit one LOW finding per package. This is a review
signal: moving shared core declarations may also be necessary for extraction.

Fixtures: independent prefix plus dispatcher and core; negative multiple callers,
direct/transitive cycle, receiver split, interface split, small group, main/test,
generated files. Consolidate G9 by removing only covered prefixes.

Done: focused tests, corpus hit classifications with >=80% precision, packages
subcommand, batched audit integration, docs/catalog/changelog and check.sh green.
