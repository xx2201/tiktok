package biz

import "time"

// 业务对象不依赖 GORM 或传输框架，仓储负责映射持久化模型。
type User struct {
	ID                                                                   int64
	Name, Password, Avatar, BackgroundImage, Signature                   string
	FollowCount, FollowerCount, WorkCount, FavoriteCount, TotalFavorited int64
	IsFollow                                                             bool
}

type Video struct {
	ID, AuthorID                     int64
	Title, PlayURL, CoverURL, Status string
	CreatedAt                        time.Time
	FavoriteCount, CommentCount      int64
	IsFavorite                       bool
	Author                           *User
}

type Comment struct {
	ID, VideoID, UserID   int64
	Content               string
	CreatedAt             time.Time
	LikeCount, TeaseCount int64
	User                  *User
}

type Message struct {
	ID, FromUserID, ToUserID int64
	Content                  string
	CreatedAt                time.Time
}

type Friend struct {
	User        *User
	Message     string
	MessageType int64
}

type Action struct {
	Kind      string `json:"kind"`
	UserID    int64  `json:"user_id"`
	TargetID  int64  `json:"target_id"`
	Type      int32  `json:"action_type"`
	CreatedAt int64  `json:"created_at"`
}
