//go:build integration

package data

import (
	"context"
	"errors"
	"fmt"
	"github.com/bytedance-youthcamp-jbzx/tiktok/internal/biz"
	"github.com/bytedance-youthcamp-jbzx/tiktok/internal/conf"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"os"
	"sync"
	"testing"
	"time"
)

func integrationData(t *testing.T) (*Data, *biz.User, *biz.User, *biz.Video) {
	t.Helper()
	dsn := os.Getenv("TIKTOK_TEST_DSN")
	if dsn == "" {
		t.Fatal("isolated TIKTOK_TEST_DSN is required")
	}
	db, err := New(conf.Database{DSN: dsn, MaxOpen: 20, MaxIdle: 5})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	stamp := time.Now().UnixNano()
	author := &biz.User{Name: fmt.Sprintf("author_%d", stamp), Password: "test", Avatar: "default.png", BackgroundImage: "background.png"}
	actor := &biz.User{Name: fmt.Sprintf("actor_%d", stamp), Password: "test", Avatar: "default.png", BackgroundImage: "background.png"}
	ctx := context.Background()
	if err := db.CreateUser(ctx, author); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateUser(ctx, actor); err != nil {
		t.Fatal(err)
	}
	video := &biz.Video{AuthorID: author.ID, Title: "transaction test", PlayURL: "test.mp4", CoverURL: "test.png", Status: "ready"}
	if err := db.CreateVideo(ctx, video); err != nil {
		t.Fatal(err)
	}
	return db, author, actor, video
}
func TestConcurrentActionIdempotency(t *testing.T) {
	db, author, actor, video := integrationData(t)
	ctx := context.Background()
	action := &biz.Action{Kind: "favorite", UserID: actor.ID, TargetID: video.ID, Type: 1, CreatedAt: 10}
	var group sync.WaitGroup
	for i := 0; i < 20; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			if err := db.ApplyAction(ctx, action); err != nil {
				t.Error(err)
			}
		}()
	}
	group.Wait()
	u, err := db.FindUser(ctx, actor.ID)
	if err != nil || u.FavoriteCount != 1 {
		t.Fatalf("duplicate actor count: %#v %v", u, err)
	}
	u, err = db.FindUser(ctx, author.ID)
	if err != nil || u.TotalFavorited != 1 {
		t.Fatalf("duplicate author count: %#v %v", u, err)
	}
	v, err := db.FindVideo(ctx, video.ID)
	if err != nil || v.FavoriteCount != 1 {
		t.Fatalf("duplicate video count: %#v %v", v, err)
	}
	action.Type, action.CreatedAt = 2, 11
	if err := db.ApplyAction(ctx, action); err != nil {
		t.Fatal(err)
	}
	action.Type, action.CreatedAt = 1, 10
	if err := db.ApplyAction(ctx, action); err != nil {
		t.Fatal(err)
	}
	v, err = db.FindVideo(ctx, video.ID)
	if err != nil || v.FavoriteCount != 0 {
		t.Fatalf("older event resurrected favorite: %#v %v", v, err)
	}
	for _, sequence := range []int64{12, 13} {
		action.Type, action.CreatedAt = 2, sequence
		if err := db.ApplyAction(ctx, action); err != nil {
			t.Fatal(err)
		}
	}
	v, err = db.FindVideo(ctx, video.ID)
	if err != nil || v.FavoriteCount != 0 {
		t.Fatalf("duplicate cancel underflow: %#v %v", v, err)
	}
}
func TestRedisPendingSurvivesFailureAndNewAction(t *testing.T) {
	db, _, actor, video := integrationData(t)
	ctx := context.Background()
	rdb := redis.NewClient(&redis.Options{Addr: "redis:6379", Password: "tiktokRedis"})
	defer rdb.Close()
	worker := &Actions{kind: "favorite", redis: rdb, db: db}
	action := &biz.Action{Kind: "favorite", UserID: actor.ID, TargetID: video.ID, Type: 1, CreatedAt: 20}
	key := fmt.Sprintf("tiktok:favorite:%d:%d:w", actor.ID, video.ID)
	t.Cleanup(func() { _ = rdb.Del(ctx, key, key[:len(key)-1]+"r").Err() })
	if err := worker.Cache(ctx, action); err != nil {
		t.Fatal(err)
	}
	if err := rdb.Del(ctx, key[:len(key)-1]+"r").Err(); err != nil {
		t.Fatal(err)
	}
	action.Type, action.CreatedAt = 2, 21
	if err := worker.Cache(ctx, action); err != nil {
		t.Fatalf("expired read key caused failure: %v", err)
	}
	action.Type, action.CreatedAt = 1, 22
	if err := worker.Cache(ctx, action); err != nil {
		t.Fatal(err)
	}
	injected := errors.New("injected video update failure")
	if err := db.db.Callback().Update().Before("gorm:update").Register("test_video_failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "videos" {
			tx.AddError(injected)
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.db.Callback().Update().Remove("test_video_failure") })
	if err := worker.Sync(ctx); !errors.Is(err, injected) {
		t.Fatalf("failure did not propagate: %v", err)
	}
	if exists, err := rdb.Exists(ctx, key).Result(); err != nil || exists != 1 {
		t.Fatalf("pending action was lost: %d %v", exists, err)
	}
	if liked, err := db.IsFavorite(ctx, actor.ID, video.ID); err != nil || liked {
		t.Fatalf("failed transaction partially committed: %v %v", liked, err)
	}
	if err := db.db.Callback().Update().Remove("test_video_failure"); err != nil {
		t.Fatal(err)
	}
	if err := worker.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	if liked, err := db.IsFavorite(ctx, actor.ID, video.ID); err != nil || !liked {
		t.Fatalf("recovery did not apply action: %v %v", liked, err)
	}
	old := "old-value"
	if err := rdb.Set(ctx, key, old, 0).Err(); err != nil {
		t.Fatal(err)
	}
	if err := rdb.Set(ctx, key, "new-value", 0).Err(); err != nil {
		t.Fatal(err)
	}
	if removed, err := removeApplied.Run(ctx, rdb, []string{key}, old).Int(); err != nil || removed != 0 {
		t.Fatalf("new pending state removed: %d %v", removed, err)
	}
	if value, err := rdb.Get(ctx, key).Result(); err != nil || value != "new-value" {
		t.Fatalf("CAS failed: %s %v", value, err)
	}
}
