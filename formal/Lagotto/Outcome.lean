/-!
# Report outcome: G12 consolidation, `--fail-on`, exit codes

Models `audit.ConsolidateRepositoryPatterns`, `Severity.AtLeast` and
`detect.reportOutcome`.

Contracts:

* Consolidation is a presentation step. It must not change whether
  `--fail-on` trips (CI must not pass or fail because five G12s were grouped).
* Consolidation is idempotent and leaves every non-G12 finding untouched.
* Incomplete loading exits 1 even when findings also meet `--fail-on`.
-/

namespace Lagotto.Outcome

inductive Sev | critical | high | medium | low
  deriving DecidableEq, Repr

def Sev.rank : Sev → Nat
  | .critical => 0 | .high => 1 | .medium => 2 | .low => 3

def Sev.atLeast (s t : Sev) : Bool := s.rank ≤ t.rank

structure Finding where
  id : String
  sev : Sev
  loc : String
  deriving DecidableEq, Repr

def isG12 (f : Finding) : Bool := f.id == "G12"

def grouped : Finding := ⟨"G12", .low, "."⟩

def consolidate (fs : List Finding) : List Finding :=
  if (fs.filter isG12).length < 5 then fs else fs.filter (!isG12 ·) ++ [grouped]

def failCount (t : Sev) (fs : List Finding) : Nat := (fs.filter (·.sev.atLeast t)).length

def exitCode (loadErrors : Nat) (failOn : Option Sev) (fs : List Finding) : Nat :=
  if loadErrors > 0 then 1
  else match failOn with
    | none => 0
    | some t => if failCount t fs > 0 then 2 else 0

theorem exit_incomplete_wins (n : Nat) (t : Option Sev) (fs : List Finding) (h : n > 0) :
    exitCode n t fs = 1 := by simp [exitCode, h]

theorem consolidate_keeps_others (fs : List Finding) :
    (consolidate fs).filter (!isG12 ·) = fs.filter (!isG12 ·) := by
  unfold consolidate
  split
  · rfl
  · simp [List.filter_append, List.filter_filter, grouped, isG12]

theorem consolidate_idempotent (fs : List Finding) :
    consolidate (consolidate fs) = consolidate fs := by
  unfold consolidate
  split
  · simp_all
  · have : ((fs.filter (!isG12 ·) ++ [grouped]).filter isG12).length = 1 := by
      simp [List.filter_append, List.filter_filter, grouped, isG12]
    exact ite_eq_left_iff.mpr (fun h => absurd (by rw [this]; omega) h)

theorem failCount_pos_iff (t : Sev) (fs : List Finding) :
    failCount t fs > 0 ↔ ∃ f ∈ fs, f.sev.atLeast t = true := by
  simp [failCount, List.length_pos_iff_exists_mem]

/-- Consolidation preserves the `--fail-on` decision, provided G12 is always
emitted at LOW (which `prematurePackageFindings` does). -/
theorem consolidate_preserves_failOn (t : Sev) (fs : List Finding)
    (hlow : ∀ f ∈ fs, isG12 f = true → f.sev = .low) :
    (failCount t (consolidate fs) > 0) ↔ (failCount t fs > 0) := by
  unfold consolidate
  split
  · exact Iff.rfl
  · rename_i hge
    rw [failCount_pos_iff, failCount_pos_iff]
    have hne : fs.filter isG12 ≠ [] := by
      intro h; rw [h] at hge; simp at hge
    obtain ⟨g, hg⟩ := List.exists_mem_of_ne_nil _ hne
    rw [List.mem_filter] at hg
    constructor
    · rintro ⟨f, hf, hat⟩
      rw [List.mem_append, List.mem_filter, List.mem_singleton] at hf
      rcases hf with ⟨hf, _⟩ | rfl
      · exact ⟨f, hf, hat⟩
      · refine ⟨g, hg.1, ?_⟩
        rw [hlow g hg.1 hg.2]; exact hat
    · rintro ⟨f, hf, hat⟩
      by_cases h12 : isG12 f = true
      · refine ⟨grouped, by simp, ?_⟩
        have := hlow f hf h12
        rw [this] at hat; exact hat
      · exact ⟨f, by simp [List.mem_filter, hf, h12], hat⟩

/-- Without the LOW invariant the property fails, so the Go invariant matters:
a hypothetical HIGH G12 would be masked by grouping. -/
theorem failOn_needs_low_g12 :
    ¬ ∀ fs, (failCount .high (consolidate fs) > 0) ↔ (failCount .high fs > 0) := by
  intro h
  have := h (List.replicate 5 ⟨"G12", .high, "p"⟩)
  revert this; decide

end Lagotto.Outcome
