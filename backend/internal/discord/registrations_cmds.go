package discord

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"factorymate/internal/auth"
	"factorymate/internal/notify"
	"factorymate/internal/registration"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) handleRegistrationsCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData, externalID string, perms memberPermissions, state LinkState) {
	if len(data.Options) == 0 {
		respondEphemeral(s, i, "Unknown subcommand.")
		return
	}
	if !CanRunCommand(perms, CommandGroupAdmin, state) {
		b.logAndDeny(ctx, s, i, externalID, "registrations", "forbidden")
		return
	}

	sub := data.Options[0]
	switch sub.Name {
	case "list":
		b.handleRegistrationsList(ctx, s, i, externalID)
	case "approve":
		b.handleRegistrationsApprove(ctx, s, i, externalID, sub.Options)
	case "reject":
		b.handleRegistrationsReject(ctx, s, i, externalID, sub.Options)
	default:
		respondEphemeral(s, i, "Unknown subcommand.")
	}
}

func (b *Bot) handleRegistrationsList(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, externalID string) {
	pending, err := b.registration.ListPending(ctx)
	if err != nil {
		respondEphemeral(s, i, "Could not load pending registrations.")
		_ = LogBotCommand(ctx, b.db, externalID, "registrations list", false, err.Error())
		return
	}
	if len(pending) == 0 {
		respondEphemeral(s, i, "No registrations are pending approval.")
		_ = LogBotCommand(ctx, b.db, externalID, "registrations list", true, "empty")
		return
	}

	lines := []string{fmt.Sprintf("**%d registration(s) pending approval:**", len(pending)), ""}
	for _, p := range pending {
		discord := p.ExternalDisplay
		if discord == "" {
			discord = p.ExternalUsername
		}
		if discord == "" {
			discord = "—"
		}
		lines = append(lines, fmt.Sprintf(
			"- **%s** (id %d) · Discord: %s · In-game: %s · submitted %s",
			p.Username, p.ID, discord, p.PendingPlayerName, p.CreatedAt,
		))
	}
	lines = append(lines, "", "Approve or reject in the web UI or with `/registrations approve` / `/registrations reject`.")
	respondEphemeral(s, i, strings.Join(lines, "\n"))
	_ = LogBotCommand(ctx, b.db, externalID, "registrations list", true, fmt.Sprintf("count=%d", len(pending)))
}

func (b *Bot) handleRegistrationsApprove(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, externalID string, opts []*discordgo.ApplicationCommandInteractionDataOption) {
	userID, err := resolveRegistrationTargetID(ctx, b.registration, s, opts)
	if err != nil {
		respondEphemeral(s, i, err.Error())
		_ = LogBotCommand(ctx, b.db, externalID, "registrations approve", false, err.Error())
		return
	}

	adminUser, err := b.registration.GetByExternal(ctx, registration.PlatformDiscord, externalID)
	if err != nil || adminUser == nil || adminUser.Role != auth.RoleAdmin {
		respondEphemeral(s, i, "Only admins can approve registrations.")
		_ = LogBotCommand(ctx, b.db, externalID, "registrations approve", false, "forbidden")
		return
	}

	expired, err := b.registration.RegistrationButtonExpired(ctx, userID)
	if err != nil {
		respondEphemeral(s, i, "Could not verify registration age.")
		_ = LogBotCommand(ctx, b.db, externalID, "registrations approve", false, err.Error())
		return
	}
	if expired {
		respondEphemeral(s, i, "This registration is older than 7 days — use the web UI or `/registrations approve`.")
		_ = LogBotCommand(ctx, b.db, externalID, "registrations approve", false, "expired")
		return
	}

	approved, err := b.registration.ApproveRegistration(ctx, userID, adminUser.ID)
	if err != nil {
		msg := "Could not approve registration."
		if errors.Is(err, registration.ErrNotPendingApproval) {
			msg = "This registration is no longer pending."
		}
		respondEphemeral(s, i, msg)
		_ = LogBotCommand(ctx, b.db, externalID, "registrations approve", false, err.Error())
		return
	}

	respondEphemeral(s, i, fmt.Sprintf("Approved **%s**.", approved.Username))
	if extID := approvedExternalID(approved); extID != "" {
		b.SendWelcomeDM(ctx, extID, approved.Username)
	}
	_ = LogBotCommand(ctx, b.db, externalID, "registrations approve", true, strconv.FormatInt(userID, 10))
}

