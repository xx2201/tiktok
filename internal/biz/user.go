package biz

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"github.com/bytedance-youthcamp-jbzx/tiktok/pkg/jwt"
	"math/rand/v2"
	"time"
	"unicode/utf8"
)

type UserUsecase struct {
	repo     UserRepository
	profiles *Profiles
	tokens   *jwt.JWT
}

func NewUserUsecase(repo UserRepository, profiles *Profiles, tokens *jwt.JWT) *UserUsecase {
	return &UserUsecase{repo: repo, profiles: profiles, tokens: tokens}
}

// 保持已有用户密码数据的算法；框架迁移不改写用户凭据。
func passwordDigest(value string) string {
	digest := md5.Sum([]byte(value))
	return hex.EncodeToString(digest[:])
}
func validateCredentials(name, password string) error {
	if utf8.RuneCountInString(name) < 1 || utf8.RuneCountInString(name) > 32 || utf8.RuneCountInString(password) < 1 || utf8.RuneCountInString(password) > 32 {
		return ErrInvalid
	}
	return nil
}
func (u *UserUsecase) token(id int64) (string, error) {
	claims := jwt.CustomClaims{Id: id}
	claims.ExpiresAt = time.Now().Add(24 * time.Hour).Unix()
	return u.tokens.CreateToken(claims)
}
func (u *UserUsecase) Register(ctx context.Context, name, password string) (int64, string, error) {
	if err := validateCredentials(name, password); err != nil {
		return 0, "", err
	}
	existing, err := u.repo.FindUserByName(ctx, name)
	if err != nil {
		return 0, "", err
	}
	if existing != nil {
		return 0, "", ErrConflict
	}
	user := &User{Name: name, Password: passwordDigest(password), Avatar: fmt.Sprintf("default%d.png", rand.IntN(10)), BackgroundImage: "default_background.jpg"}
	if err := u.repo.CreateUser(ctx, user); err != nil {
		return 0, "", err
	}
	token, err := u.token(user.ID)
	return user.ID, token, err
}
func (u *UserUsecase) Login(ctx context.Context, name, password string) (int64, string, error) {
	if err := validateCredentials(name, password); err != nil {
		return 0, "", err
	}
	user, err := u.repo.FindUserByName(ctx, name)
	if err != nil {
		return 0, "", err
	}
	if user == nil || user.Password != passwordDigest(password) {
		return 0, "", ErrForbidden
	}
	token, err := u.token(user.ID)
	return user.ID, token, err
}
func (u *UserUsecase) Info(ctx context.Context, id, viewer int64) (*User, error) {
	if id <= 0 {
		return nil, ErrInvalid
	}
	return u.profiles.User(ctx, id, viewer)
}
