package biz

import (
	"context"
	"fmt"
	"github.com/go-kratos/kratos/v2/log"
	"sync"
	"time"
	"unicode/utf8"
)

type VideoUsecase struct {
	repo     VideoRepository
	profiles *Profiles
	media    Media
	maxBytes int
	log      *log.Helper
	mu       sync.Mutex
	closing  bool
	uploads  sync.WaitGroup
}

func NewVideoUsecase(repo VideoRepository, profiles *Profiles, media Media, maxBytes int, logger log.Logger) *VideoUsecase {
	return &VideoUsecase{repo: repo, profiles: profiles, media: media, maxBytes: maxBytes, log: log.NewHelper(logger)}
}
func (u *VideoUsecase) Decorate(ctx context.Context, video *Video, viewer int64) (*Video, error) {
	result := *video
	var err error
	result.Author, err = u.profiles.User(ctx, video.AuthorID, viewer)
	if err != nil {
		return nil, err
	}
	result.IsFavorite, err = u.repo.IsFavorite(ctx, viewer, video.ID)
	if err != nil {
		return nil, err
	}
	result.PlayURL, err = u.media.URL(ctx, "video", video.PlayURL)
	if err != nil {
		return nil, err
	}
	result.CoverURL, err = u.media.URL(ctx, "cover", video.CoverURL)
	return &result, err
}
func (u *VideoUsecase) List(ctx context.Context, author, before, viewer int64) ([]*Video, int64, error) {
	if author < 0 || before < 0 {
		return nil, 0, ErrInvalid
	}
	if before == 0 {
		before = time.Now().UnixMilli()
	}
	limit := 0
	if author == 0 {
		limit = 30
	}
	rows, err := u.repo.ListVideos(ctx, author, before, limit)
	if err != nil {
		return nil, 0, err
	}
	result := make([]*Video, 0, len(rows))
	for _, row := range rows {
		video, err := u.Decorate(ctx, row, viewer)
		if err != nil {
			return nil, 0, err
		}
		result = append(result, video)
	}
	next := before
	if len(rows) > 0 {
		next = rows[len(rows)-1].CreatedAt.UnixMilli()
	}
	return result, next, nil
}
func (u *VideoUsecase) Publish(ctx context.Context, author int64, title string, data []byte) error {
	if author <= 0 || utf8.RuneCountInString(title) < 1 || utf8.RuneCountInString(title) > 32 || len(data) == 0 || len(data) > u.maxBytes {
		return ErrInvalid
	}
	user, err := u.repo.FindUser(ctx, author)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrNotFound
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.closing {
		return fmt.Errorf("video service is stopping")
	}
	name := fmt.Sprintf("%d_%d", author, time.Now().UnixNano())
	video := &Video{AuthorID: author, Title: title, PlayURL: name + ".mp4", CoverURL: name + ".png", Status: "uploading"}
	if err := u.repo.CreateVideo(ctx, video); err != nil {
		return err
	}
	u.uploads.Add(1)
	go func() {
		defer u.uploads.Done()
		// 请求返回后上传仍需继续，使用独立且有截止时间的上下文。
		background, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		err := u.media.Publish(background, data, video.PlayURL, video.CoverURL)
		if err == nil {
			err = u.repo.CompleteVideo(background, video.ID)
		}
		if err == nil {
			return
		}
		u.log.Errorf("video %d upload failed: %v", video.ID, err)
		cleanup, stop := context.WithTimeout(context.Background(), 30*time.Second)
		defer stop()
		if err := u.media.Remove(cleanup, video.PlayURL, video.CoverURL); err != nil {
			u.log.Errorf("media cleanup failed: %v", err)
		}
		if err := u.repo.DeleteVideo(cleanup, video.ID, author); err != nil {
			u.log.Errorf("video compensation failed: %v", err)
		}
	}()
	return nil
}
func (u *VideoUsecase) Close() {
	u.mu.Lock()
	u.closing = true
	u.mu.Unlock()
	u.uploads.Wait()
}
