package models

import "go.mongodb.org/mongo-driver/bson/primitive"

// model UserSubscription {
//   id        String   @id @default(cuid()) @map("_id")
//   createdAt DateTime @default(now())
//   updatedAt DateTime @updatedAt

//   isFavorite Boolean @default(false)

//   user      User    @relation(fields: [userId], references: [id], onDelete: Cascade)
//   userId    String
//   channel   Channel @relation(fields: [channelId], references: [id], onDelete: Cascade)
//   channelId String

//   @@index([userId], name: "userId")
//   @@index([channelId], name: "channelId")
//   @@index([userId, channelId], name: "userId_channelId")
//   @@map("user_subscriptions")
// }

const UserSubscriptionCollection = "user_subscriptions"

type UserSubscription struct {
	Model `bson:",inline"`

	IsFavorite bool `json:"isFavorite" bson:"isFavorite"`

	InternalUsers    []User             `json:"users" bson:"users,omitempty"`
	UserId           primitive.ObjectID `json:"userId" bson:"userId" field:"required"`
	InternalChannels []Channel          `json:"channels" bson:"channels,omitempty"`
	ChannelId        string             `json:"channelId" bson:"channelId" field:"required"`
}

func (model *UserSubscription) User() *User {
	if len(model.InternalUsers) > 0 {
		return &model.InternalUsers[0]
	}
	return nil
}

func (model *UserSubscription) Channel() *Channel {
	if len(model.InternalChannels) > 0 {
		return &model.InternalChannels[0]
	}
	return nil
}

type UserSubscriptionOptions struct {
	IsFavorite *bool
}

func NewUserSubscription(userId primitive.ObjectID, channelId string, opts ...UserSubscriptionOptions) *UserSubscription {
	subscription := &UserSubscription{
		UserId:     userId,
		ChannelId:  channelId,
		IsFavorite: false,
	}

	if len(opts) > 0 {
		if opts[0].IsFavorite != nil {
			subscription.IsFavorite = *opts[0].IsFavorite
		}
	}

	return subscription
}

func (subscription *UserSubscription) CollectionName() string {
	return UserSubscriptionCollection
}
