package savegame

import (
	"testing"
)

func TestPickLatestAutosave(t *testing.T) {
	sessions := []Session{{
		SessionName: "Conveyor Belt Cult",
		SaveHeaders: []SaveHeader{
			{SaveName: "Conveyor Belt Cult_autosave_0", SaveDateTime: "2026.08.22-15.18.49"},
			{SaveName: "Conveyor Belt Cult_autosave_3", SaveDateTime: "2026.08.22-15.38.00"},
			{SaveName: "Conveyor Belt Cult_autosave_1", SaveDateTime: "2026.08.22-15.28.49"},
		},
	}}

	got, err := PickLatestAutosave("Conveyor Belt Cult", sessions)
	if err != nil {
		t.Fatalf("PickLatestAutosave: %v", err)
	}
	if got.SaveName != "Conveyor Belt Cult_autosave_3" {
		t.Fatalf("SaveName = %q, want autosave_3", got.SaveName)
	}
}

func TestPickLatestAutosave_NoSession(t *testing.T) {
	_, err := PickLatestAutosave("Missing", []Session{{
		SessionName: "Other",
		SaveHeaders: []SaveHeader{{SaveName: "x_autosave_0", SaveDateTime: "2026.08.22-15.18.49"}},
	}})
	if err != ErrNoActiveSave {
		t.Fatalf("err = %v, want ErrNoActiveSave", err)
	}
}

func TestPickLatestAutosave_FallbackNonAutosave(t *testing.T) {
	sessions := []Session{{
		SessionName: "Test",
		SaveHeaders: []SaveHeader{
			{SaveName: "manual_save", SaveDateTime: "2026.08.22-12.00.00"},
			{SaveName: "older_manual", SaveDateTime: "2026.08.21-12.00.00"},
		},
	}}
	got, err := PickLatestAutosave("Test", sessions)
	if err != nil {
		t.Fatalf("PickLatestAutosave: %v", err)
	}
	if got.SaveName != "manual_save" {
		t.Fatalf("SaveName = %q", got.SaveName)
	}
}
