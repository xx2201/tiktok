package service

import (
	"context"
	video "github.com/bytedance-youthcamp-jbzx/tiktok/api/video/v1"
	"github.com/bytedance-youthcamp-jbzx/tiktok/internal/biz"
	"github.com/bytedance-youthcamp-jbzx/tiktok/pkg/jwt"
)

type VideoService struct {
	video.UnimplementedVideoServiceServer
	uc     *biz.VideoUsecase
	tokens *jwt.JWT
}

func NewVideoService(uc *biz.VideoUsecase, tokens *jwt.JWT) *VideoService {
	return &VideoService{uc: uc, tokens: tokens}
}
func (s *VideoService) Feed(ctx context.Context, req *video.FeedRequest) (*video.FeedResponse, error) {
	viewer, err := identity(s.tokens, req.Token, true)
	if err != nil {
		return nil, err
	}
	rows, next, err := s.uc.List(ctx, 0, req.LatestTime, viewer)
	if err != nil {
		return nil, err
	}
	return &video.FeedResponse{StatusMsg: "success", VideoList: videoViews(rows), NextTime: next}, nil
}
func (s *VideoService) PublishList(ctx context.Context, req *video.PublishListRequest) (*video.PublishListResponse, error) {
	viewer, err := identity(s.tokens, req.Token, true)
	if err != nil {
		return nil, err
	}
	if req.UserId <= 0 {
		return nil, biz.ErrInvalid
	}
	rows, _, err := s.uc.List(ctx, req.UserId, 0, viewer)
	if err != nil {
		return nil, err
	}
	return &video.PublishListResponse{StatusMsg: "success", VideoList: videoViews(rows)}, nil
}
func (s *VideoService) PublishAction(ctx context.Context, req *video.PublishActionRequest) (*video.PublishActionResponse, error) {
	viewer, err := identity(s.tokens, req.Token, false)
	if err != nil {
		return nil, err
	}
	if err := s.uc.Publish(ctx, viewer, req.Title, req.Data); err != nil {
		return nil, err
	}
	return &video.PublishActionResponse{StatusMsg: "创建记录成功，等待后台上传完成"}, nil
}
