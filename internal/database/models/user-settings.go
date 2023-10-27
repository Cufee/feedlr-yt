package models

import "go.mongodb.org/mongo-driver/bson/primitive"

// model UserSettings {
//   id        String   @id @default(cuid()) @map("_id")
//   createdAt DateTime @default(now())
//   updatedAt DateTime @updatedAt

//   user   User   @relation(fields: [userId], references: [id], onDelete: Cascade)
//   userId String @unique

//   sponsorBlockEnabled    Boolean  @default(true)
//   sponsorBlockCategories String[] @default([])

//   @@map("user_settings")
// }

const UserSettingsCollection = "user_settings"

type UserSettings struct {
	Model `bson:",inline"`

	User   *User              `json:"user" bson:"user,omitempty"`
	UserId primitive.ObjectID `json:"userId" bson:"userId" field:"required"`

	SponsorBlockEnabled    bool     `json:"sponsorBlockEnabled" bson:"sponsorBlockEnabled"`
	SponsorBlockCategories []string `json:"sponsorBlockCategories" bson:"sponsorBlockCategories"`
}

type UserSettingsOptions struct {
	SponsorBlockEnabled    *bool
	SponsorBlockCategories *[]string
}

func NewUserSettings(userId primitive.ObjectID, opts ...UserSettingsOptions) *UserSettings {
	settings := &UserSettings{
		UserId:                 userId,
		SponsorBlockEnabled:    true,
		SponsorBlockCategories: []string{},
	}

	if len(opts) > 0 {
		if opts[0].SponsorBlockEnabled != nil {
			settings.SponsorBlockEnabled = *opts[0].SponsorBlockEnabled
		}
		if opts[0].SponsorBlockCategories != nil {
			settings.SponsorBlockCategories = *opts[0].SponsorBlockCategories
		}
	}

	return settings
}

func (settings *UserSettings) CollectionName() string {
	return UserSettingsCollection
}
