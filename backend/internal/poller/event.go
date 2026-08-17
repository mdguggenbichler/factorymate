package poller

// Event is a detected state transition with template variables (§4.2.1).
// M4 consumes these for render-and-send; M3 always detects regardless of enabled.
type Event struct {
	MessageTypeKey string
	Variables      map[string]string
}
