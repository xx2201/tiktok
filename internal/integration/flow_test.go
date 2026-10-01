//go:build integration

package integration

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	application "github.com/bytedance-youthcamp-jbzx/tiktok/internal/app"
	"github.com/bytedance-youthcamp-jbzx/tiktok/internal/conf"
	"github.com/bytedance-youthcamp-jbzx/tiktok/internal/data"
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func integrationConfig(t *testing.T) *conf.Config {
	t.Helper()
	root := os.Getenv("TIKTOK_PROJECT_ROOT")
	if root == "" {
		t.Fatal("run scripts/integration.ps1 to provide the isolated component environment")
	}
	c, err := conf.Load(filepath.Join(root, "config/kratos.yml"))
	if err != nil {
		t.Fatal(err)
	}
	c.Database.DSN = os.Getenv("TIKTOK_TEST_DSN")
	if c.Database.DSN == "" {
		t.Fatal("TIKTOK_TEST_DSN is required")
	}
	c.Redis.Address = "redis:6379"
	c.Redis.DB = 1
	c.RabbitMQ.URL = "amqp://tiktokRMQ:tiktokRMQ@rabbitmq:5672/kratos-integration"
	c.RabbitMQ.SyncInterval = 100 * time.Millisecond
	c.Registry.Endpoints = []string{"etcd:2379"}
	c.Media.Endpoint = "minio:9000"
	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	for kind, name := range c.Media.Buckets {
		c.Media.Buckets[kind] = name + "-" + stamp
	}
	for name, s := range c.Services {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		address := listener.Addr().String()
		_ = listener.Close()
		s.Address, s.Endpoint = address, address
		s.Name = "tiktok.test." + name + "." + stamp
		c.Services[name] = s
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	private, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	public, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	c.MessageKeys.Public = filepath.Join(t.TempDir(), "public.pem")
	c.MessageKeys.Private = filepath.Join(t.TempDir(), "private.pem")
	if err := os.WriteFile(c.MessageKeys.Public, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: public}), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(c.MessageKeys.Private, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private}), 0600); err != nil {
		t.Fatal(err)
	}
	return c
}

type runningApp struct {
	app   *kratos.App
	done  chan error
	close func()
}

