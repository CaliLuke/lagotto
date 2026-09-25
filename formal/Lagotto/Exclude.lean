/-!
# `--exclude` segment matching

Models `pkgload.ShouldExclude` / `matchesSegments` and its two call sites:

* `pkgload.load` filters packages by import path
  (`modulePath ++ rel`), and
* `detect.collectPackageDirs` filters walked directories by filesystem path
  (`checkoutDir ++ rel`).

Paths are lists of segments (the `strings.Split(path, "/")` result).

Contract (README, CHANGELOG 0.2.x): a pattern names directories *inside the
audited module*. Whether a package is excluded must not depend on where the
module is checked out or on the module path, and the type-based and
filesystem-based detectors must agree on the excluded set.
-/

namespace Lagotto.Exclude

abbrev Path := List String

/-- Literal model of the Go loop in `matchesSegments`:
`for i := 0; i+len(pat) <= len(segs); i++ { if segs[i:i+len(pat)] == pat ... }`. -/
def matchesSegments (segs pat : Path) : Bool :=
  (List.range (segs.length + 1 - pat.length)).any fun i =>
    (List.range pat.length).all fun j => segs[i + j]? == pat[j]?

/-- The loop is exactly "pat occurs as a contiguous run of whole segments". -/
theorem matchesSegments_iff (segs pat : Path) :
    matchesSegments segs pat = true ↔ ∃ pre suf, segs = pre ++ pat ++ suf := by
  unfold matchesSegments
  simp only [List.any_eq_true, List.mem_range, List.all_eq_true, beq_iff_eq]
  constructor
  · rintro ⟨i, hi, hall⟩
    have hlen : i + pat.length ≤ segs.length := by omega
    refine ⟨segs.take i, segs.drop (i + pat.length), ?_⟩
    have hmid : (segs.drop i).take pat.length = pat := by
      apply List.ext_getElem?
      intro j
      by_cases hj : j < pat.length
      · rw [List.getElem?_take_of_lt hj, List.getElem?_drop]
        exact hall j hj
      · rw [List.getElem?_eq_none (by simp; omega), List.getElem?_eq_none (by omega)]
    conv => lhs; rw [← List.take_append_drop i segs]
    rw [List.append_assoc]
    congr 1
    conv => lhs; rw [← List.take_append_drop pat.length (segs.drop i)]
    rw [hmid, List.drop_drop]
  · rintro ⟨pre, suf, rfl⟩
    refine ⟨pre.length, ?_, ?_⟩
    · simp; omega
    · intro j hj
      rw [List.append_assoc, List.getElem?_append_right (by omega)]
      simp only [Nat.add_sub_cancel_left]
      rw [List.getElem?_append_left hj]

/-- `ShouldExclude(path, exclude)`: empty patterns are skipped. -/
def shouldExclude (path : Path) (exclude : List Path) : Bool :=
  exclude.any fun pat => !pat.isEmpty && matchesSegments path pat

/-! ## v0.2.7: patterns are matched against the whole path -/

/-- Loader filter: import path, which starts with the module path. -/
def loaderExcludesOld (modulePath rel : Path) (ex : List Path) : Bool :=
  shouldExclude (modulePath ++ rel) ex

/-- Walker filter: filesystem path, which starts with the checkout directory. -/
def walkerExcludesOld (checkoutDir rel : Path) (ex : List Path) : Bool :=
  shouldExclude (checkoutDir ++ rel) ex

/-- A checkout under a directory named `gen` silently drops every
filesystem finding (reproduced: `~/src/gen/proj` reports 0 findings). -/
theorem old_walker_depends_on_checkout :
    ¬ ∀ dir dir' rel, walkerExcludesOld dir rel [["gen"]] = walkerExcludesOld dir' rel [["gen"]] := by
  intro h
  have := h ["home", "ci", "gen", "proj"] ["home", "ci", "src", "proj"] ["internal", "solo"]
  revert this; decide

