package data

import (
	"gorm.io/gorm"
	"time"
)

type User struct {
	gorm.Model
	UserName        string `gorm:"index:idx_username,unique;type:varchar(40);not null" json:"name,omitempty"`
	Password        string `gorm:"type:varchar(256);not null" json:"password,omitempty"`
	FollowingCount  uint   `gorm:"default:0;not null" json:"follow_count,omitempty"`                                                           // 关注总数
	FollowerCount   uint   `gorm:"default:0;not null" json:"follower_count,omitempty"`                                                         // 粉丝总数
	Avatar          string `gorm:"type:varchar(256)" json:"avatar,omitempty"`                                                                  // 用户头像
	BackgroundImage string `gorm:"column:background_image;type:varchar(256);default:default_background.jpg" json:"background_image,omitempty"` // 用户个人页顶部大图
	WorkCount       uint   `gorm:"default:0;not null" json:"work_count,omitempty"`                                                             // 作品数
	FavoriteCount   uint   `gorm:"default:0;not null" json:"favorite_count,omitempty"`                                                         // 喜欢数
	TotalFavorited  uint   `gorm:"default:0;not null" json:"total_favorited,omitempty"`                                                        // 获赞总量
	Signature       string `gorm:"type:varchar(256)" json:"signature,omitempty"`                                                               // 个人简介
}

func (User) TableName() string {
	return "users"
}

type Video struct {
	ID            uint      `gorm:"primarykey"`
	CreatedAt     time.Time `gorm:"not null;index:idx_create" json:"created_at,omitempty"`
	UpdatedAt     time.Time
	DeletedAt     gorm.DeletedAt `gorm:"index"`
	Author        User           `gorm:"foreignkey:AuthorID" json:"author,omitempty"`
	AuthorID      uint           `gorm:"index:idx_authorid;not null" json:"author_id,omitempty"`
	PlayUrl       string         `gorm:"type:varchar(255);not null" json:"play_url,omitempty"`
	CoverUrl      string         `gorm:"type:varchar(255)" json:"cover_url,omitempty"`
	FavoriteCount uint           `gorm:"default:0;not null" json:"favorite_count,omitempty"`
	CommentCount  uint           `gorm:"default:0;not null" json:"comment_count,omitempty"`
	Title         string         `gorm:"type:varchar(50);not null" json:"title,omitempty"`
	Status        string         `gorm:"type:varchar(16);default:ready;not null;index"`
}

func (Video) TableName() string {
	return "videos"
}

type Comment struct {
	ID         uint      `gorm:"primarykey"`
	CreatedAt  time.Time `gorm:"index;not null" json:"create_date"`
	UpdatedAt  time.Time
	DeletedAt  gorm.DeletedAt `gorm:"index"`
	Video      Video          `gorm:"foreignkey:VideoID" json:"video,omitempty"`
	VideoID    uint           `gorm:"index:idx_videoid;not null" json:"video_id"`
	User       User           `gorm:"foreignkey:UserID" json:"user,omitempty"`
	UserID     uint           `gorm:"index:idx_userid;not null" json:"user_id"`
	Content    string         `gorm:"type:varchar(255);not null" json:"content"`
	LikeCount  uint           `gorm:"column:like_count;default:0;not null" json:"like_count,omitempty"`
	TeaseCount uint           `gorm:"column:tease_count;default:0;not null" json:"tease_count,omitempty"`
}

func (Comment) TableName() string {
	return "comments"
}

type FavoriteVideoRelation struct {
	Video   Video `gorm:"foreignkey:VideoID;" json:"video,omitempty"`
	VideoID uint  `gorm:"uniqueIndex:idx_favorite_pair;index:idx_videoid;not null" json:"video_id"`
	User    User  `gorm:"foreignkey:UserID;" json:"user,omitempty"`
	UserID  uint  `gorm:"uniqueIndex:idx_favorite_pair;index:idx_userid;not null" json:"user_id"`
}

func (FavoriteVideoRelation) TableName() string {
	return "user_favorite_videos"
}

type FollowRelation struct {
	gorm.Model
	User     User `gorm:"foreignkey:UserID;" json:"user,omitempty"`
	UserID   uint `gorm:"uniqueIndex:idx_relation_pair;index:idx_userid;not null" json:"user_id"`
	ToUser   User `gorm:"foreignkey:ToUserID;" json:"to_user,omitempty"`
	ToUserID uint `gorm:"uniqueIndex:idx_relation_pair;index:idx_userid;index:idx_userid_to;not null" json:"to_user_id"`
}

func (FollowRelation) TableName() string {
	return "relations"
}

type Message struct {
	ID         uint      `gorm:"primarykey"`
	CreatedAt  time.Time `gorm:"index;not null" json:"create_time"`
	UpdatedAt  time.Time
	DeletedAt  gorm.DeletedAt `gorm:"index"`
	FromUser   User           `gorm:"foreignkey:FromUserID;" json:"from_user,omitempty"`
	FromUserID uint           `gorm:"index:idx_userid_from;not null" json:"from_user_id"`
	ToUser     User           `gorm:"foreignkey:ToUserID;" json:"to_user,omitempty"`
	ToUserID   uint           `gorm:"index:idx_userid_from;index:idx_userid_to;not null" json:"to_user_id"`
	Content    string         `gorm:"type:text;not null" json:"content"`
}

func (Message) TableName() string {
	return "messages"
}
