package auth

import "encoding/json"

// ParseOptionalPlayerID decodes playerId from a PUT /users request body.
// Returns nil when the field is omitted; non-nil outer with nil inner to clear mapping.
func ParseOptionalPlayerID(raw json.RawMessage) (**string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	if string(raw) == "null" {
		var cleared *string
		return &cleared, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	if s == "" {
		var cleared *string
		return &cleared, nil
	}
	inner := s
	ptr := &inner
	return &ptr, nil
}