/-- A module whose path contains a default-excluded segment
(`example.com/gen`, `github.com/acme/vendor`) loads zero packages. -/
theorem old_loader_depends_on_module_path :
    ¬ ∀ m m' rel, loaderExcludesOld m rel [["gen"]] = loaderExcludesOld m' rel [["gen"]] := by
  intro h
  have := h ["example.com", "gen"] ["example.com", "other"] ["internal", "solo"]
  revert this; decide

/-- The two filters disagree, so G12/G9/G11 and G1/G2/G5 audit different sets. -/
theorem old_filters_disagree :
    ¬ ∀ m dir rel, loaderExcludesOld m rel [["vendor"]] = walkerExcludesOld dir rel [["vendor"]] := by
  intro h
  have := h ["github.com", "acme", "vendor"] ["home", "me", "vendor"] ["pkg"]
  revert this
  have := h ["github.com", "acme", "api"] ["home", "vendor", "api"] ["pkg"]
  revert this; decide

/-! ## Fix: strip the base (module path / module root directory) first -/

/-- Go: `strings.CutPrefix(pkgPath, module.Path+"/")` in the loader and
`filepath.Rel(moduleRoot, dir)` in the walker. -/
def excludesRelative (base full : Path) (ex : List Path) : Bool :=
  shouldExclude (full.drop base.length) ex

theorem excludesRelative_independent (base base' rel : Path) (ex : List Path) :
    excludesRelative base (base ++ rel) ex = excludesRelative base' (base' ++ rel) ex := by
  simp [excludesRelative]

/-- Loader and walker now agree for every module path and checkout. -/
theorem loader_walker_agree (modulePath checkoutDir rel : Path) (ex : List Path) :
    excludesRelative modulePath (modulePath ++ rel) ex
      = excludesRelative checkoutDir (checkoutDir ++ rel) ex :=
  excludesRelative_independent _ _ _ _

/-- Soundness: an exclusion is always justified by a non-empty pattern that
occurs entirely inside the module-relative path. -/
theorem excludesRelative_sound (base rel : Path) (ex : List Path)
    (h : excludesRelative base (base ++ rel) ex = true) :
    ∃ pat ∈ ex, pat ≠ [] ∧ ∃ pre suf, rel = pre ++ pat ++ suf := by
  simp only [excludesRelative, List.drop_left, shouldExclude, List.any_eq_true,
    Bool.and_eq_true, Bool.not_eq_true'] at h
  obtain ⟨pat, hmem, hne, hm⟩ := h
  refine ⟨pat, hmem, ?_, (matchesSegments_iff _ _).mp hm⟩
  cases pat <;> simp_all

/-- Completeness: every such occurrence is excluded (segment anchoring kept). -/
theorem excludesRelative_complete (base rel pat pre suf : Path) (ex : List Path)
    (hmem : pat ∈ ex) (hne : pat ≠ []) (hrel : rel = pre ++ pat ++ suf) :
    excludesRelative base (base ++ rel) ex = true := by
  simp only [excludesRelative, List.drop_left, shouldExclude, List.any_eq_true,
    Bool.and_eq_true, Bool.not_eq_true']
  refine ⟨pat, hmem, ?_, (matchesSegments_iff _ _).mpr ⟨pre, suf, hrel⟩⟩
  cases pat <;> simp_all

-- CHANGELOG examples still hold on the relative path.
example : excludesRelative ["example.com", "m"] ["example.com", "m", "a", "gen", "b"] [["gen"]] := by decide
example : excludesRelative ["example.com", "m"] ["example.com", "m", "a", "agent", "b"] [["gen"]] = false := by decide
example : excludesRelative ["m"] ["m", "x", "design", "generated"] [["design", "generated"]] := by decide
example : excludesRelative ["m"] ["m", "x", "redesign", "generated"] [["design", "generated"]] = false := by decide

end Lagotto.Exclude
