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
	"factorymate/internal/registration"

	"github.com/bwmarrin/discordgo"
)

const (
	modalRegister      = "modal_register"
	modalRegisterAdmin = "modal_register_admin:"
	modalLink          = "modal_link"
	modalRejectReason  = "modal_reject:"
	btnCompleteReg     = "btn_complete_reg:"
	btnRegApprove      = "btn_reg_approve:"
	btnRegReject       = "btn_reg_reject:"
)

func (b *Bot) handleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	enabled, err := BotEnabled(ctx, b.db)
	if err != nil {
		log.Printf("discord bot: bot_enabled check: %v", err)
		return
	}
	if !enabled {
		respondEphemeral(s, i, "Discord bot is currently disabled.")
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
	externalID := interactionUserID(i)
	memberRoles := memberRoleIDs(i)

	state, user, err := b.linkState(ctx, externalID)
	if err != nil {
		log.Printf("discord bot: link state: %v", err)
		respondEphemeral(s, i, "Something went wrong. Please try again.")
		return
	}

	perms, err := ResolveMemberPermissions(ctx, b.db, memberRoles)
	if err != nil {
		log.Printf("discord bot: permissions: %v", err)
		respondEphemeral(s, i, "Something went wrong. Please try again.")
		return
	}

	switch cmdName {
	case "help":
		_ = LogBotCommand(ctx, b.db, externalID, "help", true, "")
		respondEphemeral(s, i, formatHelpMessage())
		return
	case "whoami":
		b.handleWhoami(ctx, s, i, externalID, user, state)
		return
	case "register":
		if len(data.Options) > 0 && data.Options[0].Name == "user" {
			if !CanRunCommand(perms, CommandGroupAdmin, state) {
				b.logAndDeny(ctx, s, i, externalID, "register user", "forbidden")
				return
			}
			b.handleRegisterUser(ctx, s, i, data.Options[0])
			return
		}
		if !CanRunCommand(perms, CommandGroupRegister, state) {
			b.logAndDeny(ctx, s, i, externalID, "register", "forbidden")
			return
		}
		if state != LinkStateUnregistered {
			respondEphemeral(s, i, "You are already registered. Use /whoami to check your status.")
			_ = LogBotCommand(ctx, b.db, externalID, "register", false, "already registered")
			return
		}
		showRegisterModal(s, i, modalRegister, "FactoryMate registration")
		_ = LogBotCommand(ctx, b.db, externalID, "register", true, "modal opened")
	case "link":
		if state != LinkStateUnregistered {
			respondEphemeral(s, i, "Your Discord is already linked. Use /whoami to check your status.")
			_ = LogBotCommand(ctx, b.db, externalID, "link", false, "already linked")
			return
		}
		showLinkModal(s, i)
		_ = LogBotCommand(ctx, b.db, externalID, "link", true, "modal opened")
	case "set-player":
		if !CanRunCommand(perms, CommandGroupPlayer, state) {
			b.logAndDeny(ctx, s, i, externalID, "set-player", "forbidden")
			return
		}
		if user == nil {
			respondEphemeral(s, i, "You must be registered first. Use /register or /link.")
			return
		}
		name := ""
		for _, opt := range data.Options {
			if opt.Name == "name" {
				name = opt.StringValue()
			}
		}
		b.handleSetPlayer(ctx, s, i, externalID, user.ID, name)
	case "connection":
		b.handleConnectionCommand(ctx, s, i, data, externalID, perms, state)
	case "mods":
		b.handleModsCommand(ctx, s, i, data, externalID, perms, state)
	case "registration":
		if len(data.Options) == 0 || data.Options[0].Name != "auto-approve" {
			respondEphemeral(s, i, "Unknown subcommand.")
			return
		}
		if !CanRunCommand(perms, CommandGroupAdmin, state) {
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
			respondEphemeral(s, i, "Failed to update setting.")
			_ = LogBotCommand(ctx, b.db, externalID, "registration auto-approve", false, err.Error())
			return
		}
		msg := "Auto-approve registrations is now **off**."
		if on {
			msg = "Auto-approve registrations is now **on**."
		}
		respondEphemeral(s, i, msg)
		_ = LogBotCommand(ctx, b.db, externalID, "registration auto-approve", true, enabled)
	default:
		respondEphemeral(s, i, "Unknown command.")
	}
}

