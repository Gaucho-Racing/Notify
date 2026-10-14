package worker

import (
	"context"
	"errors"
	"strings"

	"github.com/gaucho-racing/notify/notify/model"
	"github.com/gaucho-racing/notify/notify/pkg/resend"
	"github.com/gaucho-racing/notify/notify/pkg/sentinel"
	"github.com/gaucho-racing/notify/notify/service"
)

type emailSender struct{}

func (emailSender) Provider() string {
	return "resend"
}

func (emailSender) Destination(contact sentinel.Contact) (string, bool) {
	return contact.Email, contact.Email != ""
}

func (emailSender) MaskDestination(destination string) string {
	local, domain, found := strings.Cut(destination, "@")
	if !found || local == "" {
		return "***"
	}
	return string([]rune(local)[0]) + "***@" + domain
}

func (emailSender) Send(ctx context.Context, outbound Outbound, destination string) (string, error) {
	sender, apiKey, err := emailSenderFor(outbound.Message.EmailSenderID)
	if err != nil {
		return "", err
	}
	email := resend.Email{
		From:    sender.From(),
		To:      []string{destination},
		Subject: outbound.Message.Subject,
		Text:    outbound.Message.Body,
		HTML:    outbound.Message.EmailHTML,
	}
	if sender.ReplyTo != "" {
		email.ReplyTo = []string{sender.ReplyTo}
	}
	return resend.Send(ctx, apiKey, email, outbound.DeliveryID)
}

func emailSenderFor(senderID string) (model.EmailSender, string, error) {
	if senderID == "" {
		return model.EmailSender{}, "", unavailableError{errors.New("message has no email sender assigned")}
	}
	sender, apiKey, err := service.EmailSenderForDelivery(senderID)
	if errors.Is(err, service.ErrEmailSenderNotFound) || errors.Is(err, service.ErrEmailSenderDisabled) ||
		errors.Is(err, service.ErrResendAccountNotFound) || errors.Is(err, service.ErrResendAccountDisabled) {
		return model.EmailSender{}, "", unavailableError{err}
	}
	return sender, apiKey, err
}
