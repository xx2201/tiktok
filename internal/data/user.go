package data

import (
	"context"
	"errors"
	"github.com/bytedance-youthcamp-jbzx/tiktok/internal/biz"
	"gorm.io/gorm"
)

func (d *Data) FindUser(ctx context.Context, id int64) (*biz.User, error) {
	if id <= 0 {
		return nil, nil
	}
	var row User
	err := d.query(ctx).First(&row, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return asUser(&row), nil
}
func (d *Data) FindUserByName(ctx context.Context, name string) (*biz.User, error) {
	var row User
	// 注册查重和登录需要看到主库刚写入的数据。
	err := d.primary(ctx).Where("user_name = ?", name).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return asUser(&row), nil
}
func (d *Data) CreateUser(ctx context.Context, user *biz.User) error {
	row := User{UserName: user.Name, Password: user.Password, Avatar: user.Avatar, BackgroundImage: user.BackgroundImage}
	if err := d.primary(ctx).Create(&row).Error; err != nil {
		return writeError(err)
	}
	user.ID = int64(row.ID)
	return nil
}
func (d *Data) IsFollowing(ctx context.Context, user, target int64) (bool, error) {
	if user <= 0 || target <= 0 {
		return false, nil
	}
	var count int64
	err := d.query(ctx).Model(&FollowRelation{}).Where("user_id = ? AND to_user_id = ?", user, target).Count(&count).Error
	return count > 0, err
}
