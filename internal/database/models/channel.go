package models

// model Channel {
//   id        String   @id @map("_id")
//   createdAt DateTime @default(now())
//   updatedAt DateTime @updatedAt

//   url         String
//   title       String
//   thumbnail   String?
//   description String  @db.String

//   videos        Video[]
//   subscriptions UserSubscription[]

//   @@map("channels")
// }

const ChannelCollection = "channels"

type Channel struct {
	Model      `bson:",inline"`
	ExternalID string `json:"eid" bson:"eid"`

	URL         string `json:"url" bson:"url" field:"required"`
	Title       string `json:"title" bson:"title" field:"required"`
	Thumbnail   string `json:"thumbnail" bson:"thumbnail"`
	Description string `json:"description" bson:"description"`

	Videos        []Video            `json:"videos" bson:"videos,omitempty"`
	Subscriptions []UserSubscription `json:"subscriptions" bson:"subscriptions,omitempty"`
}

type ChannelOptions struct {
	Thumbnail   *string
	Description *string
}

func NewChannel(id, url, title string, opts ...ChannelOptions) *Channel {
	var thumbnail, description string
	if len(opts) > 0 {
		if opts[0].Thumbnail != nil {
			thumbnail = *opts[0].Thumbnail
		}
		if opts[0].Description != nil {
			description = *opts[0].Description
		}
	}

	channel := Channel{
		ExternalID:  id,
		URL:         url,
		Title:       title,
		Thumbnail:   thumbnail,
		Description: description,
	}
	return &channel
}

func (c *Channel) CollectionName() string {
	return ChannelCollection
}
