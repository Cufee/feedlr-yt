package logic

import (
	"context"
	"encoding/json"
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
	PodcastSegments: types.PodcastSegmentSettingsProps{
		Enabled:             true,
		AvailableCategories: sponsorblock.PodcastAvailableCategories,
		SelectedCategories:  []string{sponsorblock.Sponsor.Value, sponsorblock.SelfPromo.Value, sponsorblock.Interaction.Value, sponsorblock.Preview.Value, sponsorblock.PodcastHook.Value, sponsorblock.PodcastTangents.Value},
	},
}

func atomicPreferencePatch(ctx context.Context, db database.SettingsClient, id string, patch database.SettingsPatch) (types.SettingsPageProps, error, bool) {
	atomic, ok := db.(database.FrontendStateClient)
	if !ok {
		return types.SettingsPageProps{}, nil, false
	}
	if _, err := GetUserSettings(ctx, db, id); err != nil {
		return types.SettingsPageProps{}, err, true
	}
	if err := atomic.PatchUserSettings(ctx, id, patch); err != nil {
		return types.SettingsPageProps{}, err, true
	}
	settings, err := GetUserSettings(ctx, db, id)
	return settings, err, true
}

func ToggleSponsorBlockCategory(ctx context.Context, db database.SettingsClient, id string, category string) (types.SettingsPageProps, error) {
	if !slices.Contains(sponsorblock.ValidCategoryValues, category) {
		return types.SettingsPageProps{}, errors.New("invalid category")
	}
	if settings, err, ok := atomicPreferencePatch(ctx, db, id, database.SettingsPatch{ToggleSponsorCategory: category, SponsorCategoryDefaults: defaultSettings.SponsorBlock.SelectedSponsorBlockCategories}); ok {
		return settings, err
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
	if settings, err, ok := atomicPreferencePatch(ctx, db, id, database.SettingsPatch{ToggleSponsorEnabled: true}); ok {
		return settings, err
	}
	settings, err := GetUserSettings(ctx, db, id)
	if err != nil {
		return types.SettingsPageProps{}, err
	}

	settings.SponsorBlock.SponsorBlockEnabled = !settings.SponsorBlock.SponsorBlockEnabled
	return settings, UpdateUserSettings(ctx, db, id, settings)
}

func TogglePodcastSegmentCategory(ctx context.Context, db database.SettingsClient, id, category string) (types.SettingsPageProps, error) {
	if !slices.Contains(sponsorblock.ValidPodcastCategoryValues, category) {
		return types.SettingsPageProps{}, errors.New("invalid podcast segment category")
	}
	if settings, err, ok := atomicPreferencePatch(ctx, db, id, database.SettingsPatch{TogglePodcastCategory: category, PodcastCategoryDefaults: defaultSettings.PodcastSegments.SelectedCategories}); ok {
		return settings, err
	}
	settings, err := GetUserSettings(ctx, db, id)
	if err != nil {
		return types.SettingsPageProps{}, err
	}
	if slices.Contains(settings.PodcastSegments.SelectedCategories, category) {
		settings.PodcastSegments.SelectedCategories = slices.DeleteFunc(settings.PodcastSegments.SelectedCategories, func(i string) bool { return i == category })
	} else {
		settings.PodcastSegments.SelectedCategories = append(settings.PodcastSegments.SelectedCategories, category)
	}
	return settings, UpdateUserSettings(ctx, db, id, settings)
}

func TogglePodcastSegments(ctx context.Context, db database.SettingsClient, id string) (types.SettingsPageProps, error) {
	if settings, err, ok := atomicPreferencePatch(ctx, db, id, database.SettingsPatch{TogglePodcastEnabled: true}); ok {
		return settings, err
	}
	settings, err := GetUserSettings(ctx, db, id)
	if err != nil {
		return types.SettingsPageProps{}, err
	}
	settings.PodcastSegments.Enabled = !settings.PodcastSegments.Enabled
	return settings, UpdateUserSettings(ctx, db, id, settings)
}

func UpdateFeedMode(ctx context.Context, db database.SettingsClient, user, mode string) (types.SettingsPageProps, error) {
	if settings, err, ok := atomicPreferencePatch(ctx, db, user, database.SettingsPatch{FeedMode: &mode}); ok {
		return settings, err
	}
	settings, err := GetUserSettings(ctx, db, user)
	if err != nil {
		return types.SettingsPageProps{}, err
	}

	settings.FeedMode = mode
	return settings, UpdateUserSettings(ctx, db, user, settings)
}

func UpdatePlayerVolume(ctx context.Context, db database.SettingsClient, user string, volume int) error {
	if volume < 0 || volume > 100 {
		return errors.New("invalid volume")
	}

	settings, err := GetUserSettings(ctx, db, user)
	if err != nil && !database.IsErrNotFound(err) {
		return err
	}
	if volume == settings.PlayerVolume {
		return nil
	}
	if atomic, ok := db.(database.FrontendStateClient); ok {
		return atomic.PatchUserSettings(ctx, user, database.SettingsPatch{Volume: &volume})
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
		settings = &models.Setting{
			Data:   data,
			UserID: id,
		}
		if atomic, ok := db.(interface {
			GetOrCreateUserSettings(context.Context, string, []byte) (*models.Setting, error)
		}); ok {
			settings, err = atomic.GetOrCreateUserSettings(ctx, id, data)
		} else {
			err = db.UpsertSettings(ctx, settings)
		}
		if err != nil {
			return types.SettingsPageProps{}, err
		}
	}

	var props types.SettingsPageProps
	err = props.Decode(settings)
	if err != nil {
		return types.SettingsPageProps{}, err
	}

	// Older JSON settings records predate podcast preferences. Hydrate the new
	// values so existing users get the safe defaults without a migration.
	if props.PodcastSegments.AvailableCategories == nil {
		// Hydration is a read projection. Persisting a whole stale settings object
		// here could replace a preference changed concurrently by another client.
		var presence struct{ PodcastSegments struct{ Enabled *bool } }
		if err := json.Unmarshal(settings.Data, &presence); err != nil {
			return types.SettingsPageProps{}, err
		}
		props.PodcastSegments.AvailableCategories = append([]sponsorblock.Category{}, defaultSettings.PodcastSegments.AvailableCategories...)
		if presence.PodcastSegments.Enabled == nil {
			props.PodcastSegments.Enabled = defaultSettings.PodcastSegments.Enabled
		}
		if props.PodcastSegments.SelectedCategories == nil {
			props.PodcastSegments.SelectedCategories = append([]string{}, defaultSettings.PodcastSegments.SelectedCategories...)
		}
	}
	return props, nil
}
