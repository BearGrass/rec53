package monitor

import "testing"

func TestRuntimeStateWarmupCompletionBeforeServingKeepsBoundedState(t *testing.T) {
	ResetRuntimeState()
	t.Cleanup(ResetRuntimeState)

	state, changed := MarkRuntimeWarmupComplete()
	if changed {
		t.Fatalf("MarkRuntimeWarmupComplete changed = true, state = %+v", state)
	}
	if state.Readiness {
		t.Fatalf("state.Readiness = true, want false")
	}
	if state.Phase != RuntimePhaseColdStart {
		t.Fatalf("state.Phase = %s, want %s", state.Phase, RuntimePhaseColdStart)
	}

	state = SetRuntimeServingPhase(true)
	if !state.Readiness {
		t.Fatal("state.Readiness = false, want true")
	}
	if state.Phase != RuntimePhaseSteady {
		t.Fatalf("state.Phase = %s, want %s", state.Phase, RuntimePhaseSteady)
	}
}

func TestRuntimeStateResetClearsWarmupCompletion(t *testing.T) {
	ResetRuntimeState()
	t.Cleanup(ResetRuntimeState)

	MarkRuntimeWarmupComplete()
	ResetRuntimeState()

	state := SetRuntimeServingPhase(true)
	if !state.Readiness {
		t.Fatal("state.Readiness = false, want true")
	}
	if state.Phase != RuntimePhaseWarming {
		t.Fatalf("state.Phase = %s, want %s", state.Phase, RuntimePhaseWarming)
	}
}
