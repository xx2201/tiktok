package server

import (
	"context"
	"encoding/json"
	comment "github.com/bytedance-youthcamp-jbzx/tiktok/api/comment/v1"
	favorite "github.com/bytedance-youthcamp-jbzx/tiktok/api/favorite/v1"
	message "github.com/bytedance-youthcamp-jbzx/tiktok/api/message/v1"
	relation "github.com/bytedance-youthcamp-jbzx/tiktok/api/relation/v1"
	user "github.com/bytedance-youthcamp-jbzx/tiktok/api/user/v1"
	video "github.com/bytedance-youthcamp-jbzx/tiktok/api/video/v1"
	"github.com/bytedance-youthcamp-jbzx/tiktok/internal/conf"
	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"io"
	"net/http"
	"strings"
	"time"
)

type Clients struct {
	User     user.UserServiceClient
	Video    video.VideoServiceClient
	Favorite favorite.FavoriteServiceClient
	Comment  comment.CommentServiceClient
	Relation relation.RelationServiceClient
	Message  message.MessageServiceClient
}

func NewHTTP(c *conf.Config, clients Clients, logger log.Logger) *khttp.Server {
	s := khttp.NewServer(khttp.Address(c.Services["api"].Address), khttp.StrictSlash(false), khttp.Timeout(35*time.Second), khttp.ResponseEncoder(responseEncoder), khttp.ErrorEncoder(errorEncoder), khttp.Middleware(recovery.Recovery(), logging.Server(log.NewFilter(logger, log.FilterKey("args")))), khttp.Filter(normalizePath(), bodyLimit(int64(c.Media.MaxBytes)+(1<<20)), rateLimit(c.HTTP.Rate, c.HTTP.Burst)))
	r := s.Route("/douyin")
	r.POST("/user/register/", bind(func() *user.UserRegisterRequest { return new(user.UserRegisterRequest) }, clients.User.Register))
	r.POST("/user/login/", bind(func() *user.UserLoginRequest { return new(user.UserLoginRequest) }, clients.User.Login))
	r.GET("/user/", bind(func() *user.UserInfoRequest { return new(user.UserInfoRequest) }, clients.User.UserInfo))
	r.GET("/feed", bind(func() *video.FeedRequest { return new(video.FeedRequest) }, clients.Video.Feed))
	r.GET("/publish/list/", bind(func() *video.PublishListRequest { return new(video.PublishListRequest) }, clients.Video.PublishList))
	r.POST("/publish/action/", upload(clients.Video, c.Media.MaxBytes))
	r.POST("/bookmark/action/", bind(func() *video.BookmarkActionRequest { return new(video.BookmarkActionRequest) }, clients.Video.BookmarkAction))
	r.GET("/bookmark/list/", bind(func() *video.BookmarkListRequest { return new(video.BookmarkListRequest) }, clients.Video.BookmarkList))
	r.POST("/favorite/action/", bind(func() *favorite.FavoriteActionRequest { return new(favorite.FavoriteActionRequest) }, clients.Favorite.FavoriteAction))
	r.GET("/favorite/list/", bind(func() *favorite.FavoriteListRequest { return new(favorite.FavoriteListRequest) }, clients.Favorite.FavoriteList))
	r.POST("/comment/action/", bind(func() *comment.CommentActionRequest { return new(comment.CommentActionRequest) }, clients.Comment.CommentAction))
	r.GET("/comment/list/", bind(func() *comment.CommentListRequest { return new(comment.CommentListRequest) }, clients.Comment.CommentList))
	r.POST("/relation/action/", bind(func() *relation.RelationActionRequest { return new(relation.RelationActionRequest) }, clients.Relation.RelationAction))
	r.GET("/relation/follow/list/", bind(func() *relation.RelationFollowListRequest { return new(relation.RelationFollowListRequest) }, clients.Relation.RelationFollowList))
	r.GET("/relation/follower/list/", bind(func() *relation.RelationFollowerListRequest { return new(relation.RelationFollowerListRequest) }, clients.Relation.RelationFollowerList))
	r.GET("/relation/friend/list/", bind(func() *relation.RelationFriendListRequest { return new(relation.RelationFriendListRequest) }, clients.Relation.RelationFriendList))
	r.POST("/message/action/", bind(func() *message.MessageActionRequest { return new(message.MessageActionRequest) }, clients.Message.MessageAction))
	r.GET("/message/chat/", bind(func() *message.MessageChatRequest { return new(message.MessageChatRequest) }, clients.Message.MessageChat))
	s.Route("/").GET("healthz", func(ctx khttp.Context) error { return ctx.JSON(http.StatusOK, map[string]string{"status": "ok"}) })
	return s
}
func bind[Req any, Resp proto.Message](newRequest func() *Req, call func(context.Context, *Req, ...grpc.CallOption) (Resp, error)) khttp.HandlerFunc {
	return func(c khttp.Context) error {
		req := newRequest()
		if err := c.BindQuery(req); err != nil {
			return errors.BadRequest("INVALID_ARGUMENT", "参数格式不合法")
		}
		handler := c.Middleware(func(ctx context.Context, _ interface{}) (interface{}, error) { return call(ctx, req) })
		res, err := handler(c, req)
		if err != nil {
			return err
		}
		return c.Result(http.StatusOK, res)
	}
}
func upload(client video.VideoServiceClient, maxBytes int) khttp.HandlerFunc {
	return func(c khttp.Context) error {
		r := c.Request()
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			return errors.BadRequest("INVALID_UPLOAD", "无法读取上传文件")
		}
		file, header, err := r.FormFile("data")
		if err != nil {
			return errors.BadRequest("INVALID_UPLOAD", "缺少视频文件")
		}
		defer file.Close()
		if header.Size > int64(maxBytes) {
			return errors.New(413, "UPLOAD_TOO_LARGE", "视频文件过大")
		}
		data, err := io.ReadAll(io.LimitReader(file, int64(maxBytes)+1))
		if err != nil {
			return errors.BadRequest("INVALID_UPLOAD", "无法读取视频数据")
		}
		if len(data) > maxBytes {
			return errors.New(413, "UPLOAD_TOO_LARGE", "视频文件过大")
		}
		req := &video.PublishActionRequest{Token: r.FormValue("token"), Title: r.FormValue("title"), Data: data}
		handler := c.Middleware(func(ctx context.Context, _ interface{}) (interface{}, error) { return client.PublishAction(ctx, req) })
		res, err := handler(c, req)
		if err != nil {
			return err
		}
		return c.Result(http.StatusOK, res)
	}
}
func bodyLimit(maxBytes int64) khttp.FilterFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			defer func() {
				if r.MultipartForm != nil {
					_ = r.MultipartForm.RemoveAll()
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func normalizePath() khttp.FilterFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Kratos Router 使用 path.Join 去掉尾斜线，直接规范路径可保留 POST 方法。
			if r.URL.Path != "/" {
				r.URL.Path = strings.TrimSuffix(r.URL.Path, "/")
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Douyin 客户端使用数字 ID 和显式零值，避免 protojson 把 int64 变成字符串。
func responseEncoder(w http.ResponseWriter, _ *http.Request, value interface{}) error {
	if message, ok := value.(proto.Message); ok {
		value = jsonMessage(message.ProtoReflect())
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	return json.NewEncoder(w).Encode(value)
}
func jsonMessage(message protoreflect.Message) map[string]interface{} {
	result := make(map[string]interface{})
	fields := message.Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		field := fields.Get(i)
		value := message.Get(field)
		if field.IsList() {
			list := value.List()
			items := make([]interface{}, 0, list.Len())
			for j := 0; j < list.Len(); j++ {
				items = append(items, jsonValue(field, list.Get(j)))
			}
			result[string(field.Name())] = items
		} else if field.Kind() == protoreflect.MessageKind && !message.Has(field) {
			result[string(field.Name())] = nil
		} else {
			result[string(field.Name())] = jsonValue(field, value)
		}
	}
	return result
}
func jsonValue(field protoreflect.FieldDescriptor, value protoreflect.Value) interface{} {
	if field.Kind() == protoreflect.MessageKind {
		return jsonMessage(value.Message())
	}
	return value.Interface()
}
func errorEncoder(w http.ResponseWriter, _ *http.Request, err error) {
	se := errors.FromError(err)
	code, message := int(se.Code), se.Message
	if code < 400 || code > 599 {
		code = http.StatusInternalServerError
	}
	if code >= 500 {
		message = "服务器内部错误"
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"status_code": -1, "status_msg": message})
}
