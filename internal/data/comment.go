package data

import (
	"context"
	"errors"
	"github.com/bytedance-youthcamp-jbzx/tiktok/internal/biz"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (d *Data) FindComment(ctx context.Context, id int64) (*biz.Comment, error) {
	var row Comment
	err := d.query(ctx).First(&row, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return asComment(&row), nil
}
func (d *Data) CreateComment(ctx context.Context, comment *biz.Comment) error {
	row := Comment{VideoID: uint(comment.VideoID), UserID: uint(comment.UserID), Content: comment.Content}
	err := d.primary(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		return videoCount(tx, comment.VideoID, "comment_count", 1)
	})
	if err != nil {
		return err
	}
	comment.ID, comment.CreatedAt = int64(row.ID), row.CreatedAt
	return nil
}
func (d *Data) DeleteComment(ctx context.Context, id, video, user int64) error {
	return d.primary(ctx).Transaction(func(tx *gorm.DB) error {
		var row Comment
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, id).Error; err != nil {
			return missing(err)
		}
		if int64(row.VideoID) != video {
			return biz.ErrInvalid
		}
		var target Video
		if err := tx.First(&target, video).Error; err != nil {
			return missing(err)
		}
		if int64(row.UserID) != user && int64(target.AuthorID) != user {
			return biz.ErrForbidden
		}
		if err := tx.Delete(&row).Error; err != nil {
			return err
		}
		return videoCount(tx, video, "comment_count", -1)
	})
}
func (d *Data) ListComments(ctx context.Context, video int64) ([]*biz.Comment, error) {
	var rows []Comment
	if err := d.query(ctx).Where("video_id = ?", video).Order("created_at DESC, id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]*biz.Comment, 0, len(rows))
	for i := range rows {
		result = append(result, asComment(&rows[i]))
	}
	return result, nil
}
