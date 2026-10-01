package biz

import (
	"context"
	"errors"
	"testing"
)

type bookmarkFixture struct {
	rows    []*Bookmark
	err     error
	actions int
}

func (r *bookmarkFixture) SetBookmark(context.Context, int64, int64, bool) error {
	r.actions++
	return r.err
}
func (r *bookmarkFixture) ListBookmarks(_ context.Context, _ int64, cursor int64, limit int) ([]*Bookmark, error) {
	if r.err != nil {
		return nil, r.err
	}
	result := make([]*Bookmark, 0)
	for _, row := range r.rows {
		if cursor == 0 || row.ID < cursor {
			result = append(result, row)
		}
		if len(result) == limit {
			break
		}
	}
	return result, nil
}

type bookmarkVideoFixture struct{ VideoRepository }

func (*bookmarkVideoFixture) FindUser(_ context.Context, id int64) (*User, error) {
	return &User{ID: id, Avatar: "avatar.png", BackgroundImage: "background.jpg"}, nil
}
func (*bookmarkVideoFixture) IsFollowing(context.Context, int64, int64) (bool, error) {
	return false, nil
}
func (*bookmarkVideoFixture) IsFavorite(context.Context, int64, int64) (bool, error) {
	return false, nil
}
func TestBookmarkPaginationAndFailures(t *testing.T) {
	ctx := context.Background()
	r := &bookmarkFixture{rows: []*Bookmark{
		{ID: 30, Video: &Video{ID: 101, AuthorID: 1, PlayURL: "a.mp4", CoverURL: "a.png"}},
		{ID: 20, Video: &Video{ID: 102, AuthorID: 1, PlayURL: "b.mp4", CoverURL: "b.png"}},
		{ID: 10, Video: &Video{ID: 103, AuthorID: 1, PlayURL: "c.mp4", CoverURL: "c.png"}},
	}}
	videos := &bookmarkVideoFixture{}
	uc := NewBookmarkUsecase(r, NewVideoUsecase(videos, NewProfiles(videos, videos, &profileMedia{}), &profileMedia{}, 1000, nil))
	rows, cursor, more, err := uc.List(ctx, 2, 0, 2)
	if err != nil || len(rows) != 2 || cursor != 20 || !more || rows[0].ID != 101 {
		t.Fatalf("first page: %#v %d %v %v", rows, cursor, more, err)
	}
	rows, cursor, more, err = uc.List(ctx, 2, cursor, 2)
	if err != nil || len(rows) != 1 || cursor != 10 || more || rows[0].ID != 103 {
		t.Fatalf("next page: %#v %d %v %v", rows, cursor, more, err)
	}
	rows, cursor, more, err = uc.List(ctx, 2, cursor, 0)
	if err != nil || len(rows) != 0 || cursor != 0 || more {
		t.Fatalf("empty page: %#v %d %v %v", rows, cursor, more, err)
	}
	for _, request := range []struct {
		user, cursor int64
		limit        int
	}{{0, 0, 20}, {2, -1, 20}, {2, 0, -1}, {2, 0, 51}} {
		if _, _, _, err := uc.List(ctx, request.user, request.cursor, request.limit); err != ErrInvalid {
			t.Fatalf("invalid pagination accepted: %v", err)
		}
	}
	for _, request := range []struct {
		user, video int64
		action      int32
	}{{0, 1, 1}, {2, 0, 1}, {2, 1, 3}} {
		if err := uc.Action(ctx, request.user, request.video, request.action); err != ErrInvalid {
			t.Fatalf("invalid action accepted: %v", err)
		}
	}
	if r.actions != 0 {
		t.Fatal("invalid action reached repository")
	}
	failure := errors.New("database failed")
	r.err = failure
	if err := uc.Action(ctx, 2, 101, 1); !errors.Is(err, failure) {
		t.Fatalf("write failure hidden: %v", err)
	}
	if _, _, _, err := uc.List(ctx, 2, 0, 20); !errors.Is(err, failure) {
		t.Fatalf("read failure hidden: %v", err)
	}
}
