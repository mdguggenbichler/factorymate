package discord

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"factorymate/internal/connection"
	"factorymate/internal/mods"
	"factorymate/internal/registration"

	"github.com/bwmarrin/discordgo"
)

// InvitePermissions matches §15.2 bot permissions bitmask.
const InvitePermissions = discordgo.PermissionViewChannel |
	discordgo.PermissionSendMessages |
	discordgo.PermissionEmbedLinks |
	discordgo.PermissionUseSlashCommands |
	discordgo.PermissionSendMessagesInThreads

// Channel is a guild text channel exposed to the admin API.
type Channel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Bot manages the discordgo gateway session (§5.1, §15).
type Bot struct {
	db           *sql.DB
	token        string
	session      *discordgo.Session
	registration *registration.Service
	connection   *connection.Service
	mods         *mods.Service
}

// NewBot constructs a bot from env. token may be empty (soft dependency).
func NewBot(db *sql.DB, regSvc *registration.Service, connSvc *connection.Service, modsSvc *mods.Service) (*Bot, error) {
	token := strings.TrimSpace(os.Getenv("DISCORD_BOT_TOKEN"))
	return &Bot{
		db:           db,
		token:        token,
		registration: regSvc,
		connection:   connSvc,
		mods:         modsSvc,
	}, nil
}

// Connected reports whether the gateway session is open.
func (b *Bot) Connected() bool {
	return b.session != nil
}

// Session returns the live discordgo session for notify.DiscordSession adapters.
func (b *Bot) Session() *discordgo.Session {
	return b.session
}

// Start opens the gateway when a token is configured and bot_enabled is true.
func (b *Bot) Start(ctx context.Context) error {
	if b.token == "" {
		log.Printf("discord bot: DISCORD_BOT_TOKEN unset — bot features disabled")
		return nil
	}

	enabled, err := BotEnabled(ctx, b.db)
	if err != nil {
		return fmt.Errorf("discord bot_enabled: %w", err)
	}
	if !enabled {
		log.Printf("discord bot: kill switch off — bot features disabled")
		return nil
	}

	dg, err := discordgo.New("Bot " + b.token)
	if err != nil {
		return fmt.Errorf("discord session: %w", err)
	}
	dg.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMembers
	dg.AddHandler(b.handleInteraction)
	dg.AddHandler(b.handleGuildMemberUpdate)

	if err := dg.Open(); err != nil {
		return fmt.Errorf("discord gateway: %w", err)
	}
	b.session = dg
	log.Printf("discord bot: connected as %s", dg.State.User.Username)

	if err := b.registerSlashCommands(ctx); err != nil {
		log.Printf("discord bot: register commands: %v", err)
	}

	go func() {
		<-ctx.Done()
		b.Stop()
	}()
	return nil
}

// Stop closes the gateway session.
func (b *Bot) Stop() {
	if b.session == nil {
		return
	}
	if err := b.session.Close(); err != nil {
		log.Printf("discord bot: close: %v", err)
	}
	b.session = nil
}

// InviteURL builds the OAuth2 invite link for the configured application.
func (b *Bot) InviteURL() (string, error) {
	if b.session == nil || b.session.State.Application == nil {
		return "", fmt.Errorf("discord bot is not connected")
	}
	clientID := b.session.State.Application.ID
	return fmt.Sprintf(
		"https://discord.com/api/oauth2/authorize?client_id=%s&permissions=%d&scope=bot%%20applications.commands",
		clientID, InvitePermissions,
	), nil
}

// ListGuildTextChannels returns text channels for the effective guild ID.
func (b *Bot) ListGuildTextChannels(ctx context.Context) ([]Channel, error) {
	if b.session == nil {
		return nil, fmt.Errorf("discord bot is not connected")
	}

	enabled, err := BotEnabled(ctx, b.db)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return nil, fmt.Errorf("discord bot is disabled")
	}

	guildID, err := EffectiveGuildID(ctx, b.db)
	if err != nil {
		return nil, err
	}
	if guildID == "" {
		return nil, fmt.Errorf("discord guild_id is not configured")
	}

	channels, err := b.session.GuildChannels(guildID, discordgo.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("list guild channels: %w", err)
	}

	out := make([]Channel, 0)
	for _, ch := range channels {
		if ch.Type != discordgo.ChannelTypeGuildText && ch.Type != discordgo.ChannelTypeGuildNews {
			continue
		}
		out = append(out, Channel{ID: ch.ID, Name: ch.Name})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