func (b *Bot) handleRegistrationsReject(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, externalID string, opts []*discordgo.ApplicationCommandInteractionDataOption) {
	userID, err := resolveRegistrationTargetID(ctx, b.registration, s, opts)
	if err != nil {
		respondEphemeral(s, i, err.Error())
		_ = LogBotCommand(ctx, b.db, externalID, "registrations reject", false, err.Error())
		return
	}

	reason := ""
	for _, opt := range opts {
		if opt.Name == "reason" {
			reason = opt.StringValue()
		}
	}

	adminUser, err := b.registration.GetByExternal(ctx, registration.PlatformDiscord, externalID)
	if err != nil || adminUser == nil || adminUser.Role != auth.RoleAdmin {
		respondEphemeral(s, i, "Only admins can reject registrations.")
		_ = LogBotCommand(ctx, b.db, externalID, "registrations reject", false, "forbidden")
		return
	}

	expired, err := b.registration.RegistrationButtonExpired(ctx, userID)
	if err != nil {
		respondEphemeral(s, i, "Could not verify registration age.")
		_ = LogBotCommand(ctx, b.db, externalID, "registrations reject", false, err.Error())
		return
	}
	if expired {
		respondEphemeral(s, i, "This registration is older than 7 days — use the web UI or `/registrations reject`.")
		_ = LogBotCommand(ctx, b.db, externalID, "registrations reject", false, "expired")
		return
	}

	targetExternalID, err := b.registration.RejectRegistration(ctx, userID, adminUser.ID, reason)
	if err != nil {
		msg := "Could not reject registration."
		if errors.Is(err, registration.ErrNotPendingApproval) {
			msg = "This registration is no longer pending."
		}
		respondEphemeral(s, i, msg)
		_ = LogBotCommand(ctx, b.db, externalID, "registrations reject", false, err.Error())
		return
	}

	respondEphemeral(s, i, "Registration rejected.")
	if targetExternalID != "" {
		b.SendRegistrationDeclinedDM(ctx, targetExternalID, reason)
	}
	_ = LogBotCommand(ctx, b.db, externalID, "registrations reject", true, strconv.FormatInt(userID, 10))
}

func (b *Bot) handleUnlinkCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData, externalID string, perms memberPermissions, state LinkState) {
	if !CanRunCommand(perms, CommandGroupAdmin, state) {
		b.logAndDeny(ctx, s, i, externalID, "unlink", "forbidden")
		return
	}

	target := optionUserValue(data.Options, "user", s)
	if target == nil {
		respondEphemeral(s, i, "Could not resolve target user.")
		return
	}

	fmUser, err := b.registration.GetByExternal(ctx, registration.PlatformDiscord, target.ID)
	if err != nil {
		respondEphemeral(s, i, "Something went wrong.")
		_ = LogBotCommand(ctx, b.db, externalID, "unlink", false, err.Error())
		return
	}
	if fmUser == nil {
		respondEphemeral(s, i, "That Discord user is not linked to a FactoryMate account.")
		_ = LogBotCommand(ctx, b.db, externalID, "unlink", false, "not linked")
		return
	}

	updated, err := b.registration.UpdateExternalIdentity(ctx, fmUser.ID, registration.ExternalUpdate{Unlink: true})
	if err != nil {
		respondEphemeral(s, i, "Could not unlink Discord account.")
		_ = LogBotCommand(ctx, b.db, externalID, "unlink", false, err.Error())
		return
	}

	respondEphemeral(s, i, fmt.Sprintf("Unlinked Discord from **%s**.", updated.Username))
	_ = LogBotCommand(ctx, b.db, externalID, "unlink", true, updated.Username)
}

