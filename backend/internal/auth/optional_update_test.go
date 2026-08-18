package auth_test

import (
	"encoding/json"
	"testing"

	"factorymate/internal/auth"
)

func TestParseOptionalPlayerID(t *testing.T) {
	t.Run("omitted", func(t *testing.T) {
		got, err := auth.ParseOptionalPlayerID(nil)
		if err != nil || got != nil {
			t.Fatalf("got = %v err = %v, want nil nil", got, err)
		}
	})

	t.Run("null clears", func(t *testing.T) {
		got, err := auth.ParseOptionalPlayerID(json.RawMessage("null"))
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if got == nil || *got != nil {
			t.Fatalf("got = %v, want non-nil outer with nil inner", got)
		}
	})

	t.Run("empty string clears", func(t *testing.T) {
		got, err := auth.ParseOptionalPlayerID(json.RawMessage(`""`))
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if got == nil || *got != nil {
			t.Fatalf("got = %v, want clear sentinel", got)
		}
	})

	t.Run("value sets", func(t *testing.T) {
		got, err := auth.ParseOptionalPlayerID(json.RawMessage(`"player-1"`))
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if got == nil || *got == nil || **got != "player-1" {
			t.Fatalf("got = %v, want player-1", got)
		}
	})
}
