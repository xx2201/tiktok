//go:build integration

package data

import (
	"context"
	"github.com/bytedance-youthcamp-jbzx/tiktok/internal/biz"
	"github.com/bytedance-youthcamp-jbzx/tiktok/internal/conf"
	"os"
	"sync"
	"testing"
)

type bookmarkTestMedia struct{}

func (bookmarkTestMedia) URL(_ context.Context, kind, name string) (string, error) {
	return kind + "/" + name, nil
}
func (bookmarkTestMedia) Publish(context.Context, []byte, string, string) error { return nil }
func (bookmarkTestMedia) Remove(context.Context, string, string) error          { return nil }

func TestBookmarkReadAfterWriteWithLaggingReplica(t *testing.T) {
	_, author, actor, video := integrationData(t)
	replicaDSN := os.Getenv("TIKTOK_TEST_REPLICA_DSN")
	if replicaDSN == "" {
		t.Fatal("isolated TIKTOK_TEST_REPLICA_DSN is required")
	}
	replica, err := New(conf.Database{DSN: replicaDSN, MaxOpen: 2, MaxIdle: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer replica.Close()
	if err := replica.Migrate(); err != nil {
		t.Fatal(err)
	}
	db, err := New(conf.Database{DSN: os.Getenv("TIKTOK_TEST_DSN"), Replicas: []string{replicaDSN}, MaxOpen: 2, MaxIdle: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	// 独立的空 schema 代表尚未追平的读源，先确认默认读取确实走该源。
	if user, err := db.FindUser(ctx, author.ID); err != nil || user != nil {
		t.Fatalf("replica fixture is not empty: %#v %v", user, err)
	}
	media := bookmarkTestMedia{}
	primary := db.Primary()
	views := biz.NewVideoUsecase(primary, biz.NewProfiles(primary, primary, media), media, 1000, nil)
	uc := biz.NewBookmarkUsecase(primary, views)
	if err := uc.Action(ctx, actor.ID, video.ID, 1); err != nil {
		t.Fatal(err)
	}
	rows, _, _, err := uc.List(ctx, actor.ID, 0, 20)
	if err != nil || len(rows) != 1 || rows[0].ID != video.ID || rows[0].Author.ID != author.ID {
		t.Fatalf("committed bookmark was not immediately readable: %#v %v", rows, err)
	}
	if user, err := db.FindUser(ctx, author.ID); err != nil || user != nil {
		t.Fatalf("primary view changed ordinary replica routing: %#v %v", user, err)
	}
}

func TestBookmarkDatabaseRules(t *testing.T) {
	db, author, actor, first := integrationData(t)
	ctx := context.Background()
	var group sync.WaitGroup
	for i := 0; i < 20; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			if err := db.SetBookmark(ctx, actor.ID, first.ID, true); err != nil {
				t.Error(err)
			}
		}()
	}
	group.Wait()
	rows, err := db.ListBookmarks(ctx, actor.ID, 0, 10)
	if err != nil || len(rows) != 1 || rows[0].Video.ID != first.ID {
		t.Fatalf("duplicate bookmarks: %#v %v", rows, err)
	}
	originalID := rows[0].ID
	second := &biz.Video{AuthorID: author.ID, Title: "second", PlayURL: "b.mp4", CoverURL: "b.png", Status: "ready"}
	if err := db.CreateVideo(ctx, second); err != nil {
		t.Fatal(err)
	}
	if err := db.SetBookmark(ctx, actor.ID, second.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := db.SetBookmark(ctx, actor.ID, first.ID, true); err != nil {
		t.Fatal(err)
	}
	rows, err = db.ListBookmarks(ctx, actor.ID, 0, 1)
	if err != nil || len(rows) != 1 || rows[0].Video.ID != second.ID {
		t.Fatalf("repeat action reordered list: %#v %v", rows, err)
	}
	next, err := db.ListBookmarks(ctx, actor.ID, rows[0].ID, 1)
	if err != nil || len(next) != 1 || next[0].ID != originalID || next[0].Video.ID != first.ID {
		t.Fatalf("cursor lost older bookmark: %#v %v", next, err)
	}
	private, err := db.ListBookmarks(ctx, author.ID, 0, 10)
	if err != nil || len(private) != 0 {
		t.Fatalf("another user's bookmarks exposed: %#v %v", private, err)
	}
	v, err := db.FindVideo(ctx, first.ID)
	if err != nil || v.FavoriteCount != 0 {
		t.Fatalf("bookmark changed likes: %#v %v", v, err)
	}
	u, err := db.FindUser(ctx, actor.ID)
	if err != nil || u.FavoriteCount != 0 {
		t.Fatalf("bookmark changed user likes: %#v %v", u, err)
	}
	u, err = db.FindUser(ctx, author.ID)
	if err != nil || u.TotalFavorited != 0 || u.WorkCount != 2 {
		t.Fatalf("bookmark changed author counters: %#v %v", u, err)
	}
	for i := 0; i < 2; i++ {
		if err := db.SetBookmark(ctx, actor.ID, first.ID, false); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.SetBookmark(ctx, actor.ID, first.ID, true); err != nil {
		t.Fatal(err)
	}
	rows, err = db.ListBookmarks(ctx, actor.ID, 0, 10)
	if err != nil || len(rows) != 2 || rows[0].Video.ID != first.ID || rows[0].ID <= originalID {
		t.Fatalf("recollection did not move to top: %#v %v", rows, err)
	}
	if err := db.db.Model(&Video{}).Where("id = ?", first.ID).Update("status", "uploading").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.SetBookmark(ctx, actor.ID, first.ID, true); err != biz.ErrNotFound {
		t.Fatalf("unready video accepted: %v", err)
	}
	if err := db.db.Delete(&Video{}, second.ID).Error; err != nil {
		t.Fatal(err)
	}
	rows, err = db.ListBookmarks(ctx, actor.ID, 0, 10)
	if err != nil || len(rows) != 0 {
		t.Fatalf("unavailable videos remain visible: %#v %v", rows, err)
	}
	for _, id := range []int64{first.ID, second.ID, 999999999} {
		if err := db.SetBookmark(ctx, actor.ID, id, false); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.SetBookmark(ctx, actor.ID, 999999999, true); err != biz.ErrNotFound {
		t.Fatalf("missing video accepted: %v", err)
	}
}
