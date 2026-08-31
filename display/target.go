package display

// Target selects a physical BUSY Bar display.
type Target string

const (
	// Front targets the front display.
	Front Target = "front"
	// Back targets the back display.
	Back Target = "back"
)
