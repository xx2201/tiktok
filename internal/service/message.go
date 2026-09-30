package service

import (
	"context"
	message "github.com/bytedance-youthcamp-jbzx/tiktok/api/message/v1"
	"github.com/bytedance-youthcamp-jbzx/tiktok/internal/biz"
	"github.com/bytedance-youthcamp-jbzx/tiktok/pkg/jwt"
)

type MessageService struct {
	message.UnimplementedMessageServiceServer
	uc     *biz.MessageUsecase
	tokens *jwt.JWT
}

func NewMessageService(uc *biz.MessageUsecase, tokens *jwt.JWT) *MessageService {
	return &MessageService{uc: uc, tokens: tokens}
}
func (s *MessageService) MessageAction(ctx context.Context, req *message.MessageActionRequest) (*message.MessageActionResponse, error) {
	viewer, err := identity(s.tokens, req.Token, false)
	if err != nil {
		return nil, err
	}
	if req.ActionType != 1 {
		return nil, biz.ErrInvalid
	}
	if err := s.uc.Send(ctx, viewer, req.ToUserId, req.Content); err != nil {
		return nil, err
	}
	return &message.MessageActionResponse{StatusMsg: "success"}, nil
}
func (s *MessageService) MessageChat(ctx context.Context, req *message.MessageChatRequest) (*message.MessageChatResponse, error) {
	viewer, err := identity(s.tokens, req.Token, false)
	if err != nil {
		return nil, err
	}
	rows, err := s.uc.Chat(ctx, viewer, req.ToUserId, req.PreMsgTime)
	if err != nil {
		return nil, err
	}
	res := &message.MessageChatResponse{StatusMsg: "success", MessageList: make([]*message.Message, 0, len(rows))}
	for _, row := range rows {
		res.MessageList = append(res.MessageList, &message.Message{Id: row.ID, FromUserId: row.FromUserID, ToUserId: row.ToUserID, Content: row.Content, CreateTime: row.CreatedAt.UnixMilli()})
	}
	return res, nil
}
