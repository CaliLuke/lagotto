/-!
# `--suppress SMELL_ID@LOCATION` matching

Models `internal/audit/suppress.go`. Finding locations have the shapes

* `dir/sub`              (package or directory)
* `dir/sub/file.go`      (file)
* `dir/file.go:Symbol`   (function, method, struct)
* `dir/sub (Type)`       (receiver-family findings)

so a location component ends at `/`, `:` or ` `.

Contract: a scoped selector suppresses the selected location and everything
nested beneath it, and nothing else. `G5@cmd/server` must not suppress
`cmd/serverless/main.go`.
-/

namespace Lagotto.Suppress

/-- Characters that end a location component. -/
def boundaryChars : List Char := ['/', ':', ' ']

/-- Specification: `loc` is `sel` itself or nested beneath it. -/
def Within (sel loc : List Char) : Prop :=
  loc = sel ∨ ∃ c ∈ boundaryChars, (sel ++ [c]).isPrefixOf loc = true

instance (sel loc : List Char) : Decidable (Within sel loc) := by
  unfold Within; infer_instance

/-- v0.2.7 behaviour: `strings.HasPrefix(location, selector)`. -/
def oldMatches (sel loc : List Char) : Bool := sel.isPrefixOf loc

/-- The raw string prefix over-suppresses sibling directories. -/
theorem old_unsound : ¬ ∀ sel loc, oldMatches sel loc = true → Within sel loc := by
  intro h
  have := h "cmd/server".toList "cmd/serverless/main.go".toList (by decide)
  revert this; decide

/-- Fixed behaviour (`locationWithin` in Go): the selector is a prefix and the
next character, if any, is a component boundary. -/
def newMatches (sel loc : List Char) : Bool :=
  sel.isPrefixOf loc &&
    match loc.drop sel.length with
    | [] => true
    | c :: _ => boundaryChars.contains c

theorem newMatches_sound (sel loc : List Char) (h : newMatches sel loc = true) :
    Within sel loc := by
  unfold newMatches at h
  simp only [Bool.and_eq_true] at h
  obtain ⟨hp, hb⟩ := h
  have hpre : sel <+: loc := List.isPrefixOf_iff_prefix.mp hp
  obtain ⟨t, rfl⟩ := hpre
  simp only [List.drop_left] at hb
  cases t with
  | nil => left; simp
  | cons c r =>
    right
    refine ⟨c, ?_, ?_⟩
    · simpa using hb
    · apply List.isPrefixOf_iff_prefix.mpr
      exact ⟨r, by simp⟩

theorem newMatches_complete (sel loc : List Char) (h : Within sel loc) :
    newMatches sel loc = true := by
  unfold newMatches
  rcases h with rfl | ⟨c, hc, hp⟩
  · simp [List.isPrefixOf_iff_prefix]
  · obtain ⟨r, rfl⟩ := List.isPrefixOf_iff_prefix.mp hp
    have : (sel ++ [c] ++ r).drop sel.length = c :: r := by simp
    simp only [Bool.and_eq_true]
    refine ⟨List.isPrefixOf_iff_prefix.mpr ⟨c :: r, by simp⟩, ?_⟩
    rw [this]
    simpa using hc

/-- The fix only removes over-matches: anything it suppresses was already
suppressed before, so existing exact and directory selectors keep working. -/
theorem newMatches_refines_old (sel loc : List Char) :
    newMatches sel loc = true → oldMatches sel loc = true := by
  unfold newMatches oldMatches
  intro h
  simp only [Bool.and_eq_true] at h
  exact h.1

-- The README and CHANGELOG selector shapes keep working.
example : newMatches "cmd/server/cli_perf.go".toList "cmd/server/cli_perf.go".toList := by decide
example : newMatches "internal/storage".toList "internal/storage".toList := by decide
example : newMatches "internal/api".toList "internal/api/h.go:Server.Get".toList := by decide
example : newMatches "internal/api/h.go".toList "internal/api/h.go:Server.Get".toList := by decide
example : newMatches "internal/store".toList "internal/store (Store)".toList := by decide
example : newMatches "cmd/server".toList "cmd/serverless/main.go".toList = false := by decide
example : newMatches "internal/a.go".toList "internal/a.go2".toList = false := by decide

end Lagotto.Suppress
