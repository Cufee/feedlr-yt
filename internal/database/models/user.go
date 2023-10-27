package models

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

	AuthId string `json:"authId" bson:"authId" field:"required"`

	Views         []VideoView        `json:"views" bson:"views,omitempty"`
	Settings      *UserSettings      `json:"settings" bson:"settings,omitempty"`
	Subscriptions []UserSubscription `json:"subscriptions" bson:"subscriptions,omitempty"`
}

func NewUser(authId string) *User {
	return &User{
		AuthId: authId,
	}
}

func (model *User) CollectionName() string {
	return UserCollection
}
