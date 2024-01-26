package main

import (
	"log/slog"
	"time"
)

type EventType string
type Target string

const (
	NS Target = "ns"
	EW Target = "ew"
)

const (
	Button          EventType = "button"
	Tick            EventType = "tick"
	StateTransition EventType = "state-transition"
)

type Event interface {
	Type() EventType
}

type ButtonEvent struct {
	Target Target
}

func (e *ButtonEvent) Type() EventType {
	return Button
}

type StateTransitionEvent struct {
	From State
	To   State
}

func (e *StateTransitionEvent) Type() EventType {
	return StateTransition
}

// Controller houses the state machine, as well as various timers and channels for communication.
type Controller struct {
	inbox              chan Event
	outbox             chan Event
	machine            StateMachine
	clock              int64
	nextTransitionTick int64
	ticker             *time.Ticker
}

func NewController() *Controller {
	return &Controller{
		inbox:  make(chan Event),
		outbox: make(chan Event),
		machine: StateMachine{
			state: DefaultState,
		},
		clock:              0,
		ticker:             time.NewTicker(1 * time.Second),
		nextTransitionTick: 30,
	}
}

func (c *Controller) Run() {
	c.emitInitialState()
	for {
		select {
		case event := <-c.inbox:
			c.handleEvent(event)
		case <-c.ticker.C:
			c.handleTick()
		}
	}
}

func (c *Controller) emitInitialState() {
	c.outbox <- &StateTransitionEvent{
		From: DefaultState,
		To:   DefaultState,
	}
}

func (c *Controller) handleEvent(event Event) {
	switch event.Type() {
	case Button:
		event := event.(*ButtonEvent)
		c.handleButton(event)
	default:
		slog.Error("unknown event", "event", event)
	}
}

func (c *Controller) handleButton(event *ButtonEvent) {
	state := c.machine.State()
	switch event.Target {
	case NS:
		if state.NSButtonPressed {
			return
		}
		if state.NS == Green && c.clock >= 15 {
			c.handleTransition()
			return
		}
		if state.NS == Green {
			c.machine.handleButtonPress(NS)
			c.nextTransitionTick = 15 - c.clock
			c.clock = 0
			return
		}

	case EW:
		if state.EWButtonPressed {
			return
		}
		if state.EW == Green && c.clock >= 15 {
			c.handleTransition()
			return
		}
		if state.EW == Green {
			c.machine.handleButtonPress(EW)
			c.nextTransitionTick = 15 - c.clock
			c.clock = 0
			return
		}
	}
}

func (c *Controller) setNextTransitionTick() {
	state := c.machine.State()
	if state.NS == Yellow || state.EW == Yellow {
		c.nextTransitionTick = 5
	} else if state.NS == Green {
		c.nextTransitionTick = 30
	} else if state.EW == Green {
		c.nextTransitionTick = 60
	}
}

func (c *Controller) handleTransition() {
	from := c.machine.State()
	c.machine.Transition()
	to := c.machine.State()
	c.outbox <- &StateTransitionEvent{
		From: from,
		To:   to,
	}
}

func (c *Controller) handleTick() {
	c.clock++
	if c.clock == c.nextTransitionTick {
		c.clock = 0
		c.handleTransition()
		c.setNextTransitionTick()
	}
}

func (c *Controller) State() State {
	return c.machine.State()
}
