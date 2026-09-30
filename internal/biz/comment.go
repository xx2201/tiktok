package biz

import (
	"context"
	"strings"
	"unicode/utf8"
)

type CommentUsecase struct {
	repo     CommentRepository
	profiles *Profiles
}

func NewCommentUsecase(repo CommentRepository, profiles *Profiles) *CommentUsecase {
	return &CommentUsecase{repo: repo, profiles: profiles}
}
func (u *CommentUsecase) Add(ctx context.Context, user, video int64, content string) (*Comment, error) {
	if user <= 0 || video <= 0 || strings.TrimSpace(content) == "" || utf8.RuneCountInString(content) > 255 {
		return nil, ErrInvalid
	}
	v, err := u.repo.FindVideo(ctx, video)
	if err != nil {
		return nil, err
	}
	if v == nil || v.Status != "ready" {
		return nil, ErrNotFound
	}
	author, err := u.profiles.User(ctx, user, user)
	if err != nil {
		return nil, err
	}
	comment := &Comment{UserID: user, VideoID: video, Content: content, User: author}
	if err := u.repo.CreateComment(ctx, comment); err != nil {
		return nil, err
	}
	return comment, nil
}
func (u *CommentUsecase) Delete(ctx context.Context, user, video, id int64) error {
	if user <= 0 || video <= 0 || id <= 0 {
		return ErrInvalid
	}
	c, err := u.repo.FindComment(ctx, id)
	if err != nil {
		return err
	}
	if c == nil {
		return ErrNotFound
	}
	if c.VideoID != video {
		return ErrInvalid
	}
	v, err := u.repo.FindVideo(ctx, video)
	if err != nil {
		return err
	}
	if v == nil {
		return ErrNotFound
	}
	// 评论作者或视频作者任意一方都可以删除；仓储事务中再检查一次。
	if user != c.UserID && user != v.AuthorID {
		return ErrForbidden
	}
	return u.repo.DeleteComment(ctx, id, video, user)
}
func (u *CommentUsecase) List(ctx context.Context, video, viewer int64) ([]*Comment, error) {
	if video <= 0 {
		return nil, ErrInvalid
	}
	comments, err := u.repo.ListComments(ctx, video)
	if err != nil {
		return nil, err
	}
	for _, c := range comments {
		c.User, err = u.profiles.User(ctx, c.UserID, viewer)
		if err != nil {
			return nil, err
		}
	}
	return comments, nil
}
