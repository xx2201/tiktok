package biz

import "context"

type Bookmark struct {
	ID    int64
	Video *Video
}
type BookmarkRepository interface {
	SetBookmark(context.Context, int64, int64, bool) error
	ListBookmarks(context.Context, int64, int64, int) ([]*Bookmark, error)
}
type BookmarkUsecase struct {
	repo   BookmarkRepository
	videos *VideoUsecase
}

func NewBookmarkUsecase(repo BookmarkRepository, videos *VideoUsecase) *BookmarkUsecase {
	return &BookmarkUsecase{repo: repo, videos: videos}
}
func (u *BookmarkUsecase) Action(ctx context.Context, user, video int64, action int32) error {
	if user <= 0 || video <= 0 || (action != 1 && action != 2) {
		return ErrInvalid
	}
	// 可播放状态由仓储在写入事务中检查，避免预检查与写入之间发生变化。
	return u.repo.SetBookmark(ctx, user, video, action == 1)
}
func (u *BookmarkUsecase) List(ctx context.Context, user, cursor int64, limit int) ([]*Video, int64, bool, error) {
	if user <= 0 || cursor < 0 || limit < 0 || limit > 50 {
		return nil, 0, false, ErrInvalid
	}
	if limit == 0 {
		limit = 20
	}
	rows, err := u.repo.ListBookmarks(ctx, user, cursor, limit+1)
	if err != nil {
		return nil, 0, false, err
	}
	more := len(rows) > limit
	if more {
		rows = rows[:limit]
	}
	result := make([]*Video, 0, len(rows))
	var next int64
	for _, row := range rows {
		video, err := u.videos.Decorate(ctx, row.Video, user)
		if err != nil {
			return nil, 0, false, err
		}
		result = append(result, video)
		next = row.ID
	}
	return result, next, more, nil
}
