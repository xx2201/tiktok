package service

import (
	"context"
	comment "github.com/bytedance-youthcamp-jbzx/tiktok/api/comment/v1"
	"github.com/bytedance-youthcamp-jbzx/tiktok/internal/biz"
	"github.com/bytedance-youthcamp-jbzx/tiktok/pkg/jwt"
)

type CommentService struct {
	comment.UnimplementedCommentServiceServer
	uc     *biz.CommentUsecase
	tokens *jwt.JWT
}

func NewCommentService(uc *biz.CommentUsecase, tokens *jwt.JWT) *CommentService {
	return &CommentService{uc: uc, tokens: tokens}
}
func (s *CommentService) CommentAction(ctx context.Context, req *comment.CommentActionRequest) (*comment.CommentActionResponse, error) {
	viewer, err := identity(s.tokens, req.Token, false)
	if err != nil {
		return nil, err
	}
	res := &comment.CommentActionResponse{StatusMsg: "success"}
	switch req.ActionType {
	case 1:
		row, err := s.uc.Add(ctx, viewer, req.VideoId, req.CommentText)
		if err != nil {
			return nil, err
		}
		res.Comment = commentView(row)
	case 2:
		if err := s.uc.Delete(ctx, viewer, req.VideoId, req.CommentId); err != nil {
			return nil, err
		}
	default:
		return nil, biz.ErrInvalid
	}
	return res, nil
}
func (s *CommentService) CommentList(ctx context.Context, req *comment.CommentListRequest) (*comment.CommentListResponse, error) {
	viewer, err := identity(s.tokens, req.Token, true)
	if err != nil {
		return nil, err
	}
	rows, err := s.uc.List(ctx, req.VideoId, viewer)
	if err != nil {
		return nil, err
	}
	res := &comment.CommentListResponse{StatusMsg: "success", CommentList: make([]*comment.Comment, 0, len(rows))}
	for _, row := range rows {
		res.CommentList = append(res.CommentList, commentView(row))
	}
	return res, nil
}
