package template

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAllDefaultTemplatesValidateAndRender(t *testing.T) {
	defaults, err := LoadDefaults()
	if err != nil {
		t.Fatalf("LoadDefaults: %v", err)
	}
	if len(defaults) != 13 {
		t.Fatalf("defaults count = %d, want 13", len(defaults))
	}

	for key, tmpl := range defaults {
		t.Run(key, func(t *testing.T) {
			if err := ValidateMessageType(key, tmpl); err != nil {
				t.Fatalf("ValidateMessageType: %v", err)
			}

			vars := SampleVariables(key)
			rendered := Render(tmpl, vars)

			if rendered.Plain == "" {
				t.Fatal("plain render is empty")
			}
			if strings.Contains(rendered.Plain, "{") {
				t.Fatalf("plain still contains placeholders: %q", rendered.Plain)
			}
			if rendered.Embed == nil {
				t.Fatal("embed render is nil")
			}
			if strings.Contains(rendered.Embed.Title, "{") ||
				strings.Contains(rendered.Embed.Description, "{") {
				t.Fatalf("embed still contains placeholders: %+v", rendered.Embed)
			}
			for _, f := range rendered.Embed.Fields {
				if strings.Contains(f.Value, "{") || strings.Contains(f.Name, "{") {
					t.Fatalf("field still contains placeholders: %+v", f)
				}
			}
		})
	}
}

func TestSubstitute(t *testing.T) {
	vars := map[string]string{"PlayerName": "Guggi", "OnlineCount": "3"}
	got := Substitute("🟢 **{PlayerName}** joined ({OnlineCount} online)", vars)
	want := "🟢 **Guggi** joined (3 online)"
	if got != want {
		t.Fatalf("Substitute() = %q, want %q", got, want)
	}

	empty := Substitute("Phase {PhaseNumber} complete", map[string]string{"PhaseNumber": ""})
	if empty != "Phase  complete" {
		t.Fatalf("empty var = %q", empty)
	}
}

func TestExtractVariables(t *testing.T) {
	tmpl := Template{
		Plain: "{PlayerName}",
		Embed: &EmbedTemplate{
			Title:  "{PlayerName}",
			Fields: []EmbedField{{Name: "Count", Value: "{OnlineCount}"}},
		},
	}
	got := ExtractVariables(tmpl)
	want := []string{"OnlineCount", "PlayerName"}
	if len(got) != len(want) {
		t.Fatalf("ExtractVariables() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ExtractVariables() = %v, want %v", got, want)
		}
	}
}

func TestMergePartialOverride(t *testing.T) {
	defaults := Template{
		Plain: "default plain",
		Embed: &EmbedTemplate{Title: "default title"},
	}

	overrideJSON := []byte(`{"embed":{"title":"custom","description":"desc","color":"#000","fields":[]}}`)
	merged, err := Merge(defaults, overrideJSON)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if merged.Plain != "default plain" {
		t.Fatalf("plain = %q, want default", merged.Plain)
	}
	if merged.Embed == nil || merged.Embed.Title != "custom" {
		t.Fatalf("embed not overridden: %+v", merged.Embed)
	}
}

func TestRenderOmitsEmptyEmbedFields(t *testing.T) {
	tmpl := Template{
		Embed: &EmbedTemplate{
			Title:       "Elevator",
			Description: "Phase at {ElevatorName}",
			Color:       "#FEE75C",
			Fields: []EmbedField{
				{Name: "Phase", Value: "{PhaseNumber}", Inline: true},
				{Name: "Location", Value: "{ElevatorName}", Inline: true},
			},
		},
	}

	rendered := Render(tmpl, map[string]string{
		"ElevatorName": "Space Elevator",
		"PhaseNumber":  "",
	})
	if len(rendered.Embed.Fields) != 1 {
		t.Fatalf("fields = %+v, want only Location", rendered.Embed.Fields)
	}
	if rendered.Embed.Fields[0].Name != "Location" {
		t.Fatalf("field name = %q", rendered.Embed.Fields[0].Name)
	}
}

func TestValidateUnknownVariable(t *testing.T) {
	tmpl := Template{Plain: "Hello {UnknownVar}"}
	err := Validate(tmpl, []string{"ServerName"}, map[string]string{"ServerName": "x"})
	if err == nil {
		t.Fatal("expected validation error")
	}
	ve, ok := err.(*ValidationError)
	if !ok || ve.Code != "unknown_variable" {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateDiscordLimits(t *testing.T) {
	long := strings.Repeat("x", 257)
	tmpl := Template{
		Embed: &EmbedTemplate{
			Title:       long,
			Description: "ok",
			Color:       "#000",
		},
	}
	err := Validate(tmpl, []string{}, map[string]string{})
	if err == nil {
		t.Fatal("expected discord limit error")
	}
	ve, ok := err.(*ValidationError)
	if !ok || ve.Code != "discord_limit" {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateMessageTypeWithMergedOverride(t *testing.T) {
	defaults, err := LoadDefaults()
	if err != nil {
		t.Fatalf("LoadDefaults: %v", err)
	}

	base := defaults["player_joined"]
	override := []byte(`{"plain":"👋 {PlayerName} is here ({OnlineCount})"}`)
	merged, err := Merge(base, override)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if err := ValidateMessageType("player_joined", merged); err != nil {
		t.Fatalf("ValidateMessageType merged: %v", err)
	}
}

func TestLoadDefaultsMatchesJSONShape(t *testing.T) {
	defaults, err := LoadDefaults()
	if err != nil {
		t.Fatalf("LoadDefaults: %v", err)
	}

	for key, tmpl := range defaults {
		if tmpl.Plain == "" {
			t.Fatalf("%s: missing plain", key)
		}
		if tmpl.Embed == nil {
			t.Fatalf("%s: missing embed", key)
		}
		raw, err := json.Marshal(tmpl)
		if err != nil {
			t.Fatalf("marshal %s: %v", key, err)
		}
		var roundTrip Template
		if err := json.Unmarshal(raw, &roundTrip); err != nil {
			t.Fatalf("unmarshal %s: %v", key, err)
		}
	}
}
