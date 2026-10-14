package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/gaucho-racing/notify/notify/database"
	"github.com/gaucho-racing/notify/notify/model"
	"github.com/gaucho-racing/notify/notify/pkg/twilio"
	ulid "github.com/gaucho-racing/ulid-go"
	"gorm.io/gorm"
)

var (
	ErrSMSSenderNotFound     = errors.New("sms sender not found")
	ErrSMSSenderDisabled     = errors.New("sms sender is disabled")
	ErrSMSSenderNameTaken    = errors.New("an sms sender with that name already exists")
	ErrNoDefaultSMSSender    = errors.New("no default sms sender is configured; pass sms_sender_id or ask a Notify admin to set a default")
	ErrDefaultSMSSenderHeld  = errors.New("the default sms sender cannot be disabled or deleted; make another sender the default first")
	ErrSMSSenderSourceNeeded = errors.New("set exactly one of from_number or messaging_service_sid")
	ErrSMSSourceNotOnAccount = errors.New("the sender is not on this Twilio account")
)

type SMSSenderInput struct {
	Name                *string
	Description         *string
	TwilioAccountID     *string
	FromNumber          *string
	MessagingServiceSID *string
	IsDefault           *bool
	Enabled             *bool
}

func ListSMSSenders() ([]model.SMSSender, error) {
	senders := []model.SMSSender{}
	if err := database.DB.Order("created_at asc").Find(&senders).Error; err != nil {
		return nil, err
	}
	return senders, nil
}

func GetSMSSender(id string) (model.SMSSender, error) {
	return getSMSSender(database.DB, id)
}

func getSMSSender(db *gorm.DB, id string) (model.SMSSender, error) {
	var sender model.SMSSender
	if err := db.Where("id = ?", id).First(&sender).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.SMSSender{}, ErrSMSSenderNotFound
		}
		return model.SMSSender{}, err
	}
	return sender, nil
}

// CreateSMSSender checks the number or messaging service against Twilio
// before storing the sender. The first sender created becomes the default.
func CreateSMSSender(ctx context.Context, input SMSSenderInput, actor Actor) (model.SMSSender, error) {
	if input.Name == nil || input.TwilioAccountID == nil {
		return model.SMSSender{}, errors.New("name and twilio_account_id are required")
	}
	sender := model.SMSSender{
		ID:                ulid.Make().Prefixed("ssnd"),
		Enabled:           true,
		CreatedByEntityID: actor.EntityID,
		UpdatedByEntityID: actor.EntityID,
	}
	applySMSSenderInput(&sender, input)
	if err := ensureSMSSourceOnAccount(ctx, sender); err != nil {
		return model.SMSSender{}, err
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockSenderTable(tx, "notify_sms_sender"); err != nil {
			return err
		}
		if err := ensureSenderNameAvailable(tx, &model.SMSSender{}, sender.Name, "", ErrSMSSenderNameTaken); err != nil {
			return err
		}
		defaults, err := countDefaultSenders(tx, &model.SMSSender{})
		if err != nil {
			return err
		}
		if defaults == 0 && sender.Enabled {
			sender.IsDefault = true
		}
		if sender.IsDefault && !sender.Enabled {
			return ErrDefaultSMSSenderHeld
		}
		if sender.IsDefault {
			if err := clearDefaultSender(tx, &model.SMSSender{}); err != nil {
				return err
			}
		}
		return tx.Create(&sender).Error
	})
	if err != nil {
		return model.SMSSender{}, err
	}
	return sender, nil
}

func UpdateSMSSender(ctx context.Context, id string, input SMSSenderInput, actor Actor) (model.SMSSender, error) {
	if input.TwilioAccountID != nil || input.FromNumber != nil || input.MessagingServiceSID != nil {
		current, err := GetSMSSender(id)
		if err != nil {
			return model.SMSSender{}, err
		}
		applySMSSenderInput(&current, input)
		if err := ensureSMSSourceOnAccount(ctx, current); err != nil {
			return model.SMSSender{}, err
		}
	}

	var sender model.SMSSender
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockSenderTable(tx, "notify_sms_sender"); err != nil {
			return err
		}
		var err error
		sender, err = getSMSSender(tx, id)
		if err != nil {
			return err
		}
		wasDefault := sender.IsDefault

		if input.Name != nil {
			if err := ensureSenderNameAvailable(tx, &model.SMSSender{}, *input.Name, sender.ID, ErrSMSSenderNameTaken); err != nil {
				return err
			}
		}
		applySMSSenderInput(&sender, input)
		if wasDefault && (!sender.IsDefault || !sender.Enabled) {
			return ErrDefaultSMSSenderHeld
		}
		sender.UpdatedByEntityID = actor.EntityID

		if sender.IsDefault && !wasDefault {
			if err := clearDefaultSender(tx, &model.SMSSender{}); err != nil {
				return err
			}
		}
		return tx.Save(&sender).Error
	})
	if err != nil {
		return model.SMSSender{}, err
	}
	return sender, nil
}

