package main

import (
	"log/slog"
	"testing"
)

func TestStateMachine(t *testing.T) {
	sm := StateMachine{state: DefaultState}
	state := sm.Transition()
	if state.NS != Yellow {
		t.Errorf("expected yellow, got %v", state.NS)
	}
}

func TestController_Run(t *testing.T) {
	ctrl := NewController()
	ctrl.clock = 29

	go func() {
		for {
			recv := <-ctrl.outbox
			slog.Info("received event", "event", recv)
		}
	}()

	ctrl.handleTick()
	if ctrl.State().NS != Yellow {
		t.Errorf("expected green, got %v", ctrl.State().NS)
	}

	if ctrl.clock != 0 {
		t.Errorf("expected clock to be 0, got %v", ctrl.clock)
	}

	if ctrl.nextTransitionTick != 5 {
		t.Errorf("expected nextTransitionTick to be 5, got %v", ctrl.nextTransitionTick)
	}

	ctrl.clock = 4
	ctrl.handleTick()
}

func TestButtonEvent(t *testing.T) {
	ctrl := NewController()
	go func() {
		for {
			recv := <-ctrl.outbox
			slog.Info("received event", "event", recv)
		}
	}()

	ctrl.clock = 14
	ctrl.handleButton(&ButtonEvent{Target: NS})
	if ctrl.nextTransitionTick != 1 {
		t.Errorf("expected nextTransitionTick to be 5, got %v", ctrl.nextTransitionTick)
	}
	ctrl.handleTick()
}
