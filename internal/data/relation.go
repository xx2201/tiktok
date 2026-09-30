package data

import (
	"context"
	"github.com/bytedance-youthcamp-jbzx/tiktok/internal/biz"
)

func (d *Data) FavoriteVideos(ctx context.Context, user int64) ([]*biz.Video, error) {
	var rows []Video
	err := d.query(ctx).Model(&Video{}).Joins("JOIN user_favorite_videos f ON f.video_id = videos.id").Where("f.user_id = ? AND videos.status = ?", user, "ready").Order("videos.created_at DESC, videos.id DESC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make([]*biz.Video, 0, len(rows))
	for i := range rows {
		result = append(result, asVideo(&rows[i]))
	}
	return result, nil
}
func (d *Data) relationUsers(ctx context.Context, user int64, kind string) ([]*biz.User, error) {
	query := d.query(ctx).Model(&User{})
	switch kind {
	case "following":
		query = query.Joins("JOIN relations r ON r.to_user_id = users.id AND r.deleted_at IS NULL").Where("r.user_id = ?", user)
	case "followers":
		query = query.Joins("JOIN relations r ON r.user_id = users.id AND r.deleted_at IS NULL").Where("r.to_user_id = ?", user)
	case "friends":
		query = query.Joins("JOIN relations r ON r.to_user_id = users.id AND r.deleted_at IS NULL").Joins("JOIN relations reverse ON reverse.user_id = users.id AND reverse.deleted_at IS NULL").Where("r.user_id = ? AND reverse.to_user_id = ?", user, user)
	}
	var rows []User
	if err := query.Order("users.id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]*biz.User, 0, len(rows))
	for i := range rows {
		result = append(result, asUser(&rows[i]))
	}
	return result, nil
}
func (d *Data) Following(ctx context.Context, user int64) ([]*biz.User, error) {
	return d.relationUsers(ctx, user, "following")
}
func (d *Data) Followers(ctx context.Context, user int64) ([]*biz.User, error) {
	return d.relationUsers(ctx, user, "followers")
}
func (d *Data) Friends(ctx context.Context, user int64) ([]*biz.User, error) {
	return d.relationUsers(ctx, user, "friends")
}
