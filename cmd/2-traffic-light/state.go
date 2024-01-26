package main

import "log/slog"

type Light int

const (
	Red = iota
	Yellow
	Green
)

func (s Light) String() string {
	switch s {
	case Red:
		return "Red"
	case Yellow:
		return "Yellow"
	case Green:
		return "Green"
	default:
		return "Unknown"
	}
}

type State struct {
	NS              Light
	EW              Light
	NSButtonPressed bool
	EWButtonPressed bool
}

var DefaultState = State{
	NS: Green,
	EW: Red,
}

type StateMachine struct {
	state State
}

func (sm *StateMachine) Transition() State {
	sm.state.NSButtonPressed = false
	sm.state.EWButtonPressed = false
	if sm.state.NS == Green && sm.state.EW == Red {
		sm.state.NS = Yellow
	} else if sm.state.NS == Yellow && sm.state.EW == Red {
		sm.state.NS = Red
		sm.state.EW = Green
	} else if sm.state.NS == Red && sm.state.EW == Green {
		sm.state.EW = Yellow
	} else if sm.state.NS == Red && sm.state.EW == Yellow {
		sm.state.NS = Green
		sm.state.EW = Red
	} else {
		slog.Error("invalid state", "state", sm.state)
	}
	return sm.state
}

func (sm *StateMachine) State() State {
	return sm.state
}

func (sm *StateMachine) handleButtonPress(ns Target) {
	switch ns {
	case NS:
		sm.state.NSButtonPressed = true
	case EW:
		sm.state.EWButtonPressed = true
	}
}
