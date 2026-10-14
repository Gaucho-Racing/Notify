package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/gaucho-racing/notify/notify/config"
	"github.com/gaucho-racing/notify/notify/database"
	"github.com/gaucho-racing/notify/notify/model"
	"github.com/gaucho-racing/notify/notify/pkg/secretbox"
	"github.com/gaucho-racing/notify/notify/pkg/twilio"
	ulid "github.com/gaucho-racing/ulid-go"
	"gorm.io/gorm"
)

const twilioSecretHintLength = 4

var (
	ErrTwilioAccountNotFound    = errors.New("twilio account not found")
	ErrTwilioAccountDisabled    = errors.New("twilio account is disabled")
	ErrTwilioAccountNameTaken   = errors.New("a twilio account with that name already exists")
	ErrTwilioAccountInUse       = errors.New("twilio account still has sms senders; move or delete them first")
	ErrInvalidTwilioCredentials = errors.New("twilio rejected the account sid and secret")
	ErrTwilioUnreachable        = errors.New("could not reach Twilio")
)

type TwilioAccountInput struct {
	Name        *string
	Description *string
	AccountSID  *string
	KeySID      *string
	Secret      *string
	Enabled     *bool
}

type TwilioAccountUsage struct {
	model.TwilioAccount
	SenderCount int64 `json:"sender_count"`
}

// TwilioSenderOptions is what an account can send from, for the portal to
// offer when setting up a sender.
type TwilioSenderOptions struct {
	PhoneNumbers      []twilio.PhoneNumber      `json:"phone_numbers"`
	MessagingServices []twilio.MessagingService `json:"messaging_services"`
}

func ListTwilioAccounts() ([]TwilioAccountUsage, error) {
	accounts := []model.TwilioAccount{}
	if err := database.DB.Order("created_at asc").Find(&accounts).Error; err != nil {
		return nil, err
	}
	var counts []struct {
		TwilioAccountID string
		Count           int64
	}
	if err := database.DB.Model(&model.SMSSender{}).
		Select("twilio_account_id, count(*) as count").
		Group("twilio_account_id").
		Scan(&counts).Error; err != nil {
		return nil, err
	}
	byAccount := map[string]int64{}
	for _, row := range counts {
		byAccount[row.TwilioAccountID] = row.Count
	}
	usages := make([]TwilioAccountUsage, 0, len(accounts))
	for _, account := range accounts {
		usages = append(usages, TwilioAccountUsage{TwilioAccount: account, SenderCount: byAccount[account.ID]})
	}
	return usages, nil
}

func GetTwilioAccount(id string) (model.TwilioAccount, error) {
	return getTwilioAccount(database.DB, id)
}

func getTwilioAccount(db *gorm.DB, id string) (model.TwilioAccount, error) {
	var account model.TwilioAccount
	if err := db.Where("id = ?", id).First(&account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.TwilioAccount{}, ErrTwilioAccountNotFound
		}
		return model.TwilioAccount{}, err
	}
	return account, nil
}

// CreateTwilioAccount checks the credentials with Twilio before storing them.
func CreateTwilioAccount(ctx context.Context, input TwilioAccountInput, actor Actor) (model.TwilioAccount, error) {
	if input.Name == nil || input.AccountSID == nil || input.Secret == nil {
		return model.TwilioAccount{}, errors.New("name, account_sid, and secret are required")
	}
	account := model.TwilioAccount{
		ID:                ulid.Make().Prefixed("twac"),
		Name:              *input.Name,
		AccountSID:        *input.AccountSID,
		Enabled:           true,
		CreatedByEntityID: actor.EntityID,
		UpdatedByEntityID: actor.EntityID,
	}
	if input.Description != nil {
		account.Description = *input.Description
	}
	if input.KeySID != nil {
		account.KeySID = *input.KeySID
	}
	if input.Enabled != nil {
		account.Enabled = *input.Enabled
	}
	if err := applyTwilioSecret(ctx, &account, *input.Secret); err != nil {
		return model.TwilioAccount{}, err
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockSenderTable(tx, "notify_twilio_account"); err != nil {
			return err
		}
		if err := ensureSenderNameAvailable(tx, &model.TwilioAccount{}, account.Name, "", ErrTwilioAccountNameTaken); err != nil {
			return err
		}
		return tx.Create(&account).Error
	})
	if err != nil {
		return model.TwilioAccount{}, err
	}
	return account, nil
}

