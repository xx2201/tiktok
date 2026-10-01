package server

import (
	"context"
	"errors"
	"github.com/bytedance-youthcamp-jbzx/tiktok/internal/biz"
	"github.com/bytedance-youthcamp-jbzx/tiktok/internal/service"
	"github.com/bytedance-youthcamp-jbzx/tiktok/pkg/jwt"
	"io"
	"net/url"
	"sync"
	"testing"
	"time"
)

type bookmarkWireRepository struct {
	biz.VideoRepository
	mu      sync.Mutex
	user    int64
	collect bool
}

func (r *bookmarkWireRepository) SetBookmark(_ context.Context, user, video int64, collect bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.user, r.collect = user, collect
	if video == 99 {
		return errors.New("private database failure")
	}
	return nil
}
func (*bookmarkWireRepository) ListBookmarks(_ context.Context, user, _ int64, _ int) ([]*biz.Bookmark, error) {
	return []*biz.Bookmark{{ID: 77, Video: &biz.Video{ID: user * 10, AuthorID: user, PlayURL: "a.mp4", CoverURL: "a.png"}}}, nil
}
func (*bookmarkWireRepository) FindUser(_ context.Context, id int64) (*biz.User, error) {
	return &biz.User{ID: id, Avatar: "a.png", BackgroundImage: "b.jpg"}, nil
}
func (*bookmarkWireRepository) IsFollowing(context.Context, int64, int64) (bool, error) {
	return false, nil
}
func (*bookmarkWireRepository) IsFavorite(context.Context, int64, int64) (bool, error) {
	return false, nil
}

type bookmarkWireMedia struct{}

func (*bookmarkWireMedia) URL(_ context.Context, kind, name string) (string, error) {
	return kind + "/" + name, nil
}
func (*bookmarkWireMedia) Publish(context.Context, []byte, string, string) error { return nil }
func (*bookmarkWireMedia) Remove(context.Context, string, string) error          { return nil }
func TestBookmarkIdentityAcrossHTTPGRPC(t *testing.T) {
	repo := &bookmarkWireRepository{}
	media := &bookmarkWireMedia{}
	videos := biz.NewVideoUsecase(repo, biz.NewProfiles(repo, repo, media), media, 1000, nil)
	tokens := jwt.NewJWT([]byte("private-test-key-must-be-at-least-32-bytes"))
	uc := biz.NewBookmarkUsecase(repo, videos)
	s, _ := testGateway(t, 1000, 1000, io.Discard, service.NewVideoService(videos, uc, tokens))
	makeToken := func(id int64) string {
		claims := jwt.CustomClaims{Id: id}
		claims.ExpiresAt = time.Now().Add(time.Hour).Unix()
		token, err := tokens.CreateToken(claims)
		if err != nil {
			t.Fatal(err)
		}
		return url.QueryEscape(token)
	}
	a, b := makeToken(1), makeToken(2)
	for _, route := range []struct{ method, path string }{{"POST", "/bookmark/action/?video_id=10&action_type=1"}, {"GET", "/bookmark/list/"}} {
		code, _ := requestJSON(t, s.Client(), route.method, s.URL+"/douyin"+route.path, nil, "")
		if code != 401 {
			t.Fatalf("anonymous bookmark request: %d", code)
		}
	}
	code, body := requestJSON(t, s.Client(), "POST", s.URL+"/douyin/bookmark/action/?token="+a+"&user_id=2&video_id=10&action_type=1", nil, "")
	if code != 200 || body["status_code"] != float64(0) {
		t.Fatalf("action: %d %#v", code, body)
	}
	repo.mu.Lock()
	if repo.user != 1 || !repo.collect {
		t.Error("client user_id changed bookmark owner")
	}
	repo.mu.Unlock()
	for i, token := range []string{a, b} {
		code, body = requestJSON(t, s.Client(), "GET", s.URL+"/douyin/bookmark/list/?token="+token+"&user_id=2&limit=1", nil, "")
		if code != 200 {
			t.Fatalf("private list: %d %#v", code, body)
		}
		rows := body["video_list"].([]interface{})
		if len(rows) != 1 || rows[0].(map[string]interface{})["id"] != float64((i+1)*10) || body["next_cursor"] != float64(77) || body["has_more"] != false {
			t.Fatalf("wrong private list or encoding: %#v", body)
		}
	}
	for _, query := range []string{"token=" + a + "&limit=51", "token=" + a + "&cursor=-1", "token=forged-token"} {
		code, _ = requestJSON(t, s.Client(), "GET", s.URL+"/douyin/bookmark/list/?"+query, nil, "")
		if code != 400 && code != 401 {
			t.Fatalf("invalid bookmark query accepted: %d", code)
		}
	}
	code, body = requestJSON(t, s.Client(), "POST", s.URL+"/douyin/bookmark/action/?token="+a+"&video_id=99&action_type=1", nil, "")
	if code != 500 || body["status_msg"] != "服务器内部错误" {
		t.Fatalf("write failure was hidden or leaked: %d %#v", code, body)
	}
}
