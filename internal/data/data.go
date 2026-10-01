package data

import (
	"context"
	"errors"
	"github.com/bytedance-youthcamp-jbzx/tiktok/internal/biz"
	"github.com/bytedance-youthcamp-jbzx/tiktok/internal/conf"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
	"time"
)

type Data struct{ db *gorm.DB }

func New(c conf.Database) (*Data, error) {
	db, err := gorm.Open(mysql.Open(c.DSN), &gorm.Config{TranslateError: true})
	if err != nil {
		return nil, err
	}
	d := &Data{db: db}
	if len(c.Replicas) > 0 {
		replicas := make([]gorm.Dialector, 0, len(c.Replicas))
		for _, dsn := range c.Replicas {
			replicas = append(replicas, mysql.Open(dsn))
		}
		if err := db.Use(dbresolver.Register(dbresolver.Config{Replicas: replicas})); err != nil {
			d.Close()
			return nil, err
		}
	}
	pool, err := db.DB()
	if err != nil {
		return nil, err
	}
	pool.SetMaxOpenConns(c.MaxOpen)
	pool.SetMaxIdleConns(c.MaxIdle)
	pool.SetConnMaxLifetime(time.Hour)
	return d, nil
}
func (d *Data) Close() error {
	pool, err := d.db.DB()
	if err != nil {
		return err
	}
	return pool.Close()
}
func (d *Data) Migrate() error {
	// 显式命令执行迁移，启动服务不会自动修改数据库结构。
	return d.db.AutoMigrate(&User{}, &Video{}, &Comment{}, &FavoriteVideoRelation{}, &FollowRelation{}, &Message{}, &InteractionState{}, &Bookmark{})
}
func missing(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return biz.ErrNotFound
	}
	return err
}
func writeError(err error) error {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return biz.ErrConflict
	}
	return err
}
func (d *Data) query(ctx context.Context) *gorm.DB   { return d.db.WithContext(ctx) }
func (d *Data) primary(ctx context.Context) *gorm.DB { return d.query(ctx).Clauses(dbresolver.Write) }

// 复用连接池，为需要提交后立即可读的业务提供主库读取视图。
func (d *Data) Primary() *Data { return &Data{db: d.db.Clauses(dbresolver.Write)} }

func asUser(u *User) *biz.User {
	return &biz.User{ID: int64(u.ID), Name: u.UserName, Password: u.Password, Avatar: u.Avatar, BackgroundImage: u.BackgroundImage, Signature: u.Signature, FollowCount: int64(u.FollowingCount), FollowerCount: int64(u.FollowerCount), WorkCount: int64(u.WorkCount), FavoriteCount: int64(u.FavoriteCount), TotalFavorited: int64(u.TotalFavorited)}
}
func asVideo(v *Video) *biz.Video {
	return &biz.Video{ID: int64(v.ID), AuthorID: int64(v.AuthorID), Title: v.Title, PlayURL: v.PlayUrl, CoverURL: v.CoverUrl, CreatedAt: v.CreatedAt, FavoriteCount: int64(v.FavoriteCount), CommentCount: int64(v.CommentCount), Status: v.Status}
}
func asComment(c *Comment) *biz.Comment {
	return &biz.Comment{ID: int64(c.ID), UserID: int64(c.UserID), VideoID: int64(c.VideoID), Content: c.Content, CreatedAt: c.CreatedAt, LikeCount: int64(c.LikeCount), TeaseCount: int64(c.TeaseCount)}
}
func asMessage(m *Message) *biz.Message {
	return &biz.Message{ID: int64(m.ID), FromUserID: int64(m.FromUserID), ToUserID: int64(m.ToUserID), Content: m.Content, CreatedAt: m.CreatedAt}
}
