/-!
# G5 / G13 component classification

Models `qualifyComponents`, `qualifyComponentsAfterComplexity`,
`ignoredComponentCount` and the per-file verdict in
`ScanMixedConcernWithOptions` (`internal/detect/mixed.go`).

Documented contracts (docs/patterns/g5-*.md, g13-*.md, `lagotto mixed --help`):

1. "the substantial, minor, and ignored categories account for the raw
   `component_count`" — the three evidence counts partition the components.
2. "`complexity_rejected_component_count` identifies the subset of minor
   components rejected by the post-candidate complexity gate".
3. "Complexity ... never trigger[s] findings. Complexity can only filter out a
   trivial single-callable island after the structural cohesion and size rules
   have already nominated it."
4. G13 "yields to G5": a file never gets both.
-/

namespace Lagotto.Mixed

/-- The fields of `cohesionComponent` the classifier reads. `callables` is the
number of `function`/`method` members. -/
structure Component where
  primary : Nat
  lines : Nat
  callables : Nat
  cycloMax : Nat
  deriving DecidableEq, Repr

structure Options where
  minLines : Nat
  minMembers : Nat
  minComponentLines : Nat
  minSingleComplexity : Nat
  cohesiveMinLines : Nat
  deriving Repr

/-- `ValidateMixedOptions` / `config.Validate`. -/
def Options.Valid (o : Options) : Prop := 1 ≤ o.minLines ∧ 1 ≤ o.minMembers ∧ 1 ≤ o.minComponentLines

def defaults : Options :=
  { minLines := 600, minMembers := 2, minComponentLines := 40,
    minSingleComplexity := 5, cohesiveMinLines := 1200 }

def isSingleCallable (c : Component) : Bool := c.primary == 1 && c.callables == 1

/-- The complexity gate in `qualifyComponentsAfterComplexity`. -/
def rejected (o : Options) (c : Component) : Bool :=
  !(decide (c.primary ≥ o.minMembers)) && decide (c.lines ≥ o.minComponentLines) &&
    decide (o.minSingleComplexity > 0) && isSingleCallable c && decide (c.cycloMax < o.minSingleComplexity)

def ignored (c : Component) : Bool := c.primary == 0

/-- A classifier is the structural nomination rule; the rest is shared. -/
structure Classifier where
  /-- Reads only `minMembers` and `minComponentLines`. -/
  nominate : Nat → Nat → Component → Bool

def Classifier.nominated (k : Classifier) (o : Options) : Component → Bool :=
  k.nominate o.minMembers o.minComponentLines

def Classifier.substantial (k : Classifier) (o : Options) (c : Component) : Bool :=
  !rejected o c && k.nominated o c

def Classifier.minor (k : Classifier) (o : Options) (c : Component) : Bool :=
  rejected o c || (!k.nominated o c && decide (c.primary > 0))

def Classifier.counts (k : Classifier) (o : Options) (cs : List Component) : Nat × Nat × Nat :=
  ((cs.filter (k.substantial o)).length, (cs.filter (k.minor o)).length, (cs.filter ignored).length)

/-- Contract 1 as a proposition. -/
def Classifier.Partitions (k : Classifier) (o : Options) (cs : List Component) : Prop :=
  (k.counts o cs).1 + (k.counts o cs).2.1 + (k.counts o cs).2.2 = cs.length

instance (k : Classifier) (o : Options) (cs : List Component) : Decidable (k.Partitions o cs) := by
  unfold Classifier.Partitions; infer_instance

/-! ## v0.2.7 nomination: members OR summed lines -/

def old : Classifier :=
  ⟨fun members lines c => decide (c.primary ≥ members) || decide (c.lines ≥ lines)⟩

/-- A 40-line `var`/`const` table that nothing in the file references has zero
primary declarations, yet it is nominated as a *substantial* island and is also
counted as ignored. It can therefore trigger G5 on its own. -/
def tableOnly : Component := { primary := 0, lines := 40, callables := 0, cycloMax := 0 }
def realCluster : Component := { primary := 3, lines := 200, callables := 3, cycloMax := 9 }

