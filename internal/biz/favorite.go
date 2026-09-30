package biz

import (
	"context"
	"time"
)

type FavoriteUsecase struct {
	repo      FavoriteRepository
	publisher ActionPublisher
	videos    *VideoUsecase
}

func NewFavoriteUsecase(repo FavoriteRepository, publisher ActionPublisher, videos *VideoUsecase) *FavoriteUsecase {
	return &FavoriteUsecase{repo: repo, publisher: publisher, videos: videos}
}
func (u *FavoriteUsecase) Action(ctx context.Context, user, video int64, action int32) error {
	if user <= 0 || video <= 0 || (action != 1 && action != 2) {
		return ErrInvalid
	}
	v, err := u.repo.FindVideo(ctx, video)
	if err != nil {
		return err
	}
	if v == nil || v.Status != "ready" {
		return ErrNotFound
	}
	actor, err := u.repo.FindUser(ctx, user)
	if err != nil {
		return err
	}
	if actor == nil {
		return ErrNotFound
	}
	return u.publisher.Publish(ctx, &Action{Kind: "favorite", UserID: user, TargetID: video, Type: action, CreatedAt: time.Now().UnixMicro()})
}
func (u *FavoriteUsecase) List(ctx context.Context, user, viewer int64) ([]*Video, error) {
	if user <= 0 {
		return nil, ErrInvalid
	}
	rows, err := u.repo.FavoriteVideos(ctx, user)
	if err != nil {
		return nil, err
	}
	result := make([]*Video, 0, len(rows))
	for _, row := range rows {
		v, err := u.videos.Decorate(ctx, row, viewer)
		if err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, nil
}
