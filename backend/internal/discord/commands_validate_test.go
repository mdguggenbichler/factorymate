package discord_test

import (
	"testing"

	"factorymate/internal/discord"

	"github.com/bwmarrin/discordgo"
)

func TestValidateApplicationCommands_registeredTree(t *testing.T) {
	t.Parallel()
	if err := discord.ValidateApplicationCommands(discord.SlashCommandsForTest()); err != nil {
		t.Fatalf("registered slash commands invalid: %v", err)
	}
}

func TestValidateApplicationCommands_rejectsMixedOptions(t *testing.T) {
	t.Parallel()
	commands := []*discordgo.ApplicationCommand{
		{
			Name: "bad",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionBoolean, Name: "public"},
				{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "set"},
			},
		},
	}
	if err := discord.ValidateApplicationCommands(commands); err == nil {
		t.Fatal("expected mixed option validation error")
	}
}
