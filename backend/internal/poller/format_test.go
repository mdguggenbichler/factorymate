package poller

import (
	"strings"
	"testing"

	"factorymate/internal/frm"
)

func TestFormatPhaseRequirementsDeliveredTotal(t *testing.T) {
	items := []frm.PhaseItem{
		{Name: "Smart Plating", RemainingCost: 0, TotalCost: 1000},
		{Name: "Versatile Framework", RemainingCost: 500, TotalCost: 1000},
	}
	got := formatPhaseRequirements(items)
	want := "Smart Plating: 1000/1000\nVersatile Framework: 500/1000"
	if got != want {
		t.Fatalf("formatPhaseRequirements = %q, want %q", got, want)
	}
	if strings.Contains(got, "Remaining") {
		t.Fatal("expected delivered/total format, not remaining")
	}
}
