package discord

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"factorymate/internal/auth"
	"factorymate/internal/notify"
	"factorymate/internal/registration"

	"github.com/bwmarrin/discordgo"
)

const (
	modalRejectReason = "modal_reject:"
	btnRegApprove     = "btn_reg_approve:"
	btnRegReject      = "btn_reg_reject:"
)

type deferredInteractionKey struct{}

func interactionDeferred(ctx context.Context) bool {
	v, _ := ctx.Value(deferredInteractionKey{}).(bool)
	return v
}

func withDeferred(ctx context.Context) context.Context {
	return context.WithValue(ctx, deferredInteractionKey{}, true)
}

func deferEphemeral(s *discordgo.Session, i *discordgo.InteractionCreate) bool {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Printf("discord bot: defer ephemeral: %v", err)
		return false
	}
	return true
}

func (b *Bot) handleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	enabled, err := BotEnabled(ctx, b.db)
	if err != nil {
		log.Printf("discord bot: bot_enabled check: %v", err)
		return
	}
	if !enabled {
		respondEphemeral(ctx, s, i, "Discord bot is currently disabled.")
		return
	}

	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		b.handleApplicationCommand(ctx, s, i)
	case discordgo.InteractionModalSubmit:
		b.handleModalSubmit(ctx, s, i)
	case discordgo.InteractionMessageComponent:
		b.handleMessageComponent(ctx, s, i)
	}
}

