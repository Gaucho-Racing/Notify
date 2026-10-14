package worker

import (
	"context"
	"errors"

	"github.com/gaucho-racing/notify/notify/model"
	"github.com/gaucho-racing/notify/notify/pkg/sentinel"
	"github.com/gaucho-racing/notify/notify/pkg/twilio"
	"github.com/gaucho-racing/notify/notify/service"
)

const smsVisibleDigits = 4

type smsSender struct{}

func (smsSender) Provider() string {
	return "twilio"
}

func (smsSender) Destination(contact sentinel.Contact) (string, bool) {
	return twilio.NormalizePhoneNumber(contact.PhoneNumber)
}

func (smsSender) MaskDestination(destination string) string {
	if len(destination) <= smsVisibleDigits {
		return "***"
	}
	return "***" + destination[len(destination)-smsVisibleDigits:]
}

func (smsSender) Send(ctx context.Context, outbound Outbound, destination string) (string, error) {
	sender, creds, err := smsSenderFor(outbound.Message.SMSSenderID)
	if err != nil {
		return "", err
	}
	return twilio.Send(ctx, creds, twilio.Message{
		To:                  destination,
		From:                sender.FromNumber,
		MessagingServiceSID: sender.MessagingServiceSID,
		Body:                outbound.Message.Body,
	})
}

func smsSenderFor(senderID string) (model.SMSSender, twilio.Credentials, error) {
	if senderID == "" {
		return model.SMSSender{}, twilio.Credentials{}, unavailableError{errors.New("message has no sms sender assigned")}
	}
	sender, creds, err := service.SMSSenderForDelivery(senderID)
	if errors.Is(err, service.ErrSMSSenderNotFound) || errors.Is(err, service.ErrSMSSenderDisabled) ||
		errors.Is(err, service.ErrTwilioAccountNotFound) || errors.Is(err, service.ErrTwilioAccountDisabled) {
		return model.SMSSender{}, twilio.Credentials{}, unavailableError{err}
	}
	return sender, creds, err
}
