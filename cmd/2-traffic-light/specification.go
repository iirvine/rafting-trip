package main

func EWGreen(s State) bool {
	return s.EW == Green && s.NS == Red
}

func NSGreen(s State) bool {
	return s.EW == Red && s.NS == Green
}

func EWYellow(s State) bool {
	return s.EW == Yellow && s.NS == Red
}

func NSYellow(s State) bool {
	return s.EW == Red && s.NS == Yellow
}

func EWYellowHold(s State, c Controller) bool {
	return EWYellow(s) && c.clock < 5
}

func NSYellowHold(s State, c Controller) bool {
	return NSYellow(s) && c.clock < 5
}

func EWGreenHold(s State, c Controller) bool {
	return EWGreen(s) && s.EWButtonPressed == false && c.clock < 30
}

func EWGreenHoldShort(s State, c Controller) bool {
	return EWGreen(s) && s.EWButtonPressed && c.clock < 15
}

func NSGreenHold(s State, c Controller) bool {
	return NSGreen(s) && s.NSButtonPressed == false && c.clock < 60
}

func NSGreenHoldShort(s State, c Controller) bool {
	return NSGreen(s) && s.NSButtonPressed && c.clock < 15
}

func ValidState(s State, c Controller) bool {
	return EWYellowHold(s, c) ||
		NSYellowHold(s, c) ||
		EWGreenHold(s, c) ||
		EWGreenHoldShort(s, c) ||
		NSGreenHold(s, c) ||
		NSGreenHoldShort(s, c)
}

// These are the events that can be sent to the controller
func ClockTick(c Controller) {
	c.clock++
}

func EWButtonPress(s State) {
	s.EWButtonPressed = true
}

func NSButtonPress(s State) {
	s.NSButtonPressed = true
}

// What happens for any event
func Next(event Event, c Controller, s State) {
	if event.Type() == Tick {
		ClockTick(c)
	}
}