func (b *Bot) handleApplicationCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()
	cmdName := data.Name
	if deferEphemeral(s, i) {
		ctx = withDeferred(ctx)
	}

	externalID := interactionUserID(i)
	memberRoles := memberRoleIDs(i)

	state, user, err := b.linkState(ctx, externalID)
	if err != nil {
		log.Printf("discord bot: link state: %v", err)
		respondEphemeral(ctx, s, i, "Something went wrong. Please try again.")
		return
	}

	perms, err := ResolveMemberPermissions(ctx, b.db, memberRoles)
	if err != nil {
		log.Printf("discord bot: permissions: %v", err)
		respondEphemeral(ctx, s, i, "Something went wrong. Please try again.")
		return
	}

	switch cmdName {
	case "help":
		_ = LogBotCommand(ctx, b.db, externalID, "help", true, "")
		respondEphemeral(ctx, s, i, formatHelpMessage())
		return
	case "whoami":
		b.handleWhoami(ctx, s, i, externalID, user, state)
		return
	case "register":
		if !CanRunCommand(perms, CommandGroupRegister, state) {
			b.logAndDeny(ctx, s, i, externalID, "register", "forbidden")
			return
		}
		if state != LinkStateUnregistered {
			respondEphemeral(ctx, s, i, "You are already registered. Use /whoami to check your status.")
			_ = LogBotCommand(ctx, b.db, externalID, "register", false, "already registered")
			return
		}
		b.handleRegisterOAuth(ctx, s, i, externalID, interactionMember(i), false)
	case "register-user":
		if !CanRunAdminCommand(perms, state, user) {
			b.logAndDeny(ctx, s, i, externalID, "register-user", "forbidden")
			return
		}
		var target *discordgo.User
		for _, opt := range data.Options {
			if opt.Name == "user" {
				target = opt.UserValue(s)
			}
		}
		if target == nil {
			respondEphemeral(ctx, s, i, "Could not resolve target user.")
			return
		}
		b.handleRegisterUser(ctx, s, i, target)
	case "set-player":
		if !CanRunCommand(perms, CommandGroupPlayer, state) {
			b.logAndDeny(ctx, s, i, externalID, "set-player", "forbidden")
			return
		}
		if user == nil {
			respondEphemeral(ctx, s, i, "You must be registered first. Use /register.")
			return
		}
		name := ""
		for _, opt := range data.Options {
			if opt.Name == "name" {
				name = opt.StringValue()
			}
		}
		b.handleSetPlayer(ctx, s, i, externalID, user.ID, name)
	case "clear-player":
		if !CanRunCommand(perms, CommandGroupPlayer, state) {
			b.logAndDeny(ctx, s, i, externalID, "clear-player", "forbidden")
			return
		}
		if user == nil {
			respondEphemeral(ctx, s, i, "You must be registered first. Use /register.")
			return
		}
		b.handleClearPlayer(ctx, s, i, externalID, user.ID)
	case "connection":
		b.handleConnectionCommand(ctx, s, i, data, externalID, perms, state, user)
	case "mods":
		b.handleModsCommand(ctx, s, i, data, externalID, perms, state)
	case "registration":
		if len(data.Options) == 0 || data.Options[0].Name != "auto-approve" {
			respondEphemeral(ctx, s, i, "Unknown subcommand.")
			return
		}
		if !CanRunAdminCommand(perms, state, user) {
			b.logAndDeny(ctx, s, i, externalID, "registration auto-approve", "forbidden")
			return
		}
		enabled := ""
		for _, opt := range data.Options[0].Options {
			if opt.Name == "enabled" {
				enabled = opt.StringValue()
			}
		}
		on := enabled == "on"
		if err := b.registration.SetAutoApprove(ctx, on); err != nil {
			respondEphemeral(ctx, s, i, "Failed to update setting.")
			_ = LogBotCommand(ctx, b.db, externalID, "registration auto-approve", false, err.Error())
			return
		}
		msg := "Auto-approve registrations is now **off**."
		if on {
			msg = "Auto-approve registrations is now **on**."
		}
		respondEphemeral(ctx, s, i, msg)
		_ = LogBotCommand(ctx, b.db, externalID, "registration auto-approve", true, enabled)
	case "registrations":
		b.handleRegistrationsCommand(ctx, s, i, data, externalID, perms, state, user)
	case "unlink":
		b.handleUnlinkCommand(ctx, s, i, data, externalID, perms, state, user)
	case "password-reset":
		b.handlePasswordResetCommand(ctx, s, i, data, externalID, perms, state, user)
	case "status":
		if !CanRunCommand(perms, CommandGroupPlayer, state) {
			b.logAndDeny(ctx, s, i, externalID, "status", "forbidden")
			return
		}
		b.handleStatusCommand(ctx, s, i, externalID)
	case "players":
		if !CanRunCommand(perms, CommandGroupPlayer, state) {
			b.logAndDeny(ctx, s, i, externalID, "players", "forbidden")
			return
		}
		b.handlePlayersCommand(ctx, s, i, externalID)
	case "broadcast":
		if !CanRunAdminCommand(perms, state, user) {
			b.logAndDeny(ctx, s, i, externalID, "broadcast", "forbidden")
			return
		}
		message := ""
		for _, opt := range data.Options {
			if opt.Name == "message" {
				message = opt.StringValue()
			}
		}
		b.handleBroadcastCommand(ctx, s, i, externalID, message)
	case "sync-roles":
		if !CanRunAdminCommand(perms, state, user) {
			b.logAndDeny(ctx, s, i, externalID, "sync-roles", "forbidden")
			return
		}
		b.handleSyncRolesCommand(ctx, s, i, externalID)
	case "notifications":
		if !CanRunCommand(perms, CommandGroupPlayer, state) {
			b.logAndDeny(ctx, s, i, externalID, "notifications", "forbidden")
			return
		}
		if user == nil {
			respondEphemeral(ctx, s, i, "You must be registered first.")
			return
		}
		b.handleNotificationsCommand(ctx, s, i, externalID, user.ID, data)
	default:
		respondEphemeral(ctx, s, i, "Unknown command.")
	}
}

func (b *Bot) handleWhoami(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, externalID string, user *auth.User, state LinkState) {
	var lines []string
	switch state {
	case LinkStateUnregistered:
		lines = []string{"Status: **not registered**", "Use /register to get a dashboard link and finish registration."}
	case LinkStatePendingApproval:
		lines = []string{"Status: **pending approval**", "An admin must approve your registration before you can log in."}
	case LinkStateActiveNotLinked:
		lines = []string{"Status: **active (not linked)** — this should not happen for Discord users."}
	case LinkStateActiveLinked:
		player := "—"
		if user.PlayerName != nil && *user.PlayerName != "" {
			player = *user.PlayerName
		} else if user.PendingPlayerName != nil && *user.PendingPlayerName != "" {
			player = *user.PendingPlayerName + " (pending)"
		}
		lines = []string{
			fmt.Sprintf("FM username: **%s**", user.Username),
			fmt.Sprintf("Role: **%s**", user.Role),
			fmt.Sprintf("Player: **%s**", player),
			"Status: **active**",
		}
	}
	respondEphemeral(ctx, s, i, strings.Join(lines, "\n"))
	_ = LogBotCommand(ctx, b.db, externalID, "whoami", true, "")
}

