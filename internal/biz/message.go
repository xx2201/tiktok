package biz

import (
	"context"
	"strings"
	"unicode/utf8"
)

type MessageUsecase struct {
	repo   MessageRepository
	cipher MessageCipher
}

func NewMessageUsecase(repo MessageRepository, cipher MessageCipher) *MessageUsecase {
	return &MessageUsecase{repo: repo, cipher: cipher}
}
func (u *MessageUsecase) Send(ctx context.Context, from, to int64, content string) error {
	if from <= 0 || to <= 0 || from == to || strings.TrimSpace(content) == "" || utf8.RuneCountInString(content) > 255 {
		return ErrInvalid
	}
	forward, err := u.repo.IsFollowing(ctx, from, to)
	if err != nil {
		return err
	}
	backward, err := u.repo.IsFollowing(ctx, to, from)
	if err != nil {
		return err
	}
	if !forward || !backward {
		return ErrForbidden
	}
	encrypted, err := u.cipher.Encrypt(content)
	if err != nil {
		return err
	}
	return u.repo.CreateMessage(ctx, &Message{FromUserID: from, ToUserID: to, Content: encrypted})
}
func (u *MessageUsecase) Chat(ctx context.Context, from, to, after int64) ([]*Message, error) {
	if from <= 0 || to <= 0 || from == to || after < 0 {
		return nil, ErrInvalid
	}
	// 客户端游标决定增量范围，重新打开聊天时传 0 即可重新获取历史。
	rows, err := u.repo.ListMessages(ctx, from, to, after)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		row.Content, err = u.cipher.Decrypt(row.Content)
		if err != nil {
			return nil, err
		}
	}
	return rows, nil
}
