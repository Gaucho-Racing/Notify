package worker

import (
	"context"
	"errors"

	"github.com/gaucho-racing/notify/notify/pkg/discord"
	"github.com/gaucho-racing/notify/notify/pkg/sentinel"
	"github.com/gaucho-racing/notify/notify/service"
)

const embedColor = 0x18181b

type discordSender struct{}

func (discordSender) Provider() string {
	return "discord"
}

func (discordSender) Destination(contact sentinel.Contact) (string, bool) {
	return contact.DiscordID, contact.DiscordID != ""
}

// Discord user IDs are not contact details on their own, so they are stored
// as-is to make a failed DM traceable to the account.
func (discordSender) MaskDestination(destination string) string {
	return destination
}

func (discordSender) Send(ctx context.Context, outbound Outbound, destination string) (string, error) {
	embed := discord.Embed{
		Title:       discord.TruncateTitle(outbound.Message.Subject),
		Description: discord.TruncateDescription(discordBody(outbound)),
		URL:         outbound.Message.ActionURL,
		Color:       embedColor,
	}
	if outbound.ApplicationName != "" {
		embed.Footer = &discord.EmbedFooter{Text: "Sent by " + outbound.ApplicationName + " via Notify"}
	}
	token, err := botToken(outbound.Message.DiscordBotID)
	if err != nil {
		return "", err
	}
	return discord.SendDirectMessage(ctx, token, destination, discord.Message{Embeds: []discord.Embed{embed}})
}

// botToken fails permanently when the message's bot is gone or disabled:
// retrying cannot help until an admin intervenes, and a manual retry re-reads
// the bot anyway.
func botToken(botID string) (string, error) {
	if botID == "" {
		return "", botUnavailableError{errors.New("message has no discord bot assigned")}
	}
	token, err := service.DiscordBotToken(botID)
	if errors.Is(err, service.ErrDiscordBotNotFound) || errors.Is(err, service.ErrDiscordBotDisabled) {
		return "", botUnavailableError{err}
	}
	return token, err
}

type botUnavailableError struct {
	error
}

func (botUnavailableError) Permanent() bool {
	return true
}

func discordBody(outbound Outbound) string {
	body := outbound.Message.Body
	if outbound.Message.ActionURL != "" {
		body += "\n\n[" + actionLabel(outbound.Message.ActionLabel) + "](" + outbound.Message.ActionURL + ")"
	}
	return body
}
