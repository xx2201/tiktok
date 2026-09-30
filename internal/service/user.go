package service

import (
	"context"
	user "github.com/bytedance-youthcamp-jbzx/tiktok/api/user/v1"
	"github.com/bytedance-youthcamp-jbzx/tiktok/internal/biz"
	"github.com/bytedance-youthcamp-jbzx/tiktok/pkg/jwt"
)

type UserService struct {
	user.UnimplementedUserServiceServer
	uc     *biz.UserUsecase
	tokens *jwt.JWT
}

func NewUserService(uc *biz.UserUsecase, tokens *jwt.JWT) *UserService {
	return &UserService{uc: uc, tokens: tokens}
}
func (s *UserService) Register(ctx context.Context, req *user.UserRegisterRequest) (*user.UserRegisterResponse, error) {
	id, token, err := s.uc.Register(ctx, req.Username, req.Password)
	if err != nil {
		return nil, err
	}
	return &user.UserRegisterResponse{StatusMsg: "success", UserId: id, Token: token}, nil
}
func (s *UserService) Login(ctx context.Context, req *user.UserLoginRequest) (*user.UserLoginResponse, error) {
	id, token, err := s.uc.Login(ctx, req.Username, req.Password)
	if err != nil {
		return nil, err
	}
	return &user.UserLoginResponse{StatusMsg: "success", UserId: id, Token: token}, nil
}
func (s *UserService) UserInfo(ctx context.Context, req *user.UserInfoRequest) (*user.UserInfoResponse, error) {
	viewer, err := identity(s.tokens, req.Token, false)
	if err != nil {
		return nil, err
	}
	u, err := s.uc.Info(ctx, req.UserId, viewer)
	if err != nil {
		return nil, err
	}
	return &user.UserInfoResponse{StatusMsg: "success", User: userView(u)}, nil
}
