package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/gaucho-racing/notify/notify/config"
	"github.com/gaucho-racing/notify/notify/database"
	"github.com/gaucho-racing/notify/notify/model"
	"github.com/gaucho-racing/notify/notify/pkg/discord"
	"github.com/gaucho-racing/notify/notify/pkg/secretbox"
	ulid "github.com/gaucho-racing/ulid-go"
	"gorm.io/gorm"
)

const discordTokenHintLength = 4

var (
	ErrDiscordBotNotFound    = errors.New("discord bot not found")
	ErrDiscordBotDisabled    = errors.New("discord bot is disabled")
	ErrDiscordBotNameTaken   = errors.New("a discord bot with that name already exists")
	ErrNoDefaultDiscordBot   = errors.New("no default discord bot is configured; pass discord_bot_id or ask a Notify admin to set a default")
	ErrDefaultDiscordBotHeld = errors.New("the default discord bot cannot be disabled or deleted; make another bot the default first")
	ErrInvalidDiscordToken   = errors.New("discord rejected the bot token")
	ErrDiscordTokenNotBot    = errors.New("the token belongs to a user account, not a bot")
	ErrDiscordUnreachable    = errors.New("could not verify the token with discord")
)

type DiscordBotInput struct {
	Name        *string
	Description *string
	Token       *string
	IsDefault   *bool
	Enabled     *bool
}

func ListDiscordBots() ([]model.DiscordBot, error) {
	bots := []model.DiscordBot{}
	if err := database.DB.Order("created_at asc").Find(&bots).Error; err != nil {
		return nil, err
	}
	return bots, nil
}

func GetDiscordBot(id string) (model.DiscordBot, error) {
	return getDiscordBot(database.DB, id)
}

func getDiscordBot(db *gorm.DB, id string) (model.DiscordBot, error) {
	var bot model.DiscordBot
	if err := db.Where("id = ?", id).First(&bot).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.DiscordBot{}, ErrDiscordBotNotFound
		}
		return model.DiscordBot{}, err
	}
	return bot, nil
}

// CreateDiscordBot verifies the token against Discord before storing it. The
// first bot created becomes the default so DISCORD sends work immediately.
func CreateDiscordBot(ctx context.Context, input DiscordBotInput, actor Actor) (model.DiscordBot, error) {
	if input.Name == nil || input.Token == nil {
		return model.DiscordBot{}, errors.New("name and token are required")
	}
	bot := model.DiscordBot{
		ID:                ulid.Make().Prefixed("dbot"),
		Name:              *input.Name,
		Enabled:           true,
		CreatedByEntityID: actor.EntityID,
		UpdatedByEntityID: actor.EntityID,
	}
	if input.Description != nil {
		bot.Description = *input.Description
	}
	if input.Enabled != nil {
		bot.Enabled = *input.Enabled
	}
	if input.IsDefault != nil {
		bot.IsDefault = *input.IsDefault
	}
	if err := applyDiscordToken(ctx, &bot, *input.Token); err != nil {
		return model.DiscordBot{}, err
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockSenderTable(tx, "notify_discord_bot"); err != nil {
			return err
		}
		if err := ensureSenderNameAvailable(tx, &model.DiscordBot{}, bot.Name, "", ErrDiscordBotNameTaken); err != nil {
			return err
		}
		defaults, err := countDefaultSenders(tx, &model.DiscordBot{})
		if err != nil {
			return err
		}
		if defaults == 0 && bot.Enabled {
			bot.IsDefault = true
		}
		if bot.IsDefault && !bot.Enabled {
			return ErrDefaultDiscordBotHeld
		}
		if bot.IsDefault {
			if err := clearDefaultSender(tx, &model.DiscordBot{}); err != nil {
				return err
			}
		}
		return tx.Create(&bot).Error
	})
	if err != nil {
		return model.DiscordBot{}, err
	}
	return bot, nil
}

