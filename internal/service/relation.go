package service

import (
	"context"
	relation "github.com/bytedance-youthcamp-jbzx/tiktok/api/relation/v1"
	"github.com/bytedance-youthcamp-jbzx/tiktok/internal/biz"
	"github.com/bytedance-youthcamp-jbzx/tiktok/pkg/jwt"
)

type RelationService struct {
	relation.UnimplementedRelationServiceServer
	uc     *biz.RelationUsecase
	tokens *jwt.JWT
}

func NewRelationService(uc *biz.RelationUsecase, tokens *jwt.JWT) *RelationService {
	return &RelationService{uc: uc, tokens: tokens}
}
func (s *RelationService) RelationAction(ctx context.Context, req *relation.RelationActionRequest) (*relation.RelationActionResponse, error) {
	viewer, err := identity(s.tokens, req.Token, false)
	if err != nil {
		return nil, err
	}
	if err := s.uc.Action(ctx, viewer, req.ToUserId, req.ActionType); err != nil {
		return nil, err
	}
	return &relation.RelationActionResponse{StatusMsg: "success"}, nil
}
func (s *RelationService) RelationFollowList(ctx context.Context, req *relation.RelationFollowListRequest) (*relation.RelationFollowListResponse, error) {
	viewer, err := identity(s.tokens, req.Token, true)
	if err != nil {
		return nil, err
	}
	rows, err := s.uc.List(ctx, req.UserId, viewer, false)
	if err != nil {
		return nil, err
	}
	return &relation.RelationFollowListResponse{StatusMsg: "success", UserList: userViews(rows)}, nil
}
func (s *RelationService) RelationFollowerList(ctx context.Context, req *relation.RelationFollowerListRequest) (*relation.RelationFollowerListResponse, error) {
	viewer, err := identity(s.tokens, req.Token, true)
	if err != nil {
		return nil, err
	}
	rows, err := s.uc.List(ctx, req.UserId, viewer, true)
	if err != nil {
		return nil, err
	}
	return &relation.RelationFollowerListResponse{StatusMsg: "success", UserList: userViews(rows)}, nil
}
func (s *RelationService) RelationFriendList(ctx context.Context, req *relation.RelationFriendListRequest) (*relation.RelationFriendListResponse, error) {
	viewer, err := identity(s.tokens, req.Token, false)
	if err != nil {
		return nil, err
	}
	rows, err := s.uc.Friends(ctx, req.UserId, viewer)
	if err != nil {
		return nil, err
	}
	res := &relation.RelationFriendListResponse{StatusMsg: "success", UserList: make([]*relation.FriendUser, 0, len(rows))}
	for _, row := range rows {
		u := row.User
		res.UserList = append(res.UserList, &relation.FriendUser{Id: u.ID, Name: u.Name, FollowCount: u.FollowCount, FollowerCount: u.FollowerCount, IsFollow: u.IsFollow, Avatar: u.Avatar, BackgroundImage: u.BackgroundImage, Signature: u.Signature, TotalFavorited: u.TotalFavorited, WorkCount: u.WorkCount, FavoriteCount: u.FavoriteCount, Message: row.Message, MsgType: row.MessageType})
	}
	return res, nil
}
