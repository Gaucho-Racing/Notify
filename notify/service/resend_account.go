package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/gaucho-racing/notify/notify/config"
	"github.com/gaucho-racing/notify/notify/database"
	"github.com/gaucho-racing/notify/notify/model"
	"github.com/gaucho-racing/notify/notify/pkg/resend"
	"github.com/gaucho-racing/notify/notify/pkg/secretbox"
	ulid "github.com/gaucho-racing/ulid-go"
	"gorm.io/gorm"
)

const resendKeyHintLength = 4

var (
	ErrResendAccountNotFound  = errors.New("resend account not found")
	ErrResendAccountDisabled  = errors.New("resend account is disabled")
	ErrResendAccountNameTaken = errors.New("a resend account with that name already exists")
	ErrResendAccountInUse     = errors.New("resend account still has email senders; move or delete them first")
	ErrInvalidResendKey       = errors.New("resend rejected the api key")
	ErrResendKeySendingOnly   = errors.New("this resend api key is sending-only and cannot list domains")
)

type ResendAccountInput struct {
	Name        *string
	Description *string
	APIKey      *string
	Enabled     *bool
}

// ResendAccountUsage pairs an account with how many senders use it, so the
// portal can explain why an account cannot be deleted.
type ResendAccountUsage struct {
	model.ResendAccount
	SenderCount int64 `json:"sender_count"`
}

func ListResendAccounts() ([]ResendAccountUsage, error) {
	accounts := []model.ResendAccount{}
	if err := database.DB.Order("created_at asc").Find(&accounts).Error; err != nil {
		return nil, err
	}
	var counts []struct {
		ResendAccountID string
		Count           int64
	}
	if err := database.DB.Model(&model.EmailSender{}).
		Select("resend_account_id, count(*) as count").
		Group("resend_account_id").
		Scan(&counts).Error; err != nil {
		return nil, err
	}
	byAccount := map[string]int64{}
	for _, row := range counts {
		byAccount[row.ResendAccountID] = row.Count
	}
	usages := make([]ResendAccountUsage, 0, len(accounts))
	for _, account := range accounts {
		usages = append(usages, ResendAccountUsage{ResendAccount: account, SenderCount: byAccount[account.ID]})
	}
	return usages, nil
}

func GetResendAccount(id string) (model.ResendAccount, error) {
	return getResendAccount(database.DB, id)
}

func getResendAccount(db *gorm.DB, id string) (model.ResendAccount, error) {
	var account model.ResendAccount
	if err := db.Where("id = ?", id).First(&account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.ResendAccount{}, ErrResendAccountNotFound
		}
		return model.ResendAccount{}, err
	}
	return account, nil
}

// CreateResendAccount checks the key with Resend before storing it.
func CreateResendAccount(ctx context.Context, input ResendAccountInput, actor Actor) (model.ResendAccount, error) {
	if input.Name == nil || input.APIKey == nil {
		return model.ResendAccount{}, errors.New("name and api_key are required")
	}
	account := model.ResendAccount{
		ID:                ulid.Make().Prefixed("rsnd"),
		Name:              *input.Name,
		Enabled:           true,
		CreatedByEntityID: actor.EntityID,
		UpdatedByEntityID: actor.EntityID,
	}
	if input.Description != nil {
		account.Description = *input.Description
	}
	if input.Enabled != nil {
		account.Enabled = *input.Enabled
	}
	if err := applyResendKey(ctx, &account, *input.APIKey); err != nil {
		return model.ResendAccount{}, err
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockSenderTable(tx, "notify_resend_account"); err != nil {
			return err
		}
		if err := ensureSenderNameAvailable(tx, &model.ResendAccount{}, account.Name, "", ErrResendAccountNameTaken); err != nil {
			return err
		}
		return tx.Create(&account).Error
	})
	if err != nil {
		return model.ResendAccount{}, err
	}
	return account, nil
}

