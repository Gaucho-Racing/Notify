package service

import "gorm.io/gorm"

// Discord bots, email senders, and SMS senders share the same shape: unique
// case-insensitive names and at most one default. These helpers enforce that
// for either table inside a write transaction.

// lockSenderTable serializes writes to one table so the default-sender
// invariants are checked against a stable view of it.
func lockSenderTable(tx *gorm.DB, table string) error {
	return tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", table).Error
}

func ensureSenderNameAvailable(tx *gorm.DB, model interface{}, name string, excludeID string, taken error) error {
	var count int64
	query := tx.Model(model).Where("lower(name) = lower(?)", name)
	if excludeID != "" {
		query = query.Where("id <> ?", excludeID)
	}
	if err := query.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return taken
	}
	return nil
}

func countDefaultSenders(tx *gorm.DB, model interface{}) (int64, error) {
	var count int64
	err := tx.Model(model).Where("is_default").Count(&count).Error
	return count, err
}

func countOtherSenders(tx *gorm.DB, model interface{}, id string) (int64, error) {
	var count int64
	err := tx.Model(model).Where("id <> ?", id).Count(&count).Error
	return count, err
}

func clearDefaultSender(tx *gorm.DB, model interface{}) error {
	return tx.Model(model).Where("is_default").Update("is_default", false).Error
}