func (b *Bot) handlePasswordResetCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData, externalID string, perms memberPermissions, state LinkState) {
	if !CanRunCommand(perms, CommandGroupAdmin, state) {
		b.logAndDeny(ctx, s, i, externalID, "password-reset", "forbidden")
		return
	}

	target := optionUserValue(data.Options, "user", s)
	if target == nil {
		respondEphemeral(s, i, "Could not resolve target user.")
		return
	}

	fmUser, err := b.registration.GetByExternal(ctx, registration.PlatformDiscord, target.ID)
	if err != nil {
		respondEphemeral(s, i, "Something went wrong.")
		_ = LogBotCommand(ctx, b.db, externalID, "password-reset", false, err.Error())
		return
	}
	if fmUser == nil {
		respondEphemeral(s, i, "That Discord user is not linked to a FactoryMate account.")
		_ = LogBotCommand(ctx, b.db, externalID, "password-reset", false, "not linked")
		return
	}

	tempPassword, err := generateTempPassword()
	if err != nil {
		respondEphemeral(s, i, "Could not generate temporary password.")
		_ = LogBotCommand(ctx, b.db, externalID, "password-reset", false, err.Error())
		return
	}

	authSvc := auth.NewService(b.db)
	if err := authSvc.UpdatePassword(ctx, fmUser.ID, tempPassword); err != nil {
		respondEphemeral(s, i, "Could not update password.")
		_ = LogBotCommand(ctx, b.db, externalID, "password-reset", false, err.Error())
		return
	}

	if b.session == nil {
		respondEphemeral(s, i, "Discord bot is not connected — password updated but DM failed.")
		_ = LogBotCommand(ctx, b.db, externalID, "password-reset", false, "bot offline")
		return
	}

	loginURL := PublicURL() + "/login"
	dmText := fmt.Sprintf(
		"Your FactoryMate password was reset by an admin.\n\nTemporary password: **%s**\nSign in: %s\n\nChange your password after logging in.",
		tempPassword, loginURL,
	)
	provider := notify.NewDiscordProvider(b.session)
	if err := provider.SendDirect(ctx, registration.PlatformDiscord, target.ID, notify.RenderedMessage{Plain: dmText}); err != nil {
		respondEphemeral(s, i, "Password updated but could not DM the user. Share the temporary password manually.")
		_ = LogBotCommand(ctx, b.db, externalID, "password-reset", false, "dm failed")
		return
	}

	respondEphemeral(s, i, fmt.Sprintf("Temporary password sent to <@%s> via DM.", target.ID))
	_ = LogBotCommand(ctx, b.db, externalID, "password-reset", true, fmUser.Username)
}

func resolveRegistrationTargetID(ctx context.Context, regSvc *registration.Service, s *discordgo.Session, opts []*discordgo.ApplicationCommandInteractionDataOption) (int64, error) {
	for _, opt := range opts {
		switch opt.Name {
		case "user":
			target := opt.UserValue(s)
			if target == nil {
				return 0, fmt.Errorf("could not resolve Discord user")
			}
			user, err := regSvc.GetByExternal(ctx, registration.PlatformDiscord, target.ID)
			if err != nil {
				return 0, fmt.Errorf("something went wrong")
			}
			if user == nil {
				return 0, fmt.Errorf("that Discord user has no pending registration")
			}
			return user.ID, nil
		case "id":
			id := opt.IntValue()
			if id <= 0 {
				return 0, fmt.Errorf("invalid FactoryMate user id")
			}
			return id, nil
		}
	}
	return 0, fmt.Errorf("provide a Discord user or FactoryMate user id")
}

func optionUserValue(opts []*discordgo.ApplicationCommandInteractionDataOption, name string, s *discordgo.Session) *discordgo.User {
	for _, opt := range opts {
		if opt.Name == name {
			return opt.UserValue(s)
		}
	}
	return nil
}

func generateTempPassword() (string, error) {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