func UpdateResendAccount(ctx context.Context, id string, input ResendAccountInput, actor Actor) (model.ResendAccount, error) {
	var newKey *model.ResendAccount
	if input.APIKey != nil {
		newKey = &model.ResendAccount{ID: id}
		if err := applyResendKey(ctx, newKey, *input.APIKey); err != nil {
			return model.ResendAccount{}, err
		}
	}

	var account model.ResendAccount
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockSenderTable(tx, "notify_resend_account"); err != nil {
			return err
		}
		var err error
		account, err = getResendAccount(tx, id)
		if err != nil {
			return err
		}
		if input.Name != nil {
			if err := ensureSenderNameAvailable(tx, &model.ResendAccount{}, *input.Name, account.ID, ErrResendAccountNameTaken); err != nil {
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
		if newKey != nil {
			account.KeyCiphertext = newKey.KeyCiphertext
			account.KeyHint = newKey.KeyHint
			account.FullAccess = newKey.FullAccess
		}
		account.UpdatedByEntityID = actor.EntityID
		return tx.Save(&account).Error
	})
	if err != nil {
		return model.ResendAccount{}, err
	}
	return account, nil
}

// DeleteResendAccount refuses while senders still point at the account, so a
// sender can never be left without a way to send.
func DeleteResendAccount(id string) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockSenderTable(tx, "notify_resend_account"); err != nil {
			return err
		}
		if _, err := getResendAccount(tx, id); err != nil {
			return err
		}
		var senders int64
		if err := tx.Model(&model.EmailSender{}).Where("resend_account_id = ?", id).Count(&senders).Error; err != nil {
			return err
		}
		if senders > 0 {
			return ErrResendAccountInUse
		}
		return tx.Delete(&model.ResendAccount{}, "id = ?", id).Error
	})
}

// ListResendAccountDomains returns the account's domains as Resend reports
// them, for the portal to show which from addresses will work.
func ListResendAccountDomains(ctx context.Context, id string) ([]resend.Domain, error) {
	key, err := resendAccountKey(id, false)
	if err != nil {
		return nil, err
	}
	domains, err := resend.ListDomains(ctx, key)
	if err != nil {
		if resend.IsRestrictedKey(err) {
			return nil, ErrResendKeySendingOnly
		}
		return nil, fmt.Errorf("%w: %v", ErrResendUnreachable, err)
	}
	return domains, nil
}

// ResendAccountKey returns the decrypted key for sending. It re-checks
// Enabled so disabling an account stops its queued deliveries too.
func ResendAccountKey(id string) (string, error) {
	return resendAccountKey(id, true)
}

func resendAccountKey(id string, requireEnabled bool) (string, error) {
	account, err := GetResendAccount(id)
	if err != nil {
		return "", err
	}
	if requireEnabled && !account.Enabled {
		return "", ErrResendAccountDisabled
	}
	return config.Secrets.Open(account.KeyCiphertext, account.ID)
}

// applyResendKey probes the key by listing domains: success means full
// access, restricted_api_key means a valid sending-only key, and anything
// else means Resend did not accept it or could not be reached.
func applyResendKey(ctx context.Context, account *model.ResendAccount, apiKey string) error {
	if config.Secrets == nil {
		return secretbox.ErrNotConfigured
	}
	_, err := resend.ListDomains(ctx, apiKey)
	switch {
	case err == nil:
		account.FullAccess = true
	case resend.IsRestrictedKey(err):
		account.FullAccess = false
	case resend.IsInvalidKey(err):
		return ErrInvalidResendKey
	default:
		return fmt.Errorf("%w: %v", ErrResendUnreachable, err)
	}
	ciphertext, err := config.Secrets.Seal(apiKey, account.ID)
	if err != nil {
		return err
	}
	account.KeyCiphertext = ciphertext
	account.KeyHint = apiKey[max(len(apiKey)-resendKeyHintLength, 0):]
	return nil
}
