package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gaucho-racing/notify/notify/database"
	"github.com/gaucho-racing/notify/notify/model"
	"github.com/gaucho-racing/notify/notify/pkg/logger"
	"github.com/gaucho-racing/notify/notify/pkg/resend"
	ulid "github.com/gaucho-racing/ulid-go"
	"gorm.io/gorm"
)

var (
	ErrEmailSenderNotFound    = errors.New("email sender not found")
	ErrEmailSenderDisabled    = errors.New("email sender is disabled")
	ErrEmailSenderNameTaken   = errors.New("an email sender with that name already exists")
	ErrNoDefaultEmailSender   = errors.New("no default email sender is configured; pass email_sender_id or ask a Notify admin to set a default")
	ErrDefaultEmailSenderHeld = errors.New("the default email sender cannot be disabled or deleted; make another sender the default first")
	ErrEmailDomainNotVerified = errors.New("the from address's domain is not verified in Resend")
	ErrResendUnreachable      = errors.New("could not check verified domains with Resend")
)

type EmailSenderInput struct {
	Name        *string
	Description *string
	FromName    *string
	FromAddress *string
	ReplyTo     *string
	IsDefault   *bool
	Enabled     *bool
}

func ListEmailSenders() ([]model.EmailSender, error) {
	senders := []model.EmailSender{}
	if err := database.DB.Order("created_at asc").Find(&senders).Error; err != nil {
		return nil, err
	}
	return senders, nil
}

func GetEmailSender(id string) (model.EmailSender, error) {
	return getEmailSender(database.DB, id)
}

func getEmailSender(db *gorm.DB, id string) (model.EmailSender, error) {
	var sender model.EmailSender
	if err := db.Where("id = ?", id).First(&sender).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.EmailSender{}, ErrEmailSenderNotFound
		}
		return model.EmailSender{}, err
	}
	return sender, nil
}

// CreateEmailSender checks the from domain against Resend before storing the
// sender. The first sender created becomes the default.
func CreateEmailSender(ctx context.Context, input EmailSenderInput, actor Actor) (model.EmailSender, error) {
	if input.Name == nil || input.FromAddress == nil {
		return model.EmailSender{}, errors.New("name and from_address are required")
	}
	if err := ensureDomainVerified(ctx, *input.FromAddress); err != nil {
		return model.EmailSender{}, err
	}
	sender := model.EmailSender{
		ID:                ulid.Make().Prefixed("esnd"),
		Enabled:           true,
		CreatedByEntityID: actor.EntityID,
		UpdatedByEntityID: actor.EntityID,
	}
	applyEmailSenderInput(&sender, input)

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockSenderTable(tx, "notify_email_sender"); err != nil {
			return err
		}
		if err := ensureSenderNameAvailable(tx, &model.EmailSender{}, sender.Name, "", ErrEmailSenderNameTaken); err != nil {
			return err
		}
		defaults, err := countDefaultSenders(tx, &model.EmailSender{})
		if err != nil {
			return err
		}
		if defaults == 0 && sender.Enabled {
			sender.IsDefault = true
		}
		if sender.IsDefault && !sender.Enabled {
			return ErrDefaultEmailSenderHeld
		}
		if sender.IsDefault {
			if err := clearDefaultSender(tx, &model.EmailSender{}); err != nil {
				return err
			}
		}
		return tx.Create(&sender).Error
	})
	if err != nil {
		return model.EmailSender{}, err
	}
	return sender, nil
}

func UpdateEmailSender(ctx context.Context, id string, input EmailSenderInput, actor Actor) (model.EmailSender, error) {
	if input.FromAddress != nil {
		if err := ensureDomainVerified(ctx, *input.FromAddress); err != nil {
			return model.EmailSender{}, err
		}
	}

	var sender model.EmailSender
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockSenderTable(tx, "notify_email_sender"); err != nil {
			return err
		}
		var err error
		sender, err = getEmailSender(tx, id)
		if err != nil {
			return err
		}
		wasDefault := sender.IsDefault

		if input.Name != nil {
			if err := ensureSenderNameAvailable(tx, &model.EmailSender{}, *input.Name, sender.ID, ErrEmailSenderNameTaken); err != nil {
				return err
			}
		}
		applyEmailSenderInput(&sender, input)
		if wasDefault && (!sender.IsDefault || !sender.Enabled) {
			return ErrDefaultEmailSenderHeld
		}
		sender.UpdatedByEntityID = actor.EntityID

		if sender.IsDefault && !wasDefault {
			if err := clearDefaultSender(tx, &model.EmailSender{}); err != nil {
				return err
			}
		}
		return tx.Save(&sender).Error
	})
	if err != nil {
		return model.EmailSender{}, err
	}
	return sender, nil
}

