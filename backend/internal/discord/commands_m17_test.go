package discord_test

import (
	"testing"

	"factorymate/internal/discord"
)

func TestSlashCommands_noLinkCommand(t *testing.T) {
	t.Parallel()
	for _, cmd := range discord.SlashCommandsForTest() {
		if cmd.Name == "link" {
			t.Fatal("slash commands must not include /link (use Account → Link Discord)")
		}
	}
}

func TestSlashCommands_registerDescriptionMentionsOAuth(t *testing.T) {
	t.Parallel()
	for _, cmd := range discord.SlashCommandsForTest() {
		if cmd.Name == "register" {
			if cmd.Description == "" {
				t.Fatal("register command needs a description")
			}
			return
		}
	}
	t.Fatal("register command not found")
}