func (b *Bot) handleRegisterOAuth(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, externalID string, member *discordgo.Member, forceApprove bool) {
	authSvc := auth.NewService(b.db)
	role, err := FMRoleForMember(ctx, b.db, memberRoleIDsFromMember(member))
	if err != nil {
		respondEphemeral(ctx, s, i, "Something went wrong.")
		_ = LogBotCommand(ctx, b.db, externalID, "register", false, err.Error())
		return
	}

	ext := externalFromMember(member, externalID)
	oauthURL, err := buildRegisterOAuthURL(ctx, authSvc, auth.OAuthStateMeta{
		ExternalUserID:      ext.UserID,
		ExternalUsername:    ext.Username,
		ExternalDisplayName: ext.DisplayName,
		ForceApprove:        forceApprove,
		Role:                role,
	})
	if err != nil {
		respondEphemeral(ctx, s, i, formatRegisterOAuthMessage(""))
		_ = LogBotCommand(ctx, b.db, externalID, "register", false, err.Error())
		return
	}

	dmText := formatRegisterOAuthMessage(oauthURL)
	provider := notify.NewDiscordProvider(s)
	if err := provider.SendDirect(ctx, registration.PlatformDiscord, externalID, notify.RenderedMessage{Plain: dmText}); err != nil {
		respondEphemeral(ctx, s, i, dmText+"\n\n(Could not DM you — DMs may be disabled.)")
		_ = LogBotCommand(ctx, b.db, externalID, "register", true, "ephemeral fallback")
		return
	}

	respondEphemeral(ctx, s, i, "Check your DMs for the registration link.")
	_ = LogBotCommand(ctx, b.db, externalID, "register", true, "oauth dm sent")
}

func (b *Bot) handleRegisterUser(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, target *discordgo.User) {
	if target == nil {
		respondEphemeral(ctx, s, i, "Could not resolve target user.")
		return
	}
	if target.Bot {
		respondEphemeral(ctx, s, i, "Cannot register bots.")
		return
	}

	existing, err := b.registration.GetByExternal(ctx, registration.PlatformDiscord, target.ID)
	if err != nil {
		respondEphemeral(ctx, s, i, "Something went wrong.")
		return
	}
	if existing != nil {
		respondEphemeral(ctx, s, i, "That user is already registered.")
		return
	}

	authSvc := auth.NewService(b.db)
	role, err := FMRoleForMember(ctx, b.db, []string{})
	if err != nil {
		role = auth.RoleViewer
	}
	oauthURL, err := buildRegisterOAuthURL(ctx, authSvc, auth.OAuthStateMeta{
		ExternalUserID:      target.ID,
		ExternalUsername:    target.Username,
		ExternalDisplayName: target.Username,
		ForceApprove:        true,
		Role:                role,
	})
	if err != nil {
		respondEphemeral(ctx, s, i, "Discord sign-in is not configured on this server.")
		_ = LogBotCommand(ctx, b.db, interactionUserID(i), "register-user", false, err.Error())
		return
	}

	dmText := "An admin invited you to join FactoryMate.\n\n" + formatRegisterOAuthMessage(oauthURL)
	provider := notify.NewDiscordProvider(s)
	if err := provider.SendDirect(ctx, registration.PlatformDiscord, target.ID, notify.RenderedMessage{Plain: dmText}); err != nil {
		respondEphemeral(ctx, s, i, "Could not DM the user. They may have DMs disabled.")
		_ = LogBotCommand(ctx, b.db, interactionUserID(i), "register-user", false, "dm failed")
		return
	}

	respondEphemeral(ctx, s, i, fmt.Sprintf("Registration link sent to <@%s>.", target.ID))
	_ = LogBotCommand(ctx, b.db, interactionUserID(i), "register-user", true, target.ID)
}

