package models

import (
	"time"
)

// model AuthNonce {
//   id        String   @id @default(cuid()) @map("_id")
//   createdAt DateTime @default(now())

//   expiresAt DateTime
//   value     String

//   @@unique([expiresAt, value], name: "expiresAt_value")
//   @@index([expiresAt], name: "expiresAt")
//   @@index([value], name: "value")
//   @@map("auth_nonces")
// }

const AuthNonceCollection = "auth_nonces"

type AuthNonce struct {
	Model `bson:",inline"`
	ID    string `json:"id" bson:"_id,omitempty" field:"required"`

	ExpiresAt time.Time `json:"expiresAt" bson:"expiresAt"`
	Value     string    `json:"value" bson:"value"`
}

func NewAuthNonce(expiresAt time.Time, value string) *AuthNonce {
	return &AuthNonce{
		ExpiresAt: expiresAt,
		Value:     value,
	}
}

func (model *AuthNonce) CollectionName() string {
	return AuthNonceCollection
}
