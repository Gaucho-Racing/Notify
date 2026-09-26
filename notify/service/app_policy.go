package service

import (
	"errors"

	"github.com/gaucho-racing/notify/notify/database"
	"github.com/gaucho-racing/notify/notify/model"
	ulid "github.com/gaucho-racing/ulid-go"
	"gorm.io/gorm"
)

var ErrAppPolicyNotFound = errors.New("application policy not found")

func ListAppPolicies() ([]model.AppPolicy, error) {
	policies := []model.AppPolicy{}
	if err := database.DB.Order("created_at asc").Find(&policies).Error; err != nil {
		return nil, err
	}
	return policies, nil
}

func GetAppPolicy(clientID string) (model.AppPolicy, error) {
	var policy model.AppPolicy
	if err := database.DB.Where("client_id = ?", clientID).First(&policy).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.AppPolicy{}, ErrAppPolicyNotFound
		}
		return model.AppPolicy{}, err
	}
	return policy, nil
}

func UpsertAppPolicy(policy model.AppPolicy, actor Actor) (model.AppPolicy, error) {
	existing, err := GetAppPolicy(policy.ClientID)
	if err != nil && !errors.Is(err, ErrAppPolicyNotFound) {
		return model.AppPolicy{}, err
	}

	policy.UpdatedByEntityID = actor.EntityID
	if errors.Is(err, ErrAppPolicyNotFound) {
		policy.ID = ulid.Make().Prefixed("pol")
		policy.CreatedByEntityID = actor.EntityID
		if err := database.DB.Create(&policy).Error; err != nil {
			return model.AppPolicy{}, err
		}
		return policy, nil
	}

	policy.ID = existing.ID
	policy.CreatedByEntityID = existing.CreatedByEntityID
	policy.CreatedAt = existing.CreatedAt
	if err := database.DB.Save(&policy).Error; err != nil {
		return model.AppPolicy{}, err
	}
	return policy, nil
}

func DeleteAppPolicy(clientID string) error {
	result := database.DB.Where("client_id = ?", clientID).Delete(&model.AppPolicy{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrAppPolicyNotFound
	}
	return nil
}
