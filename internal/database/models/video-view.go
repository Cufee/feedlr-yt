package models

import "go.mongodb.org/mongo-driver/bson/primitive"

// model VideoView {
//   id        String   @id @default(cuid()) @map("_id")
//   createdAt DateTime @default(now())
//   updatedAt DateTime @updatedAt

//   user    User   @relation(fields: [userId], references: [id], onDelete: Cascade)
//   userId  String
//   video   Video  @relation(fields: [videoId], references: [id])
//   videoId String

//   progress Int @default(0)

//   @@index([userId], name: "userId")
//   @@index([videoId], name: "videoId")
//   @@index([userId, videoId], name: "userId_videoId")
//   @@map("video_views")
// }

const VideoViewCollection = "video_views"

type VideoView struct {
	Model `bson:",inline"`

	User    *User              `json:"user" bson:"user,omitempty"`
	UserId  primitive.ObjectID `json:"userId" bson:"userId"`
	Video   *Video             `json:"video" bson:"video,omitempty"`
	VideoId string             `json:"videoId" bson:"videoId"`

	Progress int `json:"progress" bson:"progress"`
}

type VideoViewOptions struct {
	Progress *int
}

func NewVideoView(userId primitive.ObjectID, videoId string, opts ...VideoViewOptions) *VideoView {
	view := &VideoView{
		UserId:   userId,
		VideoId:  videoId,
		Progress: 0,
	}

	if len(opts) > 0 {
		if opts[0].Progress != nil {
			view.Progress = *opts[0].Progress
		}
	}

	return view
}

func (view *VideoView) CollectionName() string {
	return VideoViewCollection
}
