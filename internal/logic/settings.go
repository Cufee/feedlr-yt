package logic

import (
	"errors"

	"github.com/cufee/feedlr-yt/internal/api/sponsorblock"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/cufee/feedlr-yt/internal/types"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/exp/slices"
)

var defaultSettings = types.SettingsPageProps{
	SponsorBlock: types.SponsorBlockSettingsProps{
		SponsorBlockEnabled:             true,
		AvailableSponsorBlockCategories: sponsorblock.AvailableCategories,
		SelectedSponsorBlockCategories:  []string{sponsorblock.SelfPromo.Value, sponsorblock.Sponsor.Value, sponsorblock.Interaction.Value}},
}

func GetUserSettings(id string) (types.SettingsPageProps, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return types.SettingsPageProps{}, err
	}
	settings, err := database.DefaultClient.GetUserSettings(oid)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return types.SettingsPageProps{}, err
	}
	if settings == nil {
		return defaultSettings, nil
	}
	var props = defaultSettings
	props.SponsorBlock.SponsorBlockEnabled = settings.SponsorBlockEnabled
	props.SponsorBlock.SelectedSponsorBlockCategories = settings.SponsorBlockCategories
	return props, nil
}

func ToggleSponsorBlockCategory(id string, category string) (types.SettingsPageProps, error) {
	settings, err := GetUserSettings(id)
	if err != nil {
		return types.SettingsPageProps{}, err
	}

	if !slices.Contains(sponsorblock.ValidCategoryValues, category) {
		return types.SettingsPageProps{}, errors.New("invalid category")
	}

	if slices.Contains(settings.SponsorBlock.SelectedSponsorBlockCategories, category) {
		settings.SponsorBlock.SelectedSponsorBlockCategories = slices.DeleteFunc(settings.SponsorBlock.SelectedSponsorBlockCategories, func(i string) bool {
			return i == category
		})
	} else {
		settings.SponsorBlock.SelectedSponsorBlockCategories = append(settings.SponsorBlock.SelectedSponsorBlockCategories, category)
	}

	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return types.SettingsPageProps{}, err
	}

	_, err = database.DefaultClient.UpdateUserSettings(oid, models.UserSettingsOptions{
		SponsorBlockEnabled:    &settings.SponsorBlock.SponsorBlockEnabled,
		SponsorBlockCategories: &settings.SponsorBlock.SelectedSponsorBlockCategories,
	})
	return settings, err
}

func ToggleSponsorBlock(id string, value bool) (types.SettingsPageProps, error) {
	settings, err := GetUserSettings(id)
	if err != nil {
		return types.SettingsPageProps{}, err
	}

	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return types.SettingsPageProps{}, err
	}

	settings.SponsorBlock.SponsorBlockEnabled = !settings.SponsorBlock.SponsorBlockEnabled
	_, err = database.DefaultClient.UpdateUserSettings(oid, models.UserSettingsOptions{
		SponsorBlockEnabled:    &settings.SponsorBlock.SponsorBlockEnabled,
		SponsorBlockCategories: &settings.SponsorBlock.SelectedSponsorBlockCategories,
	})
	return settings, err
}

func UpdateFeedMode(user, mode string) (types.SettingsPageProps, error) {
	settings, err := GetUserSettings(user)
	if err != nil {
		return types.SettingsPageProps{}, err
	}

	oid, err := primitive.ObjectIDFromHex(user)
	if err != nil {
		return types.SettingsPageProps{}, err
	}

	settings.SponsorBlock.SponsorBlockEnabled = !settings.SponsorBlock.SponsorBlockEnabled
	_, err = database.DefaultClient.UpdateUserSettings(oid, models.UserSettingsOptions{

		SponsorBlockEnabled:    &settings.SponsorBlock.SponsorBlockEnabled,
		SponsorBlockCategories: &settings.SponsorBlock.SelectedSponsorBlockCategories,
	})
	return settings, err
}

// func UpdateUserSettingsFromProps(settings types.SettingsPageProps) error {
// 	oid, err := primitive.ObjectIDFromHex(settings.ID)
// 	if err != nil {
// 		return err
// 	}

// 	_, err = database.DefaultClient.UpdateUserSettings(oid, models.UserSettingsOptions{
// 		SponsorBlockEnabled:    &settings.SponsorBlock.SponsorBlockEnabled,
// 		SponsorBlockCategories: &settings.SponsorBlock.SelectedSponsorBlockCategories,
// 	})
// 	return err
// }
