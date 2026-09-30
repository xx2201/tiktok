package service

import (
	comment "github.com/bytedance-youthcamp-jbzx/tiktok/api/comment/v1"
	user "github.com/bytedance-youthcamp-jbzx/tiktok/api/user/v1"
	video "github.com/bytedance-youthcamp-jbzx/tiktok/api/video/v1"
	"github.com/bytedance-youthcamp-jbzx/tiktok/internal/biz"
	"github.com/bytedance-youthcamp-jbzx/tiktok/pkg/jwt"
	"github.com/go-kratos/kratos/v2/errors"
)

func identity(tokens *jwt.JWT, token string, optional bool) (int64, error) {
	if token == "" && optional {
		return 0, nil
	}
	claims, err := tokens.ParseToken(token)
	if err != nil || claims.Id <= 0 {
		return 0, errors.Unauthorized("UNAUTHORIZED", "token 无效或已过期")
	}
	return claims.Id, nil
}
func userView(u *biz.User) *user.User {
	return &user.User{Id: u.ID, Name: u.Name, FollowCount: u.FollowCount, FollowerCount: u.FollowerCount, IsFollow: u.IsFollow, Avatar: u.Avatar, BackgroundImage: u.BackgroundImage, Signature: u.Signature, TotalFavorited: u.TotalFavorited, WorkCount: u.WorkCount, FavoriteCount: u.FavoriteCount}
}
func videoView(v *biz.Video) *video.Video {
	return &video.Video{Id: v.ID, Author: userView(v.Author), PlayUrl: v.PlayURL, CoverUrl: v.CoverURL, Title: v.Title, FavoriteCount: v.FavoriteCount, CommentCount: v.CommentCount, IsFavorite: v.IsFavorite}
}
func videoViews(rows []*biz.Video) []*video.Video {
	result := make([]*video.Video, 0, len(rows))
	for _, row := range rows {
		result = append(result, videoView(row))
	}
	return result
}
func commentView(c *biz.Comment) *comment.Comment {
	return &comment.Comment{Id: c.ID, User: userView(c.User), Content: c.Content, CreateDate: c.CreatedAt.Format("01-02"), LikeCount: c.LikeCount, TeaseCount: c.TeaseCount}
}
func userViews(rows []*biz.User) []*user.User {
	result := make([]*user.User, 0, len(rows))
	for _, row := range rows {
		result = append(result, userView(row))
	}
	return result
}