func (b *Bot) handleWhoami(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, externalID string, user *auth.User, state LinkState) {
	var lines []string
	switch state {
	case LinkStateUnregistered:
		lines = []string{"Status: **not registered**", "Use /register to create an account or /link for an existing web account."}
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
	respondEphemeral(s, i, strings.Join(lines, "\n"))
	_ = LogBotCommand(ctx, b.db, externalID, "whoami", true, "")
}

func (b *Bot) handleRegisterUser(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, sub *discordgo.ApplicationCommandInteractionDataOption) {
	target := sub.Options[0].UserValue(s)
	if target == nil {
		respondEphemeral(s, i, "Could not resolve target user.")
		return
	}
	if target.Bot {
		respondEphemeral(s, i, "Cannot register bots.")
		return
	}

	existing, err := b.registration.GetByExternal(ctx, registration.PlatformDiscord, target.ID)
	if err != nil {
		respondEphemeral(s, i, "Something went wrong.")
		return
	}
	if existing != nil {
		respondEphemeral(s, i, "That user is already registered.")
		return
	}

	dmChannel, err := s.UserChannelCreate(target.ID)
	if err != nil {
		respondEphemeral(s, i, "Could not DM the user. They may have DMs disabled.")
		_ = LogBotCommand(ctx, b.db, interactionUserID(i), "register user", false, "dm failed")
		return
	}

	_, err = s.ChannelMessageSendComplex(dmChannel.ID, &discordgo.MessageSend{
		Content: "An admin invited you to join FactoryMate. Tap **Complete Registration** to continue.",
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "Complete Registration",
						Style:    discordgo.PrimaryButton,
						CustomID: btnCompleteReg + target.ID,
					},
				},
			},
		},
	})
	if err != nil {
		respondEphemeral(s, i, "Could not send invitation DM.")
		_ = LogBotCommand(ctx, b.db, interactionUserID(i), "register user", false, "dm send failed")
		return
	}

	respondEphemeral(s, i, fmt.Sprintf("Invitation sent to <@%s>.", target.ID))
	_ = LogBotCommand(ctx, b.db, interactionUserID(i), "register user", true, target.ID)
}

func (b *Bot) handleSetPlayer(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, externalID string, userID int64, name string) {
	updated, err := b.registration.SetPlayerName(ctx, userID, name)
	if err != nil {
		msg := "Could not update player mapping."
		if errors.Is(err, registration.ErrPlayerAlreadyLinked) {
			msg = "That player is already linked to another user."
		}
		respondEphemeral(s, i, msg)
		_ = LogBotCommand(ctx, b.db, externalID, "set-player", false, err.Error())
		return
	}
	player := playerDisplayLine(updated.PlayerName, name, updated.PlayerID != nil)
	respondEphemeral(s, i, fmt.Sprintf("Player mapping updated: **%s**", player))
	_ = LogBotCommand(ctx, b.db, externalID, "set-player", true, name)
}

func (b *Bot) handleModalSubmit(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
	customID := i.ModalSubmitData().CustomID
	externalID := interactionUserID(i)
	member := interactionMember(i)

	switch {
	case customID == modalRegister || strings.HasPrefix(customID, modalRegisterAdmin):
		b.submitRegisterModal(ctx, s, i, customID, externalID, member)
	case customID == modalLink:
		b.submitLinkModal(ctx, s, i, externalID, member)
	case strings.HasPrefix(customID, modalRejectReason):
		b.submitRejectModal(ctx, s, i, customID, externalID)
	default:
		respondEphemeral(s, i, "Unknown modal.")
	}
}

