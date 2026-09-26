package worker

import (
	"context"

	"github.com/gaucho-racing/notify/notify/pkg/discord"
	"github.com/gaucho-racing/notify/notify/pkg/sentinel"
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
	return discord.SendDirectMessage(ctx, destination, discord.Message{Embeds: []discord.Embed{embed}})
}

func discordBody(outbound Outbound) string {
	body := outbound.Message.Body
	if outbound.Message.ActionURL != "" {
		body += "\n\n[" + actionLabel(outbound.Message.ActionLabel) + "](" + outbound.Message.ActionURL + ")"
	}
	return body
}
