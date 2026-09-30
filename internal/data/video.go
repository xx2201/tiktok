package data

import (
	"context"
	"errors"
	"github.com/bytedance-youthcamp-jbzx/tiktok/internal/biz"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

func (d *Data) FindVideo(ctx context.Context, id int64) (*biz.Video, error) {
	var row Video
	err := d.query(ctx).First(&row, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return asVideo(&row), nil
}
func (d *Data) ListVideos(ctx context.Context, author, before int64, limit int) ([]*biz.Video, error) {
	var rows []Video
	query := d.query(ctx).Where("status = ?", "ready")
	if author > 0 {
		query = query.Where("author_id = ?", author)
	} else {
		query = query.Where("created_at < ?", time.UnixMilli(before))
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Order("created_at DESC, id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]*biz.Video, 0, len(rows))
	for i := range rows {
		result = append(result, asVideo(&rows[i]))
	}
	return result, nil
}
func (d *Data) IsFavorite(ctx context.Context, user, video int64) (bool, error) {
	if user <= 0 {
		return false, nil
	}
	var count int64
	err := d.query(ctx).Model(&FavoriteVideoRelation{}).Where("user_id = ? AND video_id = ?", user, video).Count(&count).Error
	return count > 0, err
}
func (d *Data) CreateVideo(ctx context.Context, video *biz.Video) error {
	row := Video{AuthorID: uint(video.AuthorID), Title: video.Title, PlayUrl: video.PlayURL, CoverUrl: video.CoverURL, Status: video.Status}
	err := d.primary(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		return updateCount(tx, video.AuthorID, "work_count", 1)
	})
	if err != nil {
		return err
	}
	video.ID, video.CreatedAt = int64(row.ID), row.CreatedAt
	return nil
}
func (d *Data) CompleteVideo(ctx context.Context, id int64) error {
	res := d.primary(ctx).Model(&Video{}).Where("id = ? AND status = ?", id, "uploading").Update("status", "ready")
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return biz.ErrNotFound
	}
	return nil
}
func (d *Data) DeleteVideo(ctx context.Context, id, author int64) error {
	return d.primary(ctx).Transaction(func(tx *gorm.DB) error {
		var row Video
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND author_id = ?", id, author).First(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := tx.Delete(&row).Error; err != nil {
			return err
		}
		return updateCount(tx, author, "work_count", -1)
	})
}
