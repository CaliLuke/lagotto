# Roadmap

Each markdown file in this directory is one planned feature for
lagotto. Status is tracked in the file's frontmatter (or first line)
and the catalog below; the body is the design / implementation plan
the author wrote before starting.

## Conventions

- **One feature per file.** No multi-feature lumped tasks.
- **Filename is `gNN-short-slug.md`** for new detectors (matching the
  G-numbered smell catalog), or `feat-short-slug.md` for non-detector
  features (subcommands, config formats, output formats, …).
- **Write the spec before writing the code.** Each task doc should
  have, at minimum: why the smell exists, the precise detection
  signal (or feature behavior), positive and negative test fixtures,
  and a Definition of Done.
- **Specs describe intent, not metrics.** Same discipline as the
  layout-refactor plan: say what the detector should _catch_, not
  just what threshold it fires at. (See the skill's "Spirit, Not
  Letter" section.)
- **When a task lands, delete the file.** A merged feature has its
  story in `CHANGELOG.md`, its docs in `docs/patterns/` (for
  detectors) or `README.md` (for features), and its tests in the
  source tree. The roadmap file's job ends at merge.

## Catalog

| File                                                       | Status  | Summary                                                             |
| ---------------------------------------------------------- | ------- | ------------------------------------------------------------------- |
| [g1e-foreign-holder.md](g1e-foreign-holder.md)             | planned | Detector for the Reach-Through Holder pattern (consumer-side smell) |
| [feat-cmd-internal-layout.md](feat-cmd-internal-layout.md) | planned | Restructure repo into the conventional cmd/ + internal/ Go layout   |

Add new entries here when adding files; remove when the file is
deleted at merge.
