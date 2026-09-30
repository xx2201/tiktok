package biz

import (
	"context"
	"time"
)

type RelationUsecase struct {
	repo      RelationRepository
	publisher ActionPublisher
	profiles  *Profiles
	cipher    MessageCipher
}

func NewRelationUsecase(repo RelationRepository, publisher ActionPublisher, profiles *Profiles, cipher MessageCipher) *RelationUsecase {
	return &RelationUsecase{repo: repo, publisher: publisher, profiles: profiles, cipher: cipher}
}
func (u *RelationUsecase) Action(ctx context.Context, user, target int64, action int32) error {
	if user <= 0 || target <= 0 || user == target || (action != 1 && action != 2) {
		return ErrInvalid
	}
	for _, id := range []int64{user, target} {
		actor, err := u.repo.FindUser(ctx, id)
		if err != nil {
			return err
		}
		if actor == nil {
			return ErrNotFound
		}
	}
	return u.publisher.Publish(ctx, &Action{Kind: "relation", UserID: user, TargetID: target, Type: action, CreatedAt: time.Now().UnixMicro()})
}
func (u *RelationUsecase) List(ctx context.Context, user, viewer int64, followers bool) ([]*User, error) {
	if user <= 0 {
		return nil, ErrInvalid
	}
	var rows []*User
	var err error
	if followers {
		rows, err = u.repo.Followers(ctx, user)
	} else {
		rows, err = u.repo.Following(ctx, user)
	}
	if err != nil {
		return nil, err
	}
	result := make([]*User, 0, len(rows))
	for _, row := range rows {
		profile, err := u.profiles.Decorate(ctx, row, viewer)
		if err != nil {
			return nil, err
		}
		result = append(result, profile)
	}
	return result, nil
}
func (u *RelationUsecase) Friends(ctx context.Context, user, viewer int64) ([]*Friend, error) {
	if user <= 0 {
		return nil, ErrInvalid
	}
	if user != viewer {
		return nil, ErrForbidden
	}
	rows, err := u.repo.Friends(ctx, user)
	if err != nil {
		return nil, err
	}
	result := make([]*Friend, 0, len(rows))
	for _, row := range rows {
		profile, err := u.profiles.Decorate(ctx, row, viewer)
		if err != nil {
			return nil, err
		}
		friend := &Friend{User: profile}
		message, err := u.repo.LatestMessage(ctx, user, row.ID)
		if err != nil {
			return nil, err
		}
		if message != nil {
			friend.Message, err = u.cipher.Decrypt(message.Content)
			if err != nil {
				return nil, err
			}
			if message.FromUserID == user {
				friend.MessageType = 1
			}
		}
		result = append(result, friend)
	}
	return result, nil
}