func (b *Bot) handleSetPlayer(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, externalID string, userID int64, name string) {
	updated, err := b.registration.SetPlayerName(ctx, userID, name)
	if err != nil {
		msg := "Could not update player mapping."
		if errors.Is(err, registration.ErrPlayerAlreadyLinked) {
			msg = "That player is already linked to another user."
		}
		respondEphemeral(ctx, s, i, msg)
		_ = LogBotCommand(ctx, b.db, externalID, "set-player", false, err.Error())
		return
	}
	player := playerDisplayLine(updated.PlayerName, name, updated.PlayerID != nil)
	respondEphemeral(ctx, s, i, fmt.Sprintf("Player mapping updated: **%s**", player))
	_ = LogBotCommand(ctx, b.db, externalID, "set-player", true, name)
}

func (b *Bot) handleClearPlayer(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, externalID string, userID int64) {
	updated, err := b.registration.ClearPlayerMapping(ctx, userID)
	if err != nil {
		msg := "Could not clear player mapping."
		if errors.Is(err, registration.ErrUserNotFound) {
			msg = "User not found."
		}
		respondEphemeral(ctx, s, i, msg)
		_ = LogBotCommand(ctx, b.db, externalID, "clear-player", false, err.Error())
		return
	}
	if updated.PlayerID != nil || (updated.PendingPlayerName != nil && *updated.PendingPlayerName != "") {
		respondEphemeral(ctx, s, i, "Player mapping could not be cleared.")
		_ = LogBotCommand(ctx, b.db, externalID, "clear-player", false, "still mapped")
		return
	}
	respondEphemeral(ctx, s, i, "Player mapping cleared.")
	_ = LogBotCommand(ctx, b.db, externalID, "clear-player", true, "")
}

func (b *Bot) handleModalSubmit(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
	if deferEphemeral(s, i) {
		ctx = withDeferred(ctx)
	}
	customID := i.ModalSubmitData().CustomID
	externalID := interactionUserID(i)

	switch {
	case strings.HasPrefix(customID, modalRejectReason):
		b.submitRejectModal(ctx, s, i, customID, externalID)
	default:
		respondEphemeral(ctx, s, i, "Unknown modal.")
	}
}

func (b *Bot) handleMessageComponent(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
	customID := i.MessageComponentData().CustomID
	externalID := interactionUserID(i)

	if !strings.HasPrefix(customID, btnRegReject) {
		if deferEphemeral(s, i) {
			ctx = withDeferred(ctx)
		}
	}

	switch {
	case strings.HasPrefix(customID, btnRegApprove):
		b.handleApproveButton(ctx, s, i, customID, externalID)
	case strings.HasPrefix(customID, btnRegReject):
		b.handleRejectButton(ctx, s, i, customID, externalID)
	default:
		respondEphemeral(ctx, s, i, "Unknown button.")
	}
}

func (b *Bot) handleApproveButton(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, customID, externalID string) {
	userID, err := parseUserIDFromCustomID(customID, btnRegApprove)
	if err != nil {
		respondEphemeral(ctx, s, i, "Invalid approval request.")
		return
	}

	adminUser, err := b.registration.GetByExternal(ctx, registration.PlatformDiscord, externalID)
	if err != nil || adminUser == nil || adminUser.Role != auth.RoleAdmin {
		respondEphemeral(ctx, s, i, "Only admins can approve registrations.")
		_ = LogBotCommand(ctx, b.db, externalID, "registration approve", false, "forbidden")
		return
	}

	expired, err := b.registration.RegistrationButtonExpired(ctx, userID)
	if err != nil {
		respondEphemeral(ctx, s, i, "Could not verify registration age.")
		_ = LogBotCommand(ctx, b.db, externalID, "registration approve", false, err.Error())
		return
	}
	if expired {
		respondEphemeral(ctx, s, i, "Button expired — use web UI or `/registrations approve`.")
		_ = LogBotCommand(ctx, b.db, externalID, "registration approve", false, "expired")
		return
	}

	approved, err := b.registration.ApproveRegistration(ctx, userID, adminUser.ID)
	if err != nil {
		msg := "Could not approve registration."
		if errors.Is(err, registration.ErrNotPendingApproval) {
			msg = "This registration is no longer pending."
		}
		respondEphemeral(ctx, s, i, msg)
		_ = LogBotCommand(ctx, b.db, externalID, "registration approve", false, err.Error())
		return
	}

	respondEphemeral(ctx, s, i, fmt.Sprintf("Approved **%s**.", approved.Username))
	if extID := approvedExternalID(approved); extID != "" {
		b.SendWelcomeDM(ctx, extID, approved.Username)
	}
	_ = LogBotCommand(ctx, b.db, externalID, "registration approve", true, strconv.FormatInt(userID, 10))
}

