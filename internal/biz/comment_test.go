package biz

import (
	"context"
	"testing"
)

type commentFixture struct{ deleted bool }

func (*commentFixture) FindUser(context.Context, int64) (*User, error) {
	return &User{ID: 10, Name: "评论作者", Avatar: "avatar.png", BackgroundImage: "background.jpg"}, nil
}
func (*commentFixture) IsFollowing(_ context.Context, viewer, target int64) (bool, error) {
	return viewer == 30 && target == 10, nil
}
func (*commentFixture) FindVideo(context.Context, int64) (*Video, error) {
	return &Video{ID: 1, AuthorID: 20, Status: "ready"}, nil
}
func (*commentFixture) FindComment(context.Context, int64) (*Comment, error) {
	return &Comment{ID: 2, VideoID: 1, UserID: 10}, nil
}
func (*commentFixture) CreateComment(context.Context, *Comment) error { return nil }
func (r *commentFixture) DeleteComment(context.Context, int64, int64, int64) error {
	r.deleted = true
	return nil
}
func (*commentFixture) ListComments(context.Context, int64) ([]*Comment, error) {
	return []*Comment{{ID: 2, VideoID: 1, UserID: 10}}, nil
}

type profileMedia struct{}

func (*profileMedia) URL(_ context.Context, kind, name string) (string, error) {
	return kind + "/" + name, nil
}
func (*profileMedia) Publish(context.Context, []byte, string, string) error { return nil }
func (*profileMedia) Remove(context.Context, string, string) error          { return nil }
func TestCommentDeleteAuthorization(t *testing.T) {
	for _, actor := range []int64{10, 20, 30} {
		repo := &commentFixture{}
		uc := NewCommentUsecase(repo, NewProfiles(repo, repo, &profileMedia{}))
		err := uc.Delete(context.Background(), actor, 1, 2)
		if actor == 30 {
			if err != ErrForbidden || repo.deleted {
				t.Fatalf("stranger deletion accepted: %v", err)
			}
		} else if err != nil || !repo.deleted {
			t.Fatalf("authorized actor %d denied: %v", actor, err)
		}
	}
	repo := &commentFixture{}
	uc := NewCommentUsecase(repo, NewProfiles(repo, repo, &profileMedia{}))
	if err := uc.Delete(context.Background(), 10, 99, 2); err != ErrInvalid || repo.deleted {
		t.Fatalf("mismatched video: %v", err)
	}
}
func TestCommentProfileUsesAuthorAndViewer(t *testing.T) {
	repo := &commentFixture{}
	uc := NewCommentUsecase(repo, NewProfiles(repo, repo, &profileMedia{}))
	rows, err := uc.List(context.Background(), 1, 30)
	if err != nil {
		t.Fatal(err)
	}
	profile := rows[0].User
	if profile.ID != 10 || !profile.IsFollow || profile.BackgroundImage != "background/background.jpg" {
		t.Fatalf("incorrect author profile: %#v", profile)
	}
	rows, err = uc.List(context.Background(), 1, 0)
	if err != nil || rows[0].User.IsFollow {
		t.Fatalf("anonymous follow state: %#v %v", rows, err)
	}
}
