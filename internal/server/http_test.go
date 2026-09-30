package server

import (
	"bytes"
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
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type wireFixture struct {
	user.UnimplementedUserServiceServer
	video.UnimplementedVideoServiceServer
	comment.UnimplementedCommentServiceServer
	favorite.UnimplementedFavoriteServiceServer
	relation.UnimplementedRelationServiceServer
	message.UnimplementedMessageServiceServer
}

func (f *wireFixture) Register(_ context.Context, req *user.UserRegisterRequest) (*user.UserRegisterResponse, error) {
	if req.Username != "接手项目" || req.Password != "secret" {
		return nil, errors.BadRequest("BAD_BINDING", "query lost")
	}
	return &user.UserRegisterResponse{UserId: 123, Token: "test-token", StatusMsg: "success"}, nil
}
func (f *wireFixture) Login(context.Context, *user.UserLoginRequest) (*user.UserLoginResponse, error) {
	return nil, errors.ServiceUnavailable("UNAVAILABLE", "private dependency detail")
}
func (f *wireFixture) UserInfo(_ context.Context, req *user.UserInfoRequest) (*user.UserInfoResponse, error) {
	return &user.UserInfoResponse{User: &user.User{Id: req.UserId}}, nil
}
func (f *wireFixture) Feed(_ context.Context, req *video.FeedRequest) (*video.FeedResponse, error) {
	return &video.FeedResponse{NextTime: req.LatestTime, VideoList: []*video.Video{{Id: 42, Author: &user.User{Id: 1}, IsFavorite: false}}}, nil
}
func (f *wireFixture) PublishAction(_ context.Context, req *video.PublishActionRequest) (*video.PublishActionResponse, error) {
	if req.Token != "test-token" || req.Title != "测试视频" || !bytes.Equal(req.Data, []byte("video-data")) {
		return nil, errors.BadRequest("BAD_UPLOAD", "multipart lost")
	}
	return &video.PublishActionResponse{StatusMsg: "success"}, nil
}
func testGateway(t *testing.T, limit float64, burst int, output io.Writer) (*httptest.Server, *wireFixture) {
	t.Helper()
	logger := log.NewStdLogger(output)
	rpc := NewGRPC("127.0.0.1:0", 4<<20, logger)
	fixture := &wireFixture{}
	user.RegisterUserServiceServer(rpc, fixture)
	video.RegisterVideoServiceServer(rpc, fixture)
	comment.RegisterCommentServiceServer(rpc, fixture)
	favorite.RegisterFavoriteServiceServer(rpc, fixture)
	relation.RegisterRelationServiceServer(rpc, fixture)
	message.RegisterMessageServiceServer(rpc, fixture)
	endpoint, err := rpc.Endpoint()
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- rpc.Start(context.Background()) }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := kgrpc.DialInsecure(ctx, kgrpc.WithEndpoint(endpoint.Host))
	if err != nil {
		t.Fatal(err)
	}
	clients := Clients{User: user.NewUserServiceClient(conn), Video: video.NewVideoServiceClient(conn), Favorite: favorite.NewFavoriteServiceClient(conn), Comment: comment.NewCommentServiceClient(conn), Relation: relation.NewRelationServiceClient(conn), Message: message.NewMessageServiceClient(conn)}
	c := &conf.Config{Services: map[string]conf.Service{"api": {Address: "127.0.0.1:0"}}, HTTP: conf.HTTP{Rate: limit, Burst: burst}, Media: conf.Media{MaxBytes: 4 << 20}}
	httpServer := httptest.NewServer(NewHTTP(c, clients, logger))
	t.Cleanup(func() {
		httpServer.Close()
		_ = conn.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := rpc.Stop(ctx); err != nil {
			t.Error(err)
		}
		if err := <-done; err != nil {
			t.Error(err)
		}
	})
	return httpServer, fixture
}
func requestJSON(t *testing.T, client *http.Client, method, url string, body io.Reader, contentType string) (int, map[string]interface{}) {
	t.Helper()
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var payload map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, payload
}
func TestHTTPGRPCRoundTrip(t *testing.T) {
	s, _ := testGateway(t, 1000, 1000, io.Discard)
	code, body := requestJSON(t, s.Client(), "POST", s.URL+"/douyin/user/register/?username=接手项目&password=secret", nil, "")
	if code != 200 || body["status_code"] != float64(0) || body["user_id"] != float64(123) {
		t.Fatalf("register: %d %#v", code, body)
	}
	code, body = requestJSON(t, s.Client(), "GET", s.URL+"/douyin/feed?latest_time=123456", nil, "")
	rows := body["video_list"].([]interface{})
	if code != 200 || body["next_time"] != float64(123456) || rows[0].(map[string]interface{})["is_favorite"] != false {
		t.Fatalf("feed: %d %#v", code, body)
	}
	code, _ = requestJSON(t, s.Client(), "GET", s.URL+"/douyin/user/?user_id=not-an-id", nil, "")
	if code != 400 {
		t.Fatalf("invalid id: %d", code)
	}
	code, body = requestJSON(t, s.Client(), "POST", s.URL+"/douyin/user/login/", nil, "")
	if code != 503 || body["status_code"] != float64(-1) || body["status_msg"] == "private dependency detail" {
		t.Fatalf("RPC failure: %d %#v", code, body)
	}
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	_ = writer.WriteField("token", "test-token")
	_ = writer.WriteField("title", "测试视频")
	file, err := writer.CreateFormFile("data", "test.mp4")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = file.Write([]byte("video-data"))
	_ = writer.Close()
	code, body = requestJSON(t, s.Client(), "POST", s.URL+"/douyin/publish/action/", &buffer, writer.FormDataContentType())
	if code != 200 || body["status_code"] != float64(0) {
		t.Fatalf("upload: %d %#v", code, body)
	}
}
func TestEveryLegacyRouteReachesGRPC(t *testing.T) {
	s, _ := testGateway(t, 1000, 1000, io.Discard)
	for _, route := range []struct{ method, path string }{
		{"GET", "/publish/list/"}, {"POST", "/favorite/action/"}, {"GET", "/favorite/list/"}, {"POST", "/comment/action/"}, {"GET", "/comment/list/"}, {"POST", "/relation/action/"}, {"GET", "/relation/follow/list/"}, {"GET", "/relation/follower/list/"}, {"GET", "/relation/friend/list/"}, {"POST", "/message/action/"}, {"GET", "/message/chat/"},
	} {
		t.Run(route.path, func(t *testing.T) {
			code, body := requestJSON(t, s.Client(), route.method, s.URL+"/douyin"+route.path, nil, "")
			if code != 501 || body["status_code"] != float64(-1) {
				t.Fatalf("route did not reach fixture: %d %#v", code, body)
			}
		})
	}
}
func TestRateLimitAndHealth(t *testing.T) {
	s, _ := testGateway(t, .01, 1, io.Discard)
	_, _ = requestJSON(t, s.Client(), "GET", s.URL+"/douyin/feed", nil, "")
	code, _ := requestJSON(t, s.Client(), "GET", s.URL+"/douyin/feed", nil, "")
	if code != 429 {
		t.Fatalf("rate limit: %d", code)
	}
	code, _ = requestJSON(t, s.Client(), "GET", s.URL+"/healthz", nil, "")
	if code != 200 {
		t.Fatalf("health: %d", code)
	}
}
func TestRequestLogsHideCredentials(t *testing.T) {
	var output bytes.Buffer
	s, _ := testGateway(t, 1000, 1000, &output)
	code, _ := requestJSON(t, s.Client(), "POST", s.URL+"/douyin/user/register/?username=接手项目&password=secret", nil, "")
	if code != 200 {
		t.Fatalf("registration failed: %d", code)
	}
	text := output.String()
	if bytes.Contains([]byte(text), []byte("secret")) || bytes.Contains([]byte(text), []byte("接手项目")) {
		t.Fatal("HTTP or gRPC request log exposed credentials")
	}
	if !bytes.Contains([]byte(text), []byte("operation=")) {
		t.Fatal("request observability was lost")
	}
}