func (b *Bot) handleRejectButton(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, customID, externalID string) {
	userID, err := parseUserIDFromCustomID(customID, btnRegReject)
	if err != nil {
		respondEphemeral(ctx, s, i, "Invalid rejection request.")
		return
	}

	adminUser, err := b.registration.GetByExternal(ctx, registration.PlatformDiscord, externalID)
	if err != nil || adminUser == nil || adminUser.Role != auth.RoleAdmin {
		respondEphemeral(ctx, s, i, "Only admins can reject registrations.")
		return
	}

	expired, err := b.registration.RegistrationButtonExpired(ctx, userID)
	if err != nil {
		respondEphemeral(ctx, s, i, "Could not verify registration age.")
		return
	}
	if expired {
		respondEphemeral(ctx, s, i, "Button expired — use web UI or `/registrations reject`.")
		return
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: modalRejectReason + strconv.FormatInt(userID, 10),
			Title:    "Reject registration",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "reason",
							Label:       "Reason (optional)",
							Style:       discordgo.TextInputParagraph,
							Required:    false,
							MaxLength:   500,
							Placeholder: "Optional message to the registrant",
						},
					},
				},
			},
		},
	})
	if err != nil {
		log.Printf("discord bot: reject modal: %v", err)
	}
	_ = LogBotCommand(ctx, b.db, externalID, "registration reject", true, "modal opened")
}

func (b *Bot) submitRejectModal(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, customID, externalID string) {
	userID, err := parseUserIDFromCustomID(customID, modalRejectReason)
	if err != nil {
		respondEphemeral(ctx, s, i, "Invalid rejection request.")
		return
	}

	adminUser, err := b.registration.GetByExternal(ctx, registration.PlatformDiscord, externalID)
	if err != nil || adminUser == nil || adminUser.Role != auth.RoleAdmin {
		respondEphemeral(ctx, s, i, "Only admins can reject registrations.")
		return
	}

	reason := modalValues(i)["reason"]
	targetExternalID, err := b.registration.RejectRegistration(ctx, userID, adminUser.ID, reason)
	if err != nil {
		msg := "Could not reject registration."
		if errors.Is(err, registration.ErrNotPendingApproval) {
			msg = "This registration is no longer pending."
		}
		respondEphemeral(ctx, s, i, msg)
		_ = LogBotCommand(ctx, b.db, externalID, "registration reject", false, err.Error())
		return
	}

	respondEphemeral(ctx, s, i, "Registration rejected.")
	if targetExternalID != "" {
		b.SendRegistrationDeclinedDM(ctx, targetExternalID, reason)
	}
	_ = LogBotCommand(ctx, b.db, externalID, "registration reject", true, strconv.FormatInt(userID, 10))
}

func (b *Bot) notifyAdminsPending(ctx context.Context, s *discordgo.Session, user auth.User, playerName string, member *discordgo.Member) error {
	admins, err := b.registration.ListAdminsWithExternal(ctx)
	if err != nil {
		return err
	}
	discordName := memberDisplay(member)
	embed := &discordgo.MessageEmbed{
		Title: "📋 New registration pending approval",
		Description: fmt.Sprintf(
			"Discord:   @%s (%s)\nIn-game:   %s\nFM user:   %s\nSubmitted: %s",
			discordName, memberDisplayName(member), playerName, user.Username, time.Now().UTC().Format("Jan 2, 2006 · 15:04 MST"),
		),
	}
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "Approve",
					Style:    discordgo.SuccessButton,
					CustomID: btnRegApprove + strconv.FormatInt(user.ID, 10),
				},
				discordgo.Button{
					Label:    "Reject",
					Style:    discordgo.DangerButton,
					CustomID: btnRegReject + strconv.FormatInt(user.ID, 10),
				},
			},
		},
	}

	for _, admin := range admins {
		extID := approvedExternalID(admin)
		if extID == "" {
			continue
		}
		ch, err := s.UserChannelCreate(extID)
		if err != nil {
			log.Printf("discord bot: admin dm channel: %v", err)
			continue
		}
		_, err = s.ChannelMessageSendComplex(ch.ID, &discordgo.MessageSend{
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: components,
		})
		if err != nil {
			log.Printf("discord bot: admin notify dm: %v", err)
		}
	}
	return nil
}