func startApplications(t *testing.T, c *conf.Config) string {
	t.Helper()
	ctx := context.Background()
	logger := log.NewStdLogger(os.Stdout)
	var running []runningApp
	t.Cleanup(func() {
		for i := len(running) - 1; i >= 0; i-- {
			if err := running[i].app.Stop(); err != nil {
				t.Error(err)
			}
			select {
			case err := <-running[i].done:
				if err != nil {
					t.Error(err)
				}
			case <-time.After(15 * time.Second):
				t.Error("application failed to stop")
			}
			running[i].close()
		}
	})
	for _, name := range []string{"user", "video", "comment", "favorite", "relation", "message", "api"} {
		app, cleanup, err := application.Build(ctx, name, c, logger)
		if err != nil {
			t.Fatalf("build %s: %v", name, err)
		}
		done := make(chan error, 1)
		go func() { done <- app.Run() }()
		running = append(running, runningApp{app: app, done: done, close: cleanup})
	}
	base := "http://" + c.Services["api"].Address
	eventually(t, 10*time.Second, func() bool {
		res, err := http.Get(base + "/healthz")
		if err != nil {
			return false
		}
		defer res.Body.Close()
		return res.StatusCode == 200
	})
	return base
}
func eventually(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("condition did not become true before deadline")
}
func call(t *testing.T, method, base, path string, values url.Values) (int, map[string]interface{}) {
	t.Helper()
	req, err := http.NewRequest(method, base+"/douyin"+path+"?"+values.Encode(), nil)
	if err != nil {
		t.Fatal(err)
	}
	return readResponse(t, req)
}
func readResponse(t *testing.T, req *http.Request) (int, map[string]interface{}) {
	t.Helper()
	client := &http.Client{Timeout: 10 * time.Second}
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
func requireOK(t *testing.T, code int, body map[string]interface{}) {
	t.Helper()
	if code != 200 || body["status_code"] != float64(0) {
		t.Fatalf("request failed: %d %#v", code, body)
	}
}
func TestCompleteBusinessFlow(t *testing.T) {
	c := integrationConfig(t)
	db, err := data.New(c.Database)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	media, err := data.NewMedia(c.Media)
	if err != nil {
		t.Fatal(err)
	}
	if err := media.Prepare(context.Background()); err != nil {
		t.Fatal(err)
	}
	// 再次准备资源应保留已有对象，同时证明新环境无需手工上传默认图片。
	if err := media.Prepare(context.Background()); err != nil {
		t.Fatal(err)
	}
	base := startApplications(t, c)
	register := func(label string) (string, string) {
		name := label + fmt.Sprint(time.Now().UnixNano())
		code, body := call(t, "POST", base, "/user/register/", url.Values{"username": {name}, "password": {"secret"}})
		requireOK(t, code, body)
		id := fmt.Sprint(int64(body["user_id"].(float64)))
		token := body["token"].(string)
		code, body = call(t, "POST", base, "/user/login/", url.Values{"username": {name}, "password": {"secret"}})
		requireOK(t, code, body)
		code, _ = call(t, "POST", base, "/user/login/", url.Values{"username": {name}, "password": {"wrong"}})
		if code != 403 {
			t.Fatalf("wrong password accepted: %d", code)
		}
		return id, token
	}
	a, tokenA := register("author")
	b, tokenB := register("viewer")
	_, tokenC := register("stranger")
	code, _ := call(t, "GET", base, "/user/", url.Values{"user_id": {a}})
	if code != 401 {
		t.Fatalf("anonymous user info: %d", code)
	}
	code, body := call(t, "GET", base, "/user/", url.Values{"user_id": {a}, "token": {tokenB}})
	requireOK(t, code, body)
	if body["user"].(map[string]interface{})["is_follow"] != false {
		t.Fatal("incorrect follow status")
	}

	videoPath := filepath.Join(t.TempDir(), "video.mp4")
	command := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "color=c=blue:s=64x64:d=0.3", "-c:v", "mpeg4", "-y", videoPath)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generate video: %v %s", err, output)
	}
	videoBytes, err := os.ReadFile(videoPath)
	if err != nil {
		t.Fatal(err)
	}
	publish := func(content []byte) {
		var buffer bytes.Buffer
		writer := multipart.NewWriter(&buffer)
		_ = writer.WriteField("token", tokenA)
		_ = writer.WriteField("title", "业务验收视频")
		file, err := writer.CreateFormFile("data", "video.mp4")
		if err != nil {
			t.Fatal(err)
		}
		_, _ = file.Write(content)
		_ = writer.Close()
		req, err := http.NewRequest("POST", base+"/douyin/publish/action/", &buffer)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())
		code, body := readResponse(t, req)
		requireOK(t, code, body)
	}
	publish(videoBytes)
	var video map[string]interface{}
	eventually(t, 15*time.Second, func() bool {
		code, body := call(t, "GET", base, "/publish/list/", url.Values{"user_id": {a}})
		if code != 200 {
			return false
		}
		rows := body["video_list"].([]interface{})
		if len(rows) != 1 {
			return false
		}
		video = rows[0].(map[string]interface{})
		return true
	})
	vid := fmt.Sprint(int64(video["id"].(float64)))
	// 私人收藏：立即可见、重试幂等、身份隔离，不改变点赞计数。
	for i := 0; i < 2; i++ {
		code, body = call(t, "POST", base, "/bookmark/action/", url.Values{"token": {tokenB}, "video_id": {vid}, "action_type": {"1"}})
		requireOK(t, code, body)
	}
	code, body = call(t, "GET", base, "/bookmark/list/", url.Values{"token": {tokenB}, "limit": {"1"}, "user_id": {a}})
	requireOK(t, code, body)
	bookmarks := body["video_list"].([]interface{})
	if len(bookmarks) != 1 || bookmarks[0].(map[string]interface{})["id"] != video["id"] || bookmarks[0].(map[string]interface{})["favorite_count"] != float64(0) || body["has_more"] != false {
		t.Fatalf("bookmark visibility or original counters: %#v", body)
	}
	code, body = call(t, "GET", base, "/bookmark/list/", url.Values{"token": {tokenA}, "user_id": {b}})
	requireOK(t, code, body)
	if len(body["video_list"].([]interface{})) != 0 {
		t.Fatal("private bookmarks exposed")
	}
	code, body = call(t, "POST", base, "/bookmark/action/", url.Values{"token": {tokenA}, "video_id": {vid}, "action_type": {"1"}})
	requireOK(t, code, body)
	code, body = call(t, "GET", base, "/bookmark/list/", url.Values{"token": {tokenA}})
	requireOK(t, code, body)
	if len(body["video_list"].([]interface{})) != 1 {
		t.Fatal("author could not bookmark own video")
	}
	code, body = call(t, "POST", base, "/bookmark/action/", url.Values{"token": {tokenA}, "video_id": {vid}, "action_type": {"2"}})
	requireOK(t, code, body)
	code, _ = call(t, "GET", base, "/bookmark/list/", nil)
	if code != 401 {
		t.Fatalf("anonymous bookmarks: %d", code)
	}
	for i := 0; i < 2; i++ {
		code, body = call(t, "POST", base, "/bookmark/action/", url.Values{"token": {tokenB}, "video_id": {vid}, "action_type": {"2"}})
		requireOK(t, code, body)
	}
	code, body = call(t, "GET", base, "/bookmark/list/", url.Values{"token": {tokenB}})
	requireOK(t, code, body)
	if len(body["video_list"].([]interface{})) != 0 {
		t.Fatal("bookmark cancellation not immediately visible")
	}
	code, _ = call(t, "POST", base, "/bookmark/action/", url.Values{"token": {tokenB}, "video_id": {"999999999"}, "action_type": {"1"}})
	if code != 404 {
		t.Fatalf("nonexistent video bookmarked: %d", code)
	}
	author := video["author"].(map[string]interface{})
	for _, mediaURL := range []string{video["play_url"].(string), video["cover_url"].(string), author["avatar"].(string), author["background_image"].(string)} {
		res, err := http.Get(mediaURL)
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != 200 {
			t.Fatalf("media unavailable: %d", res.StatusCode)
		}
		_, _ = io.Copy(io.Discard, res.Body)
		_ = res.Body.Close()
	}
	code, body = call(t, "GET", base, "/feed", nil)
	requireOK(t, code, body)
	if body["next_time"].(float64) <= 0 {
		t.Fatal("Feed cursor missing")
	}
	code, body = call(t, "GET", base, "/feed", url.Values{"latest_time": {fmt.Sprint(int64(body["next_time"].(float64)))}})
	requireOK(t, code, body)
	for _, row := range body["video_list"].([]interface{}) {
		if fmt.Sprint(int64(row.(map[string]interface{})["id"].(float64))) == vid {
			t.Fatal("Feed page repeats video")
		}
	}

	for i := 0; i < 2; i++ {
		code, body = call(t, "POST", base, "/favorite/action/", url.Values{"token": {tokenB}, "video_id": {vid}, "action_type": {"1"}})
		requireOK(t, code, body)
	}
	eventually(t, 5*time.Second, func() bool {
		code, body := call(t, "GET", base, "/favorite/list/", url.Values{"user_id": {b}, "token": {tokenB}})
		if code != 200 {
			return false
		}
		rows := body["video_list"].([]interface{})
		return len(rows) == 1 && rows[0].(map[string]interface{})["favorite_count"] == float64(1)
	})
	code, body = call(t, "POST", base, "/favorite/action/", url.Values{"token": {tokenB}, "video_id": {vid}, "action_type": {"2"}})
	requireOK(t, code, body)
	eventually(t, 5*time.Second, func() bool {
		_, body := call(t, "GET", base, "/favorite/list/", url.Values{"user_id": {b}})
		return len(body["video_list"].([]interface{})) == 0
	})
	code, _ = call(t, "POST", base, "/favorite/action/", url.Values{"token": {tokenB}, "video_id": {vid}, "action_type": {"3"}})
	if code != 400 {
		t.Fatalf("invalid favorite action accepted: %d", code)
	}

	code, body = call(t, "POST", base, "/comment/action/", url.Values{"token": {tokenB}, "video_id": {vid}, "action_type": {"1"}, "comment_text": {"这是评论"}})
	requireOK(t, code, body)
	commentID := fmt.Sprint(int64(body["comment"].(map[string]interface{})["id"].(float64)))
	code, body = call(t, "GET", base, "/comment/list/", url.Values{"video_id": {vid}})
	requireOK(t, code, body)
	commentAuthor := body["comment_list"].([]interface{})[0].(map[string]interface{})["user"].(map[string]interface{})
	if fmt.Sprint(int64(commentAuthor["id"].(float64))) != b || commentAuthor["is_follow"] != false {
		t.Fatal("comment author or follow state is wrong")
	}
	code, _ = call(t, "POST", base, "/comment/action/", url.Values{"token": {tokenC}, "video_id": {vid}, "action_type": {"2"}, "comment_id": {commentID}})
	if code != 403 {
		t.Fatalf("stranger deleted comment: %d", code)
	}
	code, body = call(t, "POST", base, "/comment/action/", url.Values{"token": {tokenA}, "video_id": {vid}, "action_type": {"2"}, "comment_id": {commentID}})
	requireOK(t, code, body)

	follow := func(token, target string) {
		code, body := call(t, "POST", base, "/relation/action/", url.Values{"token": {token}, "to_user_id": {target}, "action_type": {"1"}})
		requireOK(t, code, body)
	}
	follow(tokenA, b)
	eventually(t, 5*time.Second, func() bool {
		_, body := call(t, "GET", base, "/relation/follow/list/", url.Values{"user_id": {a}})
		return len(body["user_list"].([]interface{})) == 1
	})
	code, _ = call(t, "POST", base, "/message/action/", url.Values{"token": {tokenA}, "to_user_id": {b}, "action_type": {"1"}, "content": {"单向关注不能发私信"}})
	if code != 403 {
		t.Fatalf("one-way relationship sent message: %d", code)
	}
	follow(tokenB, a)
	eventually(t, 5*time.Second, func() bool {
		_, body := call(t, "GET", base, "/relation/friend/list/", url.Values{"user_id": {a}, "token": {tokenA}})
		return len(body["user_list"].([]interface{})) == 1
	})
	code, body = call(t, "GET", base, "/relation/follower/list/", url.Values{"user_id": {a}})
	requireOK(t, code, body)
	code, body = call(t, "POST", base, "/message/action/", url.Values{"token": {tokenA}, "to_user_id": {b}, "action_type": {"1"}, "content": {"你好，Kratos"}})
	requireOK(t, code, body)
	code, body = call(t, "GET", base, "/message/chat/", url.Values{"token": {tokenB}, "to_user_id": {a}, "pre_msg_time": {"0"}})
	requireOK(t, code, body)
	msgs := body["message_list"].([]interface{})
	if len(msgs) != 1 || msgs[0].(map[string]interface{})["content"] != "你好，Kratos" {
		t.Fatal("message round trip failed")
	}
	after := fmt.Sprint(int64(msgs[0].(map[string]interface{})["create_time"].(float64)))
	code, body = call(t, "GET", base, "/message/chat/", url.Values{"token": {tokenB}, "to_user_id": {a}, "pre_msg_time": {after}})
	requireOK(t, code, body)
	if len(body["message_list"].([]interface{})) != 0 {
		t.Fatal("incremental poll repeated message")
	}
	code, body = call(t, "GET", base, "/message/chat/", url.Values{"token": {tokenB}, "to_user_id": {a}, "pre_msg_time": {"0"}})
	requireOK(t, code, body)
	if len(body["message_list"].([]interface{})) != 1 {
		t.Fatal("reopening chat lost history")
	}

	publish([]byte("invalid-video"))
	var authorID int64
	_, _ = fmt.Sscan(a, &authorID)
	eventually(t, 10*time.Second, func() bool {
		u, err := db.FindUser(context.Background(), authorID)
		return err == nil && u.WorkCount == 1
	})
	rows, err := db.ListVideos(context.Background(), authorID, 0, 0)
	if err != nil || len(rows) != 1 {
		t.Fatalf("failed upload compensation: %d %v", len(rows), err)
	}
}