// UpdateTwilioAccount rotates credentials only as a pair: a new key SID
// without its secret (or the reverse) cannot be checked, so both are
// required whenever KeySID changes.
func UpdateTwilioAccount(ctx context.Context, id string, input TwilioAccountInput, actor Actor) (model.TwilioAccount, error) {
	if input.AccountSID != nil {
		return model.TwilioAccount{}, errors.New("account_sid cannot be changed; add a new Twilio account instead")
	}
	if input.KeySID != nil && input.Secret == nil {
		return model.TwilioAccount{}, errors.New("secret is required when changing key_sid")
	}

	var rotated *model.TwilioAccount
	if input.Secret != nil {
		current, err := GetTwilioAccount(id)
		if err != nil {
			return model.TwilioAccount{}, err
		}
		rotated = &model.TwilioAccount{ID: current.ID, AccountSID: current.AccountSID, KeySID: current.KeySID}
		if input.KeySID != nil {
			rotated.KeySID = *input.KeySID
		}
		if err := applyTwilioSecret(ctx, rotated, *input.Secret); err != nil {
			return model.TwilioAccount{}, err
		}
	}

	var account model.TwilioAccount
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockSenderTable(tx, "notify_twilio_account"); err != nil {
			return err
		}
		var err error
		account, err = getTwilioAccount(tx, id)
		if err != nil {
			return err
		}
		if input.Name != nil {
			if err := ensureSenderNameAvailable(tx, &model.TwilioAccount{}, *input.Name, account.ID, ErrTwilioAccountNameTaken); err != nil {
				return err
			}
			account.Name = *input.Name
		}
		if input.Description != nil {
			account.Description = *input.Description
		}
		if input.Enabled != nil {
			account.Enabled = *input.Enabled
		}
		if rotated != nil {
			account.KeySID = rotated.KeySID
			account.SecretCiphertext = rotated.SecretCiphertext
			account.SecretHint = rotated.SecretHint
		}
		account.UpdatedByEntityID = actor.EntityID
		return tx.Save(&account).Error
	})
	if err != nil {
		return model.TwilioAccount{}, err
	}
	return account, nil
}

// DeleteTwilioAccount refuses while senders still point at the account.
func DeleteTwilioAccount(id string) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockSenderTable(tx, "notify_twilio_account"); err != nil {
			return err
		}
		if _, err := getTwilioAccount(tx, id); err != nil {
			return err
		}
		var senders int64
		if err := tx.Model(&model.SMSSender{}).Where("twilio_account_id = ?", id).Count(&senders).Error; err != nil {
			return err
		}
		if senders > 0 {
			return ErrTwilioAccountInUse
		}
		return tx.Delete(&model.TwilioAccount{}, "id = ?", id).Error
	})
}

func ListTwilioSenderOptions(ctx context.Context, id string) (TwilioSenderOptions, error) {
	creds, err := twilioCredentials(id, false)
	if err != nil {
		return TwilioSenderOptions{}, err
	}
	numbers, err := twilio.ListPhoneNumbers(ctx, creds)
	if err != nil {
		return TwilioSenderOptions{}, twilioRequestError(err)
	}
	services, err := twilio.ListMessagingServices(ctx, creds)
	if err != nil {
		return TwilioSenderOptions{}, twilioRequestError(err)
	}
	return TwilioSenderOptions{PhoneNumbers: numbers, MessagingServices: services}, nil
}

// TwilioAccountCredentials returns decrypted credentials for sending. It
// re-checks Enabled so disabling an account stops its queued deliveries too.
func TwilioAccountCredentials(id string) (twilio.Credentials, error) {
	return twilioCredentials(id, true)
}

func twilioCredentials(id string, requireEnabled bool) (twilio.Credentials, error) {
	account, err := GetTwilioAccount(id)
	if err != nil {
		return twilio.Credentials{}, err
	}
	if requireEnabled && !account.Enabled {
		return twilio.Credentials{}, ErrTwilioAccountDisabled
	}
	if config.Secrets == nil {
		return twilio.Credentials{}, secretbox.ErrNotConfigured
	}
	secret, err := config.Secrets.Open(account.SecretCiphertext, account.ID)
	if err != nil {
		return twilio.Credentials{}, err
	}
	return twilio.Credentials{AccountSID: account.AccountSID, KeySID: account.KeySID, Secret: secret}, nil
}

// applyTwilioSecret probes the credentials by listing the account's numbers,
// which fails unless the SID, key, and secret all belong together.
func applyTwilioSecret(ctx context.Context, account *model.TwilioAccount, secret string) error {
	if config.Secrets == nil {
		return secretbox.ErrNotConfigured
	}
	creds := twilio.Credentials{AccountSID: account.AccountSID, KeySID: account.KeySID, Secret: secret}
	if _, err := twilio.ListPhoneNumbers(ctx, creds); err != nil {
		return twilioRequestError(err)
	}
	ciphertext, err := config.Secrets.Seal(secret, account.ID)
	if err != nil {
		return err
	}
	account.SecretCiphertext = ciphertext
	account.SecretHint = secret[max(len(secret)-twilioSecretHintLength, 0):]
	return nil
}

func twilioRequestError(err error) error {
	if twilio.IsAuthError(err) || twilio.IsNotFound(err) {
		return ErrInvalidTwilioCredentials
	}
	return fmt.Errorf("%w: %v", ErrTwilioUnreachable, err)
}
