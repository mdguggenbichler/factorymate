package discord

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

// SlashCommandsForTest exposes the registered slash command tree for validation tests.
func SlashCommandsForTest() []*discordgo.ApplicationCommand {
	return slashCommands()
}

// ValidateApplicationCommands ensures Discord slash command trees do not mix
// subcommands with regular options at the same level.
func ValidateApplicationCommands(commands []*discordgo.ApplicationCommand) error {
	for _, cmd := range commands {
		if err := validateCommandOptions(cmd.Name, cmd.Options); err != nil {
			return err
		}
	}
	return nil
}

func validateCommandOptions(commandName string, options []*discordgo.ApplicationCommandOption) error {
	if len(options) == 0 {
		return nil
	}

	hasSub := false
	hasOther := false
	for _, opt := range options {
		switch opt.Type {
		case discordgo.ApplicationCommandOptionSubCommand, discordgo.ApplicationCommandOptionSubCommandGroup:
			hasSub = true
		default:
			hasOther = true
		}
	}
	if hasSub && hasOther {
		return fmt.Errorf("command %q: subcommand options cannot be mixed with other option types", commandName)
	}

	for _, opt := range options {
		if err := validateNestedOptions(commandName, opt); err != nil {
			return err
		}
	}
	return nil
}

func validateNestedOptions(commandName string, opt *discordgo.ApplicationCommandOption) error {
	if len(opt.Options) == 0 {
		return nil
	}
	if opt.Type != discordgo.ApplicationCommandOptionSubCommand && opt.Type != discordgo.ApplicationCommandOptionSubCommandGroup {
		return fmt.Errorf("command %q option %q: only subcommands may have nested options", commandName, opt.Name)
	}
	return validateCommandOptions(commandName+" "+opt.Name, opt.Options)
}