func (b *Bot) submitRegisterModal(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, customID, externalID string, member *discordgo.Member) {
	values := modalValues(i)
	username := values["fm_username"]
	password := values["password"]
	playerName := values["player_name"]
	forceApprove := strings.HasPrefix(customID, modalRegisterAdmin)

	if username == "" || password == "" || playerName == "" {
		respondEphemeral(s, i, "All fields are required.")
		return
	}

	role, err := FMRoleForMember(ctx, b.db, memberRoleIDsFromMember(member))
	if err != nil {
		respondEphemeral(s, i, "Something went wrong.")
		return
	}

	ext := externalFromMember(member, externalID)
	result, err := b.registration.Register(ctx, registration.RegisterParams{
		Username:          username,
		Password:          password,
		PendingPlayerName: playerName,
		External:          ext,
		Role:              role,
		ForceApprove:      forceApprove,
	})
	if err != nil {
		msg := "Registration failed."
		switch {
		case errors.Is(err, registration.ErrUsernameTaken):
			msg = "That username is already taken."
		case errors.Is(err, registration.ErrAlreadyRegistered):
			msg = "You are already registered."
		case errors.Is(err, auth.ErrWeakPassword):
			msg = err.Error()
		}
		respondEphemeral(s, i, msg)
		_ = LogBotCommand(ctx, b.db, externalID, "register", false, err.Error())
		return
	}

	if result.PendingApproval {
		respondEphemeral(s, i, formatRegistrationPendingMessage(playerName))
		if err := b.notifyAdminsPending(ctx, s, result.User, playerName, member); err != nil {
			log.Printf("discord bot: notify admins: %v", err)
		}
		_ = LogBotCommand(ctx, b.db, externalID, "register", true, "pending_approval")
		return
	}

	playerLine := playerDisplayLine(result.User.PlayerName, playerName, result.PlayerLinked)
	respondEphemeral(s, i, formatRegistrationApprovedMessage(result.User.Username, string(result.User.Role), playerLine))
	b.SendWelcomeDM(ctx, externalID, result.User.Username)
	_ = LogBotCommand(ctx, b.db, externalID, "register", true, "active")
}

func (b *Bot) submitLinkModal(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, externalID string, member *discordgo.Member) {
	values := modalValues(i)
	username := values["fm_username"]
	password := values["password"]
	if username == "" || password == "" {
		respondEphemeral(s, i, "Username and password are required.")
		return
	}

	ext := externalFromMember(member, externalID)
	linked, err := b.registration.LinkAccount(ctx, username, password, ext)
	if err != nil {
		msg := "Link failed. Check your credentials."
		switch {
		case errors.Is(err, auth.ErrInvalidCredentials):
			msg = "Invalid username or password."
		case errors.Is(err, auth.ErrPendingApproval):
			msg = "That account is pending approval."
		case errors.Is(err, registration.ErrAlreadyRegistered):
			msg = "This Discord account or FM user is already linked."
		}
		respondEphemeral(s, i, msg)
		_ = LogBotCommand(ctx, b.db, externalID, "link", false, err.Error())
		return
	}

	respondEphemeral(s, i, fmt.Sprintf("✅ Linked to **%s**. Use /whoami to check your status.", linked.Username))
	_ = LogBotCommand(ctx, b.db, externalID, "link", true, linked.Username)
}

func (b *Bot) handleMessageComponent(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
	customID := i.MessageComponentData().CustomID
	externalID := interactionUserID(i)

	switch {
	case strings.HasPrefix(customID, btnCompleteReg):
		targetID := strings.TrimPrefix(customID, btnCompleteReg)
		if externalID != targetID {
			respondEphemeral(s, i, "This invitation is not for you.")
			return
		}
		showRegisterModal(s, i, modalRegisterAdmin+targetID, "Complete FactoryMate registration")
	case strings.HasPrefix(customID, btnRegApprove):
		b.handleApproveButton(ctx, s, i, customID, externalID)
	case strings.HasPrefix(customID, btnRegReject):
		b.handleRejectButton(ctx, s, i, customID, externalID)
	default:
		respondEphemeral(s, i, "Unknown button.")
	}
}