// DeleteEmailSender refuses to remove the default while other senders remain.
// Deliveries still queued for a deleted sender fail permanently.
func DeleteEmailSender(id string) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockSenderTable(tx, "notify_email_sender"); err != nil {
			return err
		}
		sender, err := getEmailSender(tx, id)
		if err != nil {
			return err
		}
		if sender.IsDefault {
			others, err := countOtherSenders(tx, &model.EmailSender{}, id)
			if err != nil {
				return err
			}
			if others > 0 {
				return ErrDefaultEmailSenderHeld
			}
		}
		return tx.Delete(&model.EmailSender{}, "id = ?", id).Error
	})
}

// ResolveEmailSenderForSend picks the sender a new message will use: the
// named one, or the default when none is named.
func ResolveEmailSenderForSend(id string) (model.EmailSender, error) {
	var sender model.EmailSender
	if id == "" {
		if err := database.DB.Where("is_default").First(&sender).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return model.EmailSender{}, ErrNoDefaultEmailSender
			}
			return model.EmailSender{}, err
		}
	} else {
		var err error
		if sender, err = GetEmailSender(id); err != nil {
			return model.EmailSender{}, err
		}
	}
	if !sender.Enabled {
		return model.EmailSender{}, ErrEmailSenderDisabled
	}
	return sender, nil
}

// EmailSenderForDelivery re-checks Enabled so disabling a sender stops its
// queued deliveries too.
func EmailSenderForDelivery(id string) (model.EmailSender, error) {
	sender, err := GetEmailSender(id)
	if err != nil {
		return model.EmailSender{}, err
	}
	if !sender.Enabled {
		return model.EmailSender{}, ErrEmailSenderDisabled
	}
	return sender, nil
}

func applyEmailSenderInput(sender *model.EmailSender, input EmailSenderInput) {
	if input.Name != nil {
		sender.Name = *input.Name
	}
	if input.Description != nil {
		sender.Description = *input.Description
	}
	if input.FromName != nil {
		sender.FromName = *input.FromName
	}
	if input.FromAddress != nil {
		sender.FromAddress = *input.FromAddress
	}
	if input.ReplyTo != nil {
		sender.ReplyTo = *input.ReplyTo
	}
	if input.IsDefault != nil {
		sender.IsDefault = *input.IsDefault
	}
	if input.Enabled != nil {
		sender.Enabled = *input.Enabled
	}
}

// ensureDomainVerified requires an exact match: Resend treats a subdomain as
// its own domain, so notify.example.com being verified does not cover
// example.com. A sending-only API key cannot list domains, in which case the
// check is skipped and Resend rejects a bad domain at send time instead.
func ensureDomainVerified(ctx context.Context, address string) error {
	_, domain, _ := strings.Cut(address, "@")
	domain = strings.ToLower(domain)

	domains, err := resend.ListDomains(ctx)
	if err != nil {
		if resend.IsRestrictedKey(err) {
			logger.SugarLogger.Warnf("RESEND_API_KEY is sending-only, so %s was not checked against verified domains", domain)
			return nil
		}
		return fmt.Errorf("%w: %v", ErrResendUnreachable, err)
	}
	verified := []string{}
	for _, candidate := range domains {
		if !candidate.CanSend() {
			continue
		}
		if strings.EqualFold(candidate.Name, domain) {
			return nil
		}
		verified = append(verified, candidate.Name)
	}
	if len(verified) == 0 {
		return fmt.Errorf("%w: %s (no domains are verified yet)", ErrEmailDomainNotVerified, domain)
	}
	return fmt.Errorf("%w: %s (verified: %s)", ErrEmailDomainNotVerified, domain, strings.Join(verified, ", "))
}
