package monitor

import "sync"

type RuntimePhase string

const (
	RuntimePhaseColdStart    RuntimePhase = "cold-start"
	RuntimePhaseWarming      RuntimePhase = "warming"
	RuntimePhaseSteady       RuntimePhase = "steady"
	RuntimePhaseShuttingDown RuntimePhase = "shutting-down"
)

type RuntimeStateSnapshot struct {
	Readiness bool
	Phase     RuntimePhase
}

var runtimeState = struct {
	mu         sync.RWMutex
	ready      bool
	phase      RuntimePhase
	warmupDone bool
}{
	phase: RuntimePhaseColdStart,
}

func ResetRuntimeState() {
	runtimeState.mu.Lock()
	runtimeState.ready = false
	runtimeState.phase = RuntimePhaseColdStart
	runtimeState.warmupDone = false
	runtimeState.mu.Unlock()
}

func SetRuntimeState(ready bool, phase RuntimePhase) {
	runtimeState.mu.Lock()
	runtimeState.ready = ready
	runtimeState.phase = phase
	if phase == RuntimePhaseColdStart {
		runtimeState.warmupDone = false
	}
	runtimeState.mu.Unlock()
}

func SetRuntimeServingPhase(warmupEnabled bool) RuntimeStateSnapshot {
	phase := RuntimePhaseSteady
	if warmupEnabled {
		runtimeState.mu.RLock()
		warmupDone := runtimeState.warmupDone
		runtimeState.mu.RUnlock()
		if !warmupDone {
			phase = RuntimePhaseWarming
		}
	}
	SetRuntimeState(true, phase)
	return RuntimeStateSnapshot{Readiness: true, Phase: phase}
}

func MarkRuntimeShuttingDown() RuntimeStateSnapshot {
	SetRuntimeState(false, RuntimePhaseShuttingDown)
	return RuntimeStateSnapshot{Readiness: false, Phase: RuntimePhaseShuttingDown}
}

func MarkRuntimeWarmupComplete() (RuntimeStateSnapshot, bool) {
	runtimeState.mu.Lock()
	defer runtimeState.mu.Unlock()
	runtimeState.warmupDone = true
	if !runtimeState.ready || runtimeState.phase != RuntimePhaseWarming {
		return RuntimeStateSnapshot{
			Readiness: runtimeState.ready,
			Phase:     runtimeState.phase,
		}, false
	}
	runtimeState.phase = RuntimePhaseSteady
	return RuntimeStateSnapshot{
		Readiness: runtimeState.ready,
		Phase:     runtimeState.phase,
	}, true
}

func RuntimeState() RuntimeStateSnapshot {
	runtimeState.mu.RLock()
	defer runtimeState.mu.RUnlock()
	return RuntimeStateSnapshot{
		Readiness: runtimeState.ready,
		Phase:     runtimeState.phase,
	}
}