func UpdateDiscordBot(ctx context.Context, id string, input DiscordBotInput, actor Actor) (model.DiscordBot, error) {
	var newToken *model.DiscordBot
	if input.Token != nil {
		newToken = &model.DiscordBot{ID: id}
		if err := applyDiscordToken(ctx, newToken, *input.Token); err != nil {
			return model.DiscordBot{}, err
		}
	}

	var bot model.DiscordBot
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockSenderTable(tx, "notify_discord_bot"); err != nil {
			return err
		}
		var err error
		bot, err = getDiscordBot(tx, id)
		if err != nil {
			return err
		}
		wasDefault := bot.IsDefault

		if input.Name != nil {
			if err := ensureSenderNameAvailable(tx, &model.DiscordBot{}, *input.Name, bot.ID, ErrDiscordBotNameTaken); err != nil {
				return err
			}
			bot.Name = *input.Name
		}
		if input.Description != nil {
			bot.Description = *input.Description
		}
		if input.Enabled != nil {
			bot.Enabled = *input.Enabled
		}
		if input.IsDefault != nil {
			bot.IsDefault = *input.IsDefault
		}
		if wasDefault && (!bot.IsDefault || !bot.Enabled) {
			return ErrDefaultDiscordBotHeld
		}
		if newToken != nil {
			bot.DiscordUserID = newToken.DiscordUserID
			bot.DiscordUsername = newToken.DiscordUsername
			bot.AvatarURL = newToken.AvatarURL
			bot.TokenCiphertext = newToken.TokenCiphertext
			bot.TokenHint = newToken.TokenHint
		}
		bot.UpdatedByEntityID = actor.EntityID

		if bot.IsDefault && !wasDefault {
			if err := clearDefaultSender(tx, &model.DiscordBot{}); err != nil {
				return err
			}
		}
		return tx.Save(&bot).Error
	})
	if err != nil {
		return model.DiscordBot{}, err
	}
	return bot, nil
}

// DeleteDiscordBot refuses to remove the default while other bots remain, so
// callers relying on the default never silently lose Discord delivery.
// Deliveries still queued for a deleted bot fail permanently.
func DeleteDiscordBot(id string) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockSenderTable(tx, "notify_discord_bot"); err != nil {
			return err
		}
		bot, err := getDiscordBot(tx, id)
		if err != nil {
			return err
		}
		if bot.IsDefault {
			others, err := countOtherSenders(tx, &model.DiscordBot{}, id)
			if err != nil {
				return err
			}
			if others > 0 {
				return ErrDefaultDiscordBotHeld
			}
		}
		return tx.Delete(&model.DiscordBot{}, "id = ?", id).Error
	})
}

// ResolveDiscordBotForSend picks the bot a new message will use: the named
// one, or the default when none is named.
func ResolveDiscordBotForSend(id string) (model.DiscordBot, error) {
	var bot model.DiscordBot
	if id == "" {
		if err := database.DB.Where("is_default").First(&bot).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return model.DiscordBot{}, ErrNoDefaultDiscordBot
			}
			return model.DiscordBot{}, err
		}
	} else {
		var err error
		if bot, err = GetDiscordBot(id); err != nil {
			return model.DiscordBot{}, err
		}
	}
	if !bot.Enabled {
		return model.DiscordBot{}, ErrDiscordBotDisabled
	}
	return bot, nil
}

// DiscordBotToken returns the decrypted token for sending. It re-checks
// Enabled so disabling a bot stops its queued deliveries too.
func DiscordBotToken(id string) (string, error) {
	bot, err := GetDiscordBot(id)
	if err != nil {
		return "", err
	}
	if !bot.Enabled {
		return "", ErrDiscordBotDisabled
	}
	return config.Secrets.Open(bot.TokenCiphertext, bot.ID)
}

func applyDiscordToken(ctx context.Context, bot *model.DiscordBot, token string) error {
	if config.Secrets == nil {
		return secretbox.ErrNotConfigured
	}
	user, err := discord.GetCurrentUser(ctx, token)
	if err != nil {
		var discordErr discord.Error
		if errors.As(err, &discordErr) && discordErr.StatusCode == http.StatusUnauthorized {
			return ErrInvalidDiscordToken
		}
		return fmt.Errorf("%w: %v", ErrDiscordUnreachable, err)
	}
	if !user.Bot {
		return ErrDiscordTokenNotBot
	}
	ciphertext, err := config.Secrets.Seal(token, bot.ID)
	if err != nil {
		return err
	}
	bot.DiscordUserID = user.ID
	bot.DiscordUsername = user.Username
	bot.AvatarURL = user.AvatarURL()
	bot.TokenCiphertext = ciphertext
	bot.TokenHint = token[len(token)-discordTokenHintLength:]
	return nil
}
