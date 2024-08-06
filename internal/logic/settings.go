package logic

import (
	"context"
	"errors"
	"slices"

	"github.com/cufee/feedlr-yt/internal/api/sponsorblock"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/cufee/feedlr-yt/internal/types"
)

var defaultSettings = types.SettingsPageProps{
	PlayerVolume: 100,
	SponsorBlock: types.SponsorBlockSettingsProps{
		SponsorBlockEnabled:             true,
		AvailableSponsorBlockCategories: sponsorblock.AvailableCategories,
		SelectedSponsorBlockCategories:  []string{sponsorblock.SelfPromo.Value, sponsorblock.Sponsor.Value, sponsorblock.Interaction.Value}},
}

func ToggleSponsorBlockCategory(ctx context.Context, db database.SettingsClient, id string, category string) (types.SettingsPageProps, error) {
	if !slices.Contains(sponsorblock.ValidCategoryValues, category) {
		return types.SettingsPageProps{}, errors.New("invalid category")
	}

	settings, err := GetUserSettings(ctx, db, id)
	if err != nil {
		return types.SettingsPageProps{}, err
	}

	if slices.Contains(settings.SponsorBlock.SelectedSponsorBlockCategories, category) {
		settings.SponsorBlock.SelectedSponsorBlockCategories = slices.DeleteFunc(settings.SponsorBlock.SelectedSponsorBlockCategories, func(i string) bool {
			return i == category
		})
	} else {
		settings.SponsorBlock.SelectedSponsorBlockCategories = append(settings.SponsorBlock.SelectedSponsorBlockCategories, category)
	}
	return settings, UpdateUserSettings(ctx, db, id, settings)
}

func ToggleSponsorBlock(ctx context.Context, db database.SettingsClient, id string) (types.SettingsPageProps, error) {
	settings, err := GetUserSettings(ctx, db, id)
	if err != nil {
		return types.SettingsPageProps{}, err
	}

	settings.SponsorBlock.SponsorBlockEnabled = !settings.SponsorBlock.SponsorBlockEnabled
	return settings, UpdateUserSettings(ctx, db, id, settings)
}

func UpdateFeedMode(ctx context.Context, db database.SettingsClient, user, mode string) (types.SettingsPageProps, error) {
	settings, err := GetUserSettings(ctx, db, user)
	if err != nil {
		return types.SettingsPageProps{}, err
	}

	settings.SponsorBlock.SponsorBlockEnabled = !settings.SponsorBlock.SponsorBlockEnabled
	return settings, UpdateUserSettings(ctx, db, user, settings)
}

func UpdatePlayerVolume(ctx context.Context, db database.SettingsClient, user string, volume int) error {
	if volume == 0 {
		return nil
	}

	settings, err := GetUserSettings(ctx, db, user)
	if err != nil && !database.IsErrNotFound(err) {
		return err
	}

	settings.PlayerVolume = volume
	return UpdateUserSettings(ctx, db, user, settings)
}

func UpdateUserSettings(ctx context.Context, db database.SettingsClient, userID string, updated types.SettingsPageProps) error {
	settings, err := db.GetUserSettings(ctx, userID)
	if err != nil {
		return err
	}

	settings.Data, err = updated.Encode()
	if err != nil {
		return err
	}

	err = db.UpsertSettings(ctx, settings)
	if err != nil {
		return err
	}
	return nil
}

func GetUserSettings(ctx context.Context, db database.SettingsClient, id string) (types.SettingsPageProps, error) {
	settings, err := db.GetUserSettings(ctx, id)
	if err != nil && !database.IsErrNotFound(err) {
		return types.SettingsPageProps{}, err
	}
	if settings == nil || database.IsErrNotFound(err) {
		data, _ := defaultSettings.Encode()
		settings := &models.Setting{
			Data:   data,
			UserID: id,
		}
		return defaultSettings, db.UpsertSettings(ctx, settings)
	}

	var props types.SettingsPageProps
	err = props.Decode(settings)
	if err != nil {
		return types.SettingsPageProps{}, err
	}

	return props, nil
}