func (b *Bot) linkState(ctx context.Context, externalID string) (LinkState, *auth.User, error) {
	user, err := b.registration.GetByExternal(ctx, registration.PlatformDiscord, externalID)
	if err != nil {
		return LinkStateUnregistered, nil, err
	}
	if user == nil {
		return LinkStateUnregistered, nil, nil
	}
	switch user.Status {
	case auth.StatusPendingApproval:
		return LinkStatePendingApproval, user, nil
	case auth.StatusActive:
		if user.External.UserID != nil && *user.External.UserID != "" {
			return LinkStateActiveLinked, user, nil
		}
		return LinkStateActiveNotLinked, user, nil
	default:
		return LinkStateUnregistered, user, nil
	}
}

func (b *Bot) logAndDeny(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, externalID, command, detail string) {
	respondEphemeral(ctx, s, i, "You don't have permission to run this command.")
	_ = LogBotCommand(ctx, b.db, externalID, command, false, detail)
}

func respondEphemeral(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	if interactionDeferred(ctx) {
		editEphemeral(s, i, content)
		return
	}
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Printf("discord bot: ephemeral respond: %v", err)
	}
}

func editEphemeral(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	_, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: &content})
	if err != nil {
		log.Printf("discord bot: ephemeral edit: %v", err)
	}
}

func modalValues(i *discordgo.InteractionCreate) map[string]string {
	out := make(map[string]string)
	for _, row := range i.ModalSubmitData().Components {
		actionRow, ok := row.(*discordgo.ActionsRow)
		if !ok {
			continue
		}
		for _, c := range actionRow.Components {
			if input, ok := c.(*discordgo.TextInput); ok {
				out[input.CustomID] = input.Value
			}
		}
	}
	return out
}

func interactionUserID(i *discordgo.InteractionCreate) string {
	if i.Member != nil && i.Member.User != nil {
		return i.Member.User.ID
	}
	if i.User != nil {
		return i.User.ID
	}
	return ""
}

func interactionMember(i *discordgo.InteractionCreate) *discordgo.Member {
	if i.Member != nil {
		return i.Member
	}
	if i.User != nil {
		return &discordgo.Member{User: i.User}
	}
	return nil
}

func memberRoleIDs(i *discordgo.InteractionCreate) []string {
	return memberRoleIDsFromMember(interactionMember(i))
}

func memberRoleIDsFromMember(member *discordgo.Member) []string {
	if member == nil {
		return nil
	}
	return member.Roles
}

func externalFromMember(member *discordgo.Member, externalID string) registration.ExternalIdentity {
	ext := registration.ExternalIdentity{
		Platform: registration.PlatformDiscord,
		UserID:   externalID,
	}
	if member != nil && member.User != nil {
		ext.Username = member.User.Username
		ext.DisplayName = memberDisplayName(member)
	}
	return ext
}

func memberDisplay(member *discordgo.Member) string {
	if member == nil || member.User == nil {
		return "unknown"
	}
	return member.User.Username
}

func memberDisplayName(member *discordgo.Member) string {
	if member == nil || member.User == nil {
		return "unknown"
	}
	if member.Nick != "" {
		return member.Nick
	}
	if member.User.GlobalName != "" {
		return member.User.GlobalName
	}
	return member.User.Username
}

func ParseUserIDFromCustomID(customID, prefix string) (int64, error) {
	return parseUserIDFromCustomID(customID, prefix)
}

func parseUserIDFromCustomID(customID, prefix string) (int64, error) {
	raw := strings.TrimPrefix(customID, prefix)
	return strconv.ParseInt(raw, 10, 64)
}

func approvedExternalID(user auth.User) string {
	if user.External.UserID != nil {
		return *user.External.UserID
	}
	return ""
}
