package data

import (
	"context"
	"errors"
	"github.com/bytedance-youthcamp-jbzx/tiktok/internal/biz"
	"gorm.io/gorm"
	"time"
)

func (d *Data) CreateMessage(ctx context.Context, message *biz.Message) error {
	row := Message{FromUserID: uint(message.FromUserID), ToUserID: uint(message.ToUserID), Content: message.Content}
	if err := d.primary(ctx).Create(&row).Error; err != nil {
		return err
	}
	message.ID, message.CreatedAt = int64(row.ID), row.CreatedAt
	return nil
}
func (d *Data) ListMessages(ctx context.Context, from, to, after int64) ([]*biz.Message, error) {
	var rows []Message
	query := d.query(ctx).Where("((from_user_id = ? AND to_user_id = ?) OR (from_user_id = ? AND to_user_id = ?))", from, to, to, from)
	if after > 0 {
		query = query.Where("created_at > ?", time.UnixMilli(after))
	}
	if err := query.Order("created_at ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]*biz.Message, 0, len(rows))
	for i := range rows {
		result = append(result, asMessage(&rows[i]))
	}
	return result, nil
}
func (d *Data) LatestMessage(ctx context.Context, from, to int64) (*biz.Message, error) {
	var row Message
	err := d.query(ctx).Where("(from_user_id = ? AND to_user_id = ?) OR (from_user_id = ? AND to_user_id = ?)", from, to, to, from).Order("created_at DESC, id DESC").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return asMessage(&row), nil
}