theorem old_breaks_partition : ¬ old.Partitions defaults [tableOnly] := by decide

theorem old_table_counts_as_island :
    (([tableOnly, realCluster].filter (old.substantial defaults)).length = 2) := by decide

/-! ## Fix: a substantial component must contain a primary declaration -/

def fixed : Classifier :=
  ⟨fun members lines c => decide (c.primary > 0) &&
    (decide (c.primary ≥ members) || decide (c.lines ≥ lines))⟩

theorem rejected_has_primary (o : Options) (c : Component) (h : rejected o c = true) :
    c.primary = 1 := by
  simp [rejected, isSingleCallable] at h
  omega

/-- Each component falls in exactly one category (for the fixed rule). -/
theorem fixed_exactly_one (o : Options) (c : Component) :
    (fixed.substantial o c).toNat + (fixed.minor o c).toNat + (ignored c).toNat = 1 := by
  simp only [Classifier.substantial, Classifier.minor, Classifier.nominated, fixed, ignored]
  by_cases hr : rejected o c = true
  · have := rejected_has_primary o c hr
    simp [hr, this]
  · have hr' : rejected o c = false := by simpa using hr
    rw [hr']
    by_cases hp : c.primary = 0
    · simp [hp]
    · have hp' : 0 < c.primary := Nat.pos_of_ne_zero hp
      generalize (decide (c.primary ≥ o.minMembers) || decide (c.lines ≥ o.minComponentLines)) = q
      cases q <;> simp [hp, hp']

theorem length_filter_add (p q r : Component → Bool) (cs : List Component)
    (h : ∀ c, (p c).toNat + (q c).toNat + (r c).toNat = 1) :
    (cs.filter p).length + (cs.filter q).length + (cs.filter r).length = cs.length := by
  induction cs with
  | nil => rfl
  | cons c cs ih =>
    have hc := h c
    simp only [List.filter_cons, List.length_cons]
    cases hp : p c <;> cases hq : q c <;> cases hr : r c <;>
      simp_all [Bool.toNat] <;> omega

/-- Contract 1 holds for every file and every option set. -/
theorem fixed_partitions (o : Options) (cs : List Component) : fixed.Partitions o cs := by
  simp only [Classifier.Partitions, Classifier.counts]
  exact length_filter_add _ _ _ cs (fixed_exactly_one o)

/-- Contract 2: complexity-rejected components are a subset of minor ones. -/
theorem rejected_subset_minor (k : Classifier) (o : Options) (c : Component) :
    rejected o c = true → k.minor o c = true := by
  intro h; simp [Classifier.minor, h]

/-! ## Per-file verdict -/

inductive Verdict | g5 | g13 | none
  deriving DecidableEq, Repr

/-- Structural nomination before complexity is computed (`qualifyComponents`). -/
def Classifier.initial (k : Classifier) (o : Options) (cs : List Component) : List Component :=
  cs.filter (k.nominated o)

def Classifier.final (k : Classifier) (o : Options) (cs : List Component) : List Component :=
  cs.filter (k.substantial o)

def Classifier.verdict (k : Classifier) (o : Options) (lines : Nat) (dotImports : Bool)
    (cs : List Component) : Verdict :=
  let g5Candidate := decide (lines ≥ o.minLines) && decide ((k.initial o cs).length ≥ 2)
  let g13Candidate := decide (o.cohesiveMinLines > 0) && decide (lines ≥ o.cohesiveMinLines) && !dotImports
  if !g5Candidate && !g13Candidate then .none
  else if lines ≥ o.minLines ∧ (k.final o cs).length ≥ 2 then .g5
  else if g13Candidate && decide ((k.final o cs).length ≤ 1) then .g13
  else .none

theorem final_eq_filter_initial (k : Classifier) (o : Options) (cs : List Component) :
    k.final o cs = (k.initial o cs).filter (fun c => !rejected o c) := by
  simp only [Classifier.final, Classifier.initial, List.filter_filter]
  rfl

theorem final_sublist_initial (k : Classifier) (o : Options) (cs : List Component) :
    (k.final o cs).Sublist (k.initial o cs) := by
  rw [final_eq_filter_initial]; exact List.filter_sublist

/-- Contract 3: turning the complexity gate on never creates a G5. -/
theorem complexity_never_creates_g5 (k : Classifier) (o : Options) (lines : Nat) (d : Bool)
    (cs : List Component) (h : k.verdict o lines d cs = .g5) :
    k.verdict { o with minSingleComplexity := 0 } lines d cs = .g5 := by
  have hsub := (final_sublist_initial k o cs).length_le
  have hinit : k.initial { o with minSingleComplexity := 0 } cs = k.initial o cs := rfl
  have hfinal : k.final { o with minSingleComplexity := 0 } cs = k.initial o cs := by
    rw [final_eq_filter_initial, hinit]
    simp [rejected]
  simp only [Classifier.verdict] at h ⊢
  rw [hinit, hfinal]
  split at h
  · cases h
  · split at h
    · rename_i hg
      have : (k.initial o cs).length ≥ 2 := Nat.le_trans hg.2 hsub
      simp [hg.1, this]
    · split at h <;> cases h

/-- Contract 4: G5 and G13 are mutually exclusive, and a G5 always has the
line floor and two substantial components behind it. -/
theorem g5_evidence (k : Classifier) (o : Options) (lines : Nat) (d : Bool) (cs : List Component)
    (h : k.verdict o lines d cs = .g5) : lines ≥ o.minLines ∧ (k.final o cs).length ≥ 2 := by
  simp only [Classifier.verdict] at h
  split at h
  · cases h
  · split at h
    · assumption
    · split at h <;> cases h

theorem g13_has_at_most_one (k : Classifier) (o : Options) (lines : Nat) (d : Bool) (cs : List Component)
    (h : k.verdict o lines d cs = .g13) : (k.final o cs).length ≤ 1 ∧ lines ≥ o.cohesiveMinLines := by
  simp only [Classifier.verdict] at h
  split at h
  · cases h
  · split at h
    · cases h
    · split at h
      · rename_i hg; simp at hg; exact ⟨hg.2, hg.1.1.2⟩
      · cases h

/-! ## Coverage (issue #56)

With a G13 floor below the G5 floor, a mixed file between the two gets
nothing. `ValidateMixedOptions` now rejects `0 < cohesiveMinLines < minLines`;
under that validation every file past the G13 floor gets exactly one verdict. -/

theorem blind_spot_without_validation :
    fixed.verdict { defaults with minLines := 600, cohesiveMinLines := 500 } 550 false
      [realCluster, realCluster] = .none := by decide

theorem coverage (k : Classifier) (o : Options) (lines : Nat) (cs : List Component)
    (hon : o.cohesiveMinLines > 0) (hfloor : o.cohesiveMinLines ≥ o.minLines)
    (hlines : lines ≥ o.cohesiveMinLines) :
    k.verdict o lines false cs = .g5 ∨ k.verdict o lines false cs = .g13 := by
  have hmin : lines ≥ o.minLines := Nat.le_trans hfloor hlines
  simp only [Classifier.verdict]
  have hg13 : (decide (o.cohesiveMinLines > 0) && decide (lines ≥ o.cohesiveMinLines) && !false) = true := by
    simp [hon, hlines]
  rw [hg13]
  by_cases h2 : (k.final o cs).length ≥ 2
  · left; simp [hmin, h2]
  · right; simp [hmin, h2]; omega

/-- The fixed rule never nominates a zero-primary table, so the example file
above is no longer a G5 candidate. -/
example : (fixed.final defaults [tableOnly, realCluster]).length = 1 := by decide

end Lagotto.Mixed
