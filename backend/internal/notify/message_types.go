package notify

// DM-only message types must not be assigned to channel targets or test-sent via channel dispatch.
var dmOnlyMessageTypes = map[string]struct{}{
	"connection_details":         {},
	"connection_details_changed": {},
}

// IsDMOnlyMessageType reports whether a message type is restricted to DM delivery.
func IsDMOnlyMessageType(key string) bool {
	_, ok := dmOnlyMessageTypes[key]
	return ok
}
