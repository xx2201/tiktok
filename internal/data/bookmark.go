package data

import (
	"context"
	"github.com/bytedance-youthcamp-jbzx/tiktok/internal/biz"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

type Bookmark struct {
	ID        int64     `gorm:"primaryKey;autoIncrement;index:idx_bookmark_page,priority:2"`
	UserID    int64     `gorm:"not null;uniqueIndex:idx_bookmark_pair,priority:1;index:idx_bookmark_page,priority:1"`
	VideoID   int64     `gorm:"not null;uniqueIndex:idx_bookmark_pair,priority:2"`
	CreatedAt time.Time `gorm:"not null"`
}

func (Bookmark) TableName() string { return "user_bookmarks" }
func (d *Data) SetBookmark(ctx context.Context, user, video int64, collect bool) error {
	return d.primary(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockUsers(tx, user); err != nil {
			return err
		}
		if !collect {
			return tx.Where("user_id = ? AND video_id = ?", user, video).Delete(&Bookmark{}).Error
		}
		var target Video
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND status = ?", video, "ready").First(&target).Error; err != nil {
			return missing(err)
		}
		row := Bookmark{UserID: user, VideoID: video}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error
	})
}
func (d *Data) ListBookmarks(ctx context.Context, user, cursor int64, limit int) ([]*biz.Bookmark, error) {
	var rows []struct {
		Video      Video `gorm:"embedded"`
		BookmarkID int64
	}
	// 私人列表固定绑定 token 身份；主库读取保证刚提交的收藏立即可见。
	query := d.primary(ctx).Table("user_bookmarks AS b").Select("v.*, b.id AS bookmark_id").
		Joins("JOIN videos AS v ON v.id = b.video_id AND v.deleted_at IS NULL AND v.status = ?", "ready").
		Where("b.user_id = ?", user)
	if cursor > 0 {
		query = query.Where("b.id < ?", cursor)
	}
	if err := query.Order("b.id DESC").Limit(limit).Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]*biz.Bookmark, 0, len(rows))
	for i := range rows {
		result = append(result, &biz.Bookmark{ID: rows[i].BookmarkID, Video: asVideo(&rows[i].Video)})
	}
	return result, nil
}
