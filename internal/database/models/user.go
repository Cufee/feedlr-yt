package models

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// model User {
//   id        String   @id @default(cuid()) @map("_id")
//   authId    String   @unique
//   createdAt DateTime @default(now())
//   updatedAt DateTime @updatedAt

//   views         VideoView[]
//   settings      UserSettings?
//   subscriptions UserSubscription[]

//   @@map("users")
// }

const UserCollection = "users"

type User struct {
	Model `bson:",inline"`

	AuthId string `json:"authId" bson:"authId,omitempty"`

	Views         []VideoView        `json:"views" bson:"views,omitempty"`
	Settings      *UserSettings      `json:"settings" bson:"settings,omitempty"`
	Subscriptions []UserSubscription `json:"subscriptions" bson:"subscriptions,omitempty"`
}

func init() {
	addIndexHandler(UserCollection, func(coll *mongo.Collection) ([]string, error) {
		return coll.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
			{
				Keys: bson.M{"authId": 1},
			},
		})
	})
}

func NewUser(authId string) *User {
	return &User{
		AuthId: authId,
	}
}

func (model *User) CollectionName() string {
	return UserCollection
}
