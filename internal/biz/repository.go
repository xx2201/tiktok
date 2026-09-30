package biz

import "context"

type UserReader interface {
	FindUser(context.Context, int64) (*User, error)
}
type RelationReader interface {
	IsFollowing(context.Context, int64, int64) (bool, error)
}
type UserRepository interface {
	UserReader
	RelationReader
	FindUserByName(context.Context, string) (*User, error)
	CreateUser(context.Context, *User) error
}
type VideoRepository interface {
	UserReader
	RelationReader
	FindVideo(context.Context, int64) (*Video, error)
	ListVideos(context.Context, int64, int64, int) ([]*Video, error)
	IsFavorite(context.Context, int64, int64) (bool, error)
	CreateVideo(context.Context, *Video) error
	CompleteVideo(context.Context, int64) error
	DeleteVideo(context.Context, int64, int64) error
}
type CommentRepository interface {
	UserReader
	RelationReader
	FindVideo(context.Context, int64) (*Video, error)
	FindComment(context.Context, int64) (*Comment, error)
	CreateComment(context.Context, *Comment) error
	DeleteComment(context.Context, int64, int64, int64) error
	ListComments(context.Context, int64) ([]*Comment, error)
}
type FavoriteRepository interface {
	VideoRepository
	FavoriteVideos(context.Context, int64) ([]*Video, error)
}
type RelationRepository interface {
	UserReader
	RelationReader
	Following(context.Context, int64) ([]*User, error)
	Followers(context.Context, int64) ([]*User, error)
	Friends(context.Context, int64) ([]*User, error)
	LatestMessage(context.Context, int64, int64) (*Message, error)
}
type MessageRepository interface {
	UserReader
	RelationReader
	CreateMessage(context.Context, *Message) error
	ListMessages(context.Context, int64, int64, int64) ([]*Message, error)
}
type ActionPublisher interface {
	Publish(context.Context, *Action) error
}
type Media interface {
	URL(context.Context, string, string) (string, error)
	Publish(context.Context, []byte, string, string) error
	Remove(context.Context, string, string) error
}
type MessageCipher interface {
	Encrypt(string) (string, error)
	Decrypt(string) (string, error)
}
