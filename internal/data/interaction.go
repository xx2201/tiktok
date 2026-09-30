package data

import (
	"context"
	"errors"
	"github.com/bytedance-youthcamp-jbzx/tiktok/internal/biz"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"sort"
)

// 记录已落库动作的序号，避免重复消费或旧消息重新到达导致状态和计数回退。
type InteractionState struct {
	Kind     string `gorm:"primaryKey;size:16"`
	UserID   int64  `gorm:"primaryKey"`
	TargetID int64  `gorm:"primaryKey"`
	Sequence int64  `gorm:"not null"`
}

func updateCount(tx *gorm.DB, user int64, column string, delta int) error {
	return countChange(tx.Model(&User{}).Where("id = ?", user), column, delta)
}
func videoCount(tx *gorm.DB, video int64, column string, delta int) error {
	return countChange(tx.Model(&Video{}).Where("id = ?", video), column, delta)
}
func countChange(query *gorm.DB, column string, delta int) error {
	if delta < 0 {
		query = query.Where(column + " > 0")
	}
	res := query.UpdateColumn(column, gorm.Expr(column+" + ?", delta))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return biz.ErrConflict
	}
	return nil
}
func lockUsers(tx *gorm.DB, ids ...int64) error {
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for i, id := range ids {
		if i > 0 && id == ids[i-1] {
			continue
		}
		var user User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, id).Error; err != nil {
			return missing(err)
		}
	}
	return nil
}
func (d *Data) ApplyAction(ctx context.Context, action *biz.Action) error {
	if action.UserID <= 0 || action.TargetID <= 0 || action.CreatedAt <= 0 || (action.Type != 1 && action.Type != 2) {
		return biz.ErrInvalid
	}
	return d.primary(ctx).Transaction(func(tx *gorm.DB) error {
		state := InteractionState{Kind: action.Kind, UserID: action.UserID, TargetID: action.TargetID}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&state).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&state, "kind = ? AND user_id = ? AND target_id = ?", action.Kind, action.UserID, action.TargetID).Error; err != nil {
			return err
		}
		if state.Sequence >= action.CreatedAt {
			return nil
		}
		var err error
		switch action.Kind {
		case "favorite":
			err = applyFavorite(tx, action)
		case "relation":
			err = applyRelation(tx, action)
		default:
			return biz.ErrInvalid
		}
		if err != nil {
			return err
		}
		return tx.Model(&state).Update("sequence", action.CreatedAt).Error
	})
}
func applyFavorite(tx *gorm.DB, action *biz.Action) error {
	var video Video
	if err := tx.First(&video, action.TargetID).Error; err != nil {
		return missing(err)
	}
	if err := lockUsers(tx, action.UserID, int64(video.AuthorID)); err != nil {
		return err
	}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&video, action.TargetID).Error; err != nil {
		return missing(err)
	}
	var row FavoriteVideoRelation
	err := tx.Where("user_id = ? AND video_id = ?", action.UserID, action.TargetID).First(&row).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	exists := err == nil
	if (action.Type == 1) == exists {
		return nil
	}
	delta := 1
	if action.Type == 1 {
		if err := tx.Create(&FavoriteVideoRelation{UserID: uint(action.UserID), VideoID: uint(action.TargetID)}).Error; err != nil {
			return err
		}
	} else {
		delta = -1
		if err := tx.Where("user_id = ? AND video_id = ?", action.UserID, action.TargetID).Delete(&FavoriteVideoRelation{}).Error; err != nil {
			return err
		}
	}
	if err := videoCount(tx, action.TargetID, "favorite_count", delta); err != nil {
		return err
	}
	if err := updateCount(tx, action.UserID, "favorite_count", delta); err != nil {
		return err
	}
	return updateCount(tx, int64(video.AuthorID), "total_favorited", delta)
}
func applyRelation(tx *gorm.DB, action *biz.Action) error {
	if action.UserID == action.TargetID {
		return biz.ErrInvalid
	}
	if err := lockUsers(tx, action.UserID, action.TargetID); err != nil {
		return err
	}
	var row FollowRelation
	err := tx.Where("user_id = ? AND to_user_id = ?", action.UserID, action.TargetID).First(&row).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	exists := err == nil
	if (action.Type == 1) == exists {
		return nil
	}
	delta := 1
	if action.Type == 1 {
		if err := tx.Create(&FollowRelation{UserID: uint(action.UserID), ToUserID: uint(action.TargetID)}).Error; err != nil {
			return err
		}
	} else {
		delta = -1
		if err := tx.Unscoped().Delete(&row).Error; err != nil {
			return err
		}
	}
	if err := updateCount(tx, action.UserID, "following_count", delta); err != nil {
		return err
	}
	return updateCount(tx, action.TargetID, "follower_count", delta)
}
