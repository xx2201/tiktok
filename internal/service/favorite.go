package service

import (
	"context"
	favorite "github.com/bytedance-youthcamp-jbzx/tiktok/api/favorite/v1"
	"github.com/bytedance-youthcamp-jbzx/tiktok/internal/biz"
	"github.com/bytedance-youthcamp-jbzx/tiktok/pkg/jwt"
)

type FavoriteService struct {
	favorite.UnimplementedFavoriteServiceServer
	uc     *biz.FavoriteUsecase
	tokens *jwt.JWT
}

func NewFavoriteService(uc *biz.FavoriteUsecase, tokens *jwt.JWT) *FavoriteService {
	return &FavoriteService{uc: uc, tokens: tokens}
}
func (s *FavoriteService) FavoriteAction(ctx context.Context, req *favorite.FavoriteActionRequest) (*favorite.FavoriteActionResponse, error) {
	viewer, err := identity(s.tokens, req.Token, false)
	if err != nil {
		return nil, err
	}
	if err := s.uc.Action(ctx, viewer, req.VideoId, req.ActionType); err != nil {
		return nil, err
	}
	return &favorite.FavoriteActionResponse{StatusMsg: "success"}, nil
}
func (s *FavoriteService) FavoriteList(ctx context.Context, req *favorite.FavoriteListRequest) (*favorite.FavoriteListResponse, error) {
	viewer, err := identity(s.tokens, req.Token, true)
	if err != nil {
		return nil, err
	}
	rows, err := s.uc.List(ctx, req.UserId, viewer)
	if err != nil {
		return nil, err
	}
	return &favorite.FavoriteListResponse{StatusMsg: "success", VideoList: videoViews(rows)}, nil
}
