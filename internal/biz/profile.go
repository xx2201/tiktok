package biz

import (
	"context"
	"github.com/go-kratos/kratos/v2/errors"
)

var (
	ErrInvalid   = errors.BadRequest("INVALID_ARGUMENT", "参数不合法")
	ErrNotFound  = errors.NotFound("NOT_FOUND", "对象不存在")
	ErrForbidden = errors.Forbidden("FORBIDDEN", "没有操作权限")
	ErrConflict  = errors.Conflict("CONFLICT", "对象已存在")
)

type Profiles struct {
	users     UserReader
	relations RelationReader
	media     Media
}

func NewProfiles(users UserReader, relations RelationReader, media Media) *Profiles {
	return &Profiles{users: users, relations: relations, media: media}
}

func (p *Profiles) User(ctx context.Context, id, viewer int64) (*User, error) {
	u, err := p.users.FindUser(ctx, id)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, ErrNotFound
	}
	return p.Decorate(ctx, u, viewer)
}

func (p *Profiles) Decorate(ctx context.Context, u *User, viewer int64) (*User, error) {
	result := *u
	result.Password = ""
	var err error
	result.IsFollow, err = p.relations.IsFollowing(ctx, viewer, u.ID)
	if err != nil {
		return nil, err
	}
	result.Avatar, err = p.media.URL(ctx, "avatar", u.Avatar)
	if err != nil {
		return nil, err
	}
	result.BackgroundImage, err = p.media.URL(ctx, "background", u.BackgroundImage)
	if err != nil {
		return nil, err
	}
	return &result, nil
}