// DeleteSMSSender refuses to remove the default while other senders remain.
// Deliveries still queued for a deleted sender fail permanently.
func DeleteSMSSender(id string) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockSenderTable(tx, "notify_sms_sender"); err != nil {
			return err
		}
		sender, err := getSMSSender(tx, id)
		if err != nil {
			return err
		}
		if sender.IsDefault {
			others, err := countOtherSenders(tx, &model.SMSSender{}, id)
			if err != nil {
				return err
			}
			if others > 0 {
				return ErrDefaultSMSSenderHeld
			}
		}
		return tx.Delete(&model.SMSSender{}, "id = ?", id).Error
	})
}

// ResolveSMSSenderForSend picks the sender a new message will use: the named
// one, or the default when none is named.
func ResolveSMSSenderForSend(id string) (model.SMSSender, error) {
	var sender model.SMSSender
	if id == "" {
		if err := database.DB.Where("is_default").First(&sender).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return model.SMSSender{}, ErrNoDefaultSMSSender
			}
			return model.SMSSender{}, err
		}
	} else {
		var err error
		if sender, err = GetSMSSender(id); err != nil {
			return model.SMSSender{}, err
		}
	}
	if !sender.Enabled {
		return model.SMSSender{}, ErrSMSSenderDisabled
	}
	account, err := GetTwilioAccount(sender.TwilioAccountID)
	if err != nil {
		return model.SMSSender{}, err
	}
	if !account.Enabled {
		return model.SMSSender{}, ErrTwilioAccountDisabled
	}
	return sender, nil
}

// SMSSenderForDelivery returns the sender and its account's credentials. It
// re-checks both Enabled flags so disabling either stops queued deliveries.
func SMSSenderForDelivery(id string) (model.SMSSender, twilio.Credentials, error) {
	sender, err := GetSMSSender(id)
	if err != nil {
		return model.SMSSender{}, twilio.Credentials{}, err
	}
	if !sender.Enabled {
		return model.SMSSender{}, twilio.Credentials{}, ErrSMSSenderDisabled
	}
	creds, err := TwilioAccountCredentials(sender.TwilioAccountID)
	if err != nil {
		return model.SMSSender{}, twilio.Credentials{}, err
	}
	return sender, creds, nil
}

func applySMSSenderInput(sender *model.SMSSender, input SMSSenderInput) {
	if input.Name != nil {
		sender.Name = *input.Name
	}
	if input.Description != nil {
		sender.Description = *input.Description
	}
	if input.TwilioAccountID != nil {
		sender.TwilioAccountID = *input.TwilioAccountID
	}
	if input.FromNumber != nil {
		sender.FromNumber = *input.FromNumber
	}
	if input.MessagingServiceSID != nil {
		sender.MessagingServiceSID = *input.MessagingServiceSID
	}
	if input.IsDefault != nil {
		sender.IsDefault = *input.IsDefault
	}
	if input.Enabled != nil {
		sender.Enabled = *input.Enabled
	}
}

// ensureSMSSourceOnAccount confirms the sender's number (SMS-capable) or
// messaging service exists on its Twilio account, so a typo fails here
// instead of on every delivery.
func ensureSMSSourceOnAccount(ctx context.Context, sender model.SMSSender) error {
	if (sender.FromNumber == "") == (sender.MessagingServiceSID == "") {
		return ErrSMSSenderSourceNeeded
	}
	creds, err := twilioCredentials(sender.TwilioAccountID, false)
	if err != nil {
		return err
	}
	if sender.MessagingServiceSID != "" {
		if _, err := twilio.GetMessagingService(ctx, creds, sender.MessagingServiceSID); err != nil {
			if twilio.IsNotFound(err) {
				return fmt.Errorf("%w: messaging service %s", ErrSMSSourceNotOnAccount, sender.MessagingServiceSID)
			}
			return twilioRequestError(err)
		}
		return nil
	}
	numbers, err := twilio.ListPhoneNumbers(ctx, creds)
	if err != nil {
		return twilioRequestError(err)
	}
	for _, number := range numbers {
		if number.PhoneNumber == sender.FromNumber {
			if !number.SMS {
				return fmt.Errorf("%w: %s is on the account but cannot send SMS", ErrSMSSourceNotOnAccount, sender.FromNumber)
			}
			return nil
		}
	}
	return fmt.Errorf("%w: %s", ErrSMSSourceNotOnAccount, sender.FromNumber)
}
