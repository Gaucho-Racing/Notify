package worker

import (
	"context"
	"errors"

	"github.com/gaucho-racing/notify/notify/pkg/discord"
	"github.com/gaucho-racing/notify/notify/pkg/sentinel"
	"github.com/gaucho-racing/notify/notify/service"
)

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
	token, err := botToken(outbound.Message.DiscordBotID)
	if err != nil {
		return "", err
	}
	message := discord.Message{
		Content:         outbound.Message.Body,
		Embeds:          outbound.Message.DiscordEmbeds,
		AllowedMentions: discord.NoMassMentions,
	}
	if outbound.ToChannel && outbound.Message.DiscordWebhook != nil {
		return discord.SendAsWebhook(ctx, token, destination, *outbound.Message.DiscordWebhook, message)
	}
	if outbound.ToChannel {
		return discord.SendChannelMessage(ctx, token, destination, message)
	}
	return discord.SendDirectMessage(ctx, token, destination, message)
}

func botToken(botID string) (string, error) {
	if botID == "" {
		return "", unavailableError{errors.New("message has no discord bot assigned")}
	}
	token, err := service.DiscordBotToken(botID)
	if errors.Is(err, service.ErrDiscordBotNotFound) || errors.Is(err, service.ErrDiscordBotDisabled) {
		return "", unavailableError{err}
	}
	return token, err
}
