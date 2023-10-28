package logic

import (
	"github.com/cufee/feedlr-yt/internal/api/sponsorblock"
	"github.com/cufee/feedlr-yt/internal/types"
)

func GetUserSettings(id string) (types.SettingsPageProps, error) {
	return types.SettingsPageProps{
		SponsorBlock: types.SponsorBlockSettingsProps{
			SponsorBlockEnabled:             true,
			AvailableSponsorBlockCategories: sponsorblock.AvailableCategories,
			SelectedSponsorBlockCategories:  []string{sponsorblock.SelfPromo.Value, sponsorblock.Sponsor.Value, sponsorblock.Interaction.Value}},
	}, nil
}