func (b *Bot) handleApproveButton(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, customID, externalID string) {
	userID, err := parseUserIDFromCustomID(customID, btnRegApprove)
	if err != nil {
		respondEphemeral(s, i, "Invalid approval request.")
		return
	}

	adminUser, err := b.registration.GetByExternal(ctx, registration.PlatformDiscord, externalID)
	if err != nil || adminUser == nil || adminUser.Role != auth.RoleAdmin {
		respondEphemeral(s, i, "Only admins can approve registrations.")
		_ = LogBotCommand(ctx, b.db, externalID, "registration approve", false, "forbidden")
		return
	}

	approved, err := b.registration.ApproveRegistration(ctx, userID, adminUser.ID)
	if err != nil {
		msg := "Could not approve registration."
		if errors.Is(err, registration.ErrNotPendingApproval) {
			msg = "This registration is no longer pending."
		}
		respondEphemeral(s, i, msg)
		_ = LogBotCommand(ctx, b.db, externalID, "registration approve", false, err.Error())
		return
	}

	respondEphemeral(s, i, fmt.Sprintf("Approved **%s**.", approved.Username))
	if extID := approvedExternalID(approved); extID != "" {
		b.SendWelcomeDM(ctx, extID, approved.Username)
	}
	_ = LogBotCommand(ctx, b.db, externalID, "registration approve", true, strconv.FormatInt(userID, 10))
}

func (b *Bot) handleRejectButton(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, customID, externalID string) {
	userID, err := parseUserIDFromCustomID(customID, btnRegReject)
	if err != nil {
		respondEphemeral(s, i, "Invalid rejection request.")
		return
	}

	adminUser, err := b.registration.GetByExternal(ctx, registration.PlatformDiscord, externalID)
	if err != nil || adminUser == nil || adminUser.Role != auth.RoleAdmin {
		respondEphemeral(s, i, "Only admins can reject registrations.")
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
		respondEphemeral(s, i, "Invalid rejection request.")
		return
	}

	adminUser, err := b.registration.GetByExternal(ctx, registration.PlatformDiscord, externalID)
	if err != nil || adminUser == nil || adminUser.Role != auth.RoleAdmin {
		respondEphemeral(s, i, "Only admins can reject registrations.")
		return
	}

	reason := modalValues(i)["reason"]
	targetExternalID, err := b.registration.RejectRegistration(ctx, userID, adminUser.ID, reason)
	if err != nil {
		msg := "Could not reject registration."
		if errors.Is(err, registration.ErrNotPendingApproval) {
			msg = "This registration is no longer pending."
		}
		respondEphemeral(s, i, msg)
		_ = LogBotCommand(ctx, b.db, externalID, "registration reject", false, err.Error())
		return
	}

	respondEphemeral(s, i, "Registration rejected.")
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
		return LinkStateActiveLinked, user, nil
	}
}

func (b *Bot) logAndDeny(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, externalID, command, detail string) {
	respondEphemeral(s, i, "You don't have permission to run this command.")
	_ = LogBotCommand(ctx, b.db, externalID, command, false, detail)
}

func showRegisterModal(s *discordgo.Session, i *discordgo.InteractionCreate, customID, title string) {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: customID,
			Title:    title,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{Components: []discordgo.MessageComponent{discordgo.TextInput{
					CustomID: "fm_username", Label: "FactoryMate username", Style: discordgo.TextInputShort, Required: true, MaxLength: 32,
				}}},
				discordgo.ActionsRow{Components: []discordgo.MessageComponent{discordgo.TextInput{
					CustomID: "password", Label: "Dashboard password", Style: discordgo.TextInputShort, Required: true, MinLength: 8,
				}}},
				discordgo.ActionsRow{Components: []discordgo.MessageComponent{discordgo.TextInput{
					CustomID: "player_name", Label: "In-game player name", Style: discordgo.TextInputShort, Required: true, MaxLength: 64,
				}}},
			},
		},
	})
}

func showLinkModal(s *discordgo.Session, i *discordgo.InteractionCreate) {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: modalLink,
			Title:    "Link FactoryMate account",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{Components: []discordgo.MessageComponent{discordgo.TextInput{
					CustomID: "fm_username", Label: "FactoryMate username", Style: discordgo.TextInputShort, Required: true,
				}}},
				discordgo.ActionsRow{Components: []discordgo.MessageComponent{discordgo.TextInput{
					CustomID: "password", Label: "Dashboard password", Style: discordgo.TextInputShort, Required: true,
				}}},
			},
		},
	})
}

func respondEphemeral(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
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
