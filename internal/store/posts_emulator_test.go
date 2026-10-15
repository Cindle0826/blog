package store

// 這個檔案的測試要連 Firestore 模擬器，沒有模擬器時會自動略過（go test ./... 照樣通過）。
//
// 跑法（模擬器需要 Java 21 以上）：
//
//	gcloud emulators firestore start --host-port=127.0.0.1:8090    # 另開一個終端機
//	FIRESTORE_EMULATOR_HOST=127.0.0.1:8090 go test ./internal/store -count=1
//
// 為什麼不打真的 Firestore：測試會大量建立、刪除文件，而且每個測試開始前都會清空
// 整個資料庫。只要 FIRESTORE_EMULATOR_HOST 沒設，newTestStore 就直接略過，
// 不可能連到正式環境；專案 ID 也用模擬器專用的 demo- 前綴。
//
// 模擬器不會檢查複合索引，所以「查詢缺索引」這類問題這裡測不到，要靠 Terraform。

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
)

const testProject = "demo-cindle-blog-test"

// newTestStore 回傳連到模擬器、而且已經清空的 store。
func newTestStore(t *testing.T) (*PostStore, *firestore.Client) {
	t.Helper()
	host := os.Getenv("FIRESTORE_EMULATOR_HOST")
	if host == "" {
		t.Skip("沒有設定 FIRESTORE_EMULATOR_HOST，略過需要 Firestore 模擬器的測試")
	}

	ctx := context.Background()
	c, err := firestore.NewClient(ctx, testProject)
	if err != nil {
		t.Fatalf("連不到模擬器：%v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	clearEmulator(t, host)
	return NewPostStore(c), c
}

// clearEmulator 刪掉模擬器裡這個專案的所有文件。這是模擬器專用的 API，正式環境沒有。
func clearEmulator(t *testing.T, host string) {
	t.Helper()
	url := fmt.Sprintf("http://%s/emulator/v1/projects/%s/databases/(default)/documents", host, testProject)
	req, _ := http.NewRequest(http.MethodDelete, url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("清空模擬器失敗：%v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("清空模擬器失敗：HTTP %d", resp.StatusCode)
	}
}

var ctx = context.Background()

// rawDoc 直接讀 Firestore 存的原始資料，用來確認「實際寫進去的是什麼」，
// 例如 null 和 [] 的差別、有沒有多出拼錯的欄位。
func rawDoc(t *testing.T, c *firestore.Client, id string) map[string]any {
	t.Helper()
	snap, err := c.Collection(postsCollection).Doc(id).Get(ctx)
	if err != nil {
		t.Fatalf("讀取 %s 失敗：%v", id, err)
	}
	return snap.Data()
}

func countDocs(t *testing.T, c *firestore.Client) int {
	t.Helper()
	snaps, err := c.Collection(postsCollection).Documents(ctx).GetAll()
	if err != nil {
		t.Fatal(err)
	}
	return len(snaps)
}

// seed 直接寫入一份文件，完全控制每個欄位（包括 updatedAt、publishedAt）。
// 透過 Create 的話時間由伺服器決定，排序相關的測試就沒辦法寫。
func seed(t *testing.T, c *firestore.Client, id string, fields map[string]any) {
	t.Helper()
	doc := map[string]any{
		"slug":        strings.ToLower(id),
		"kind":        "post",
		"status":      "draft",
		"title":       "title " + id,
		"summary":     "",
		"markdown":    "",
		"html":        "",
		"toc":         []any{},
		"readingMin":  0,
		"hasCode":     false,
		"tagSlugs":    []string{},
		"cover":       nil,
		"pinned":      false,
		"publishedAt": nil,
		"createdAt":   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		"updatedAt":   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	for k, v := range fields {
		doc[k] = v
	}
	if _, err := c.Collection(postsCollection).Doc(id).Set(ctx, doc); err != nil {
		t.Fatalf("seed %s 失敗：%v", id, err)
	}
}

// fakeID 產生符合 ValidID 的 20 字元 ID。n 決定字典序，方便測試 ID 排序。
func fakeID(n int) string {
	return fmt.Sprintf("seed%016d", n)
}

func day(d int) time.Time {
	return time.Date(2026, 9, d, 12, 0, 0, 0, time.UTC)
}

func mustCreate(t *testing.T, s *PostStore, p Post) *Post {
	t.Helper()
	got, err := s.Create(ctx, p)
	if err != nil {
		t.Fatalf("Create(%+v) 失敗：%v", p, err)
	}
	return got
}

func ids(posts []Post) []string {
	out := make([]string, len(posts))
	for i, p := range posts {
		out[i] = p.ID
	}
	return out
}

// ── Create ─────────────────────────────────────────────────

func TestCreateDefaults(t *testing.T) {
	s, c := newTestStore(t)
	before := time.Now().Add(-time.Second)

	got := mustCreate(t, s, Post{Title: "Hello World"})

	if !ValidID(got.ID) {
		t.Errorf("ID = %q，不是 Firestore 自動產生的格式", got.ID)
	}
	if got.Status != "draft" {
		t.Errorf("Status = %q，新文章一律是 draft", got.Status)
	}
	if got.Kind != "post" {
		t.Errorf("Kind = %q，沒給時預設 post", got.Kind)
	}
	if got.PublishedAt != nil {
		t.Errorf("PublishedAt = %v，草稿要是 nil", got.PublishedAt)
	}
	if got.Slug != "hello-world" {
		t.Errorf("Slug = %q，預期從標題產生 hello-world", got.Slug)
	}
	if got.CreatedAt.Before(before) || got.UpdatedAt.Before(before) {
		t.Errorf("CreatedAt/UpdatedAt = %v / %v，應該是現在", got.CreatedAt, got.UpdatedAt)
	}

	raw := rawDoc(t, c, got.ID)
	if v, ok := raw["publishedAt"]; !ok || v != nil {
		t.Errorf("Firestore 的 publishedAt = %#v（存在=%v），草稿要存 null", v, ok)
	}
	if _, ok := raw["id"]; ok {
		t.Error("ID 是 document ID，不能另外存成欄位（tag 是 firestore:\"-\"）")
	}
}

func TestCreateStatusCannotBeForced(t *testing.T) {
	s, _ := newTestStore(t)
	now := time.Now()
	got := mustCreate(t, s, Post{Title: "x", Status: "published", PublishedAt: &now})
	if got.Status != "draft" || got.PublishedAt != nil {
		t.Errorf("Create 要忽略傳進來的 Status/PublishedAt：得到 %q / %v", got.Status, got.PublishedAt)
	}
}

func TestCreateKeepsFields(t *testing.T) {
	s, c := newTestStore(t)
	cover := &Image{URL: "/images/a.webp", Alt: "封面", Width: 1600, Height: 900}
	got := mustCreate(t, s, Post{
		Title:    "Fields",
		Kind:     "note",
		Summary:  "摘要",
		Markdown: "## 內文",
		TagSlugs: []string{"go", "gcp"},
		Cover:    cover,
		Pinned:   true,
	})

	if got.Kind != "note" || got.Summary != "摘要" || got.Markdown != "## 內文" || !got.Pinned {
		t.Errorf("欄位沒有照原樣存：%+v", got)
	}
	if !slices.Equal(got.TagSlugs, []string{"go", "gcp"}) {
		t.Errorf("TagSlugs = %v", got.TagSlugs)
	}
	if got.Cover == nil || *got.Cover != *cover {
		t.Errorf("Cover = %+v，預期 %+v", got.Cover, cover)
	}

	raw := rawDoc(t, c, got.ID)
	rc, ok := raw["cover"].(map[string]any)
	if !ok {
		t.Fatalf("Firestore 的 cover = %#v，預期 map", raw["cover"])
	}
	for _, k := range []string{"url", "alt", "width", "height"} {
		if _, ok := rc[k]; !ok {
			t.Errorf("cover 少了小寫的 %q 欄位（firestore tag 寫錯？）：%v", k, rc)
		}
	}
}

func TestCreateEmptyTagsStoredAsArray(t *testing.T) {
	s, c := newTestStore(t)
	got := mustCreate(t, s, Post{Title: "x", TagSlugs: []string{}})
	raw := rawDoc(t, c, got.ID)
	if tags, ok := raw["tagSlugs"].([]any); !ok || len(tags) != 0 {
		t.Errorf("Firestore 的 tagSlugs = %#v，空 slice 要存成 []", raw["tagSlugs"])
	}
}

func TestCreateUserSlug(t *testing.T) {
	s, _ := newTestStore(t)
	got := mustCreate(t, s, Post{Title: "Whatever Title", Slug: "my-own-slug"})
	if got.Slug != "my-own-slug" {
		t.Errorf("Slug = %q，使用者填的要照用", got.Slug)
	}
}

func TestCreateUserSlugConflict(t *testing.T) {
	s, c := newTestStore(t)
	mustCreate(t, s, Post{Title: "A", Slug: "taken"})

	_, err := s.Create(ctx, Post{Title: "B", Slug: "taken"})
	if !errors.Is(err, ErrSlugConflict) {
		t.Fatalf("err = %v，預期 ErrSlugConflict", err)
	}
	if n := countDocs(t, c); n != 1 {
		t.Errorf("撞號時不能寫入：現在有 %d 篇", n)
	}
}

// 使用者填的 slug 撞號一律 409，不能偷偷改成 -2。
func TestCreateUserSlugConflictIsNotRenamed(t *testing.T) {
	s, _ := newTestStore(t)
	mustCreate(t, s, Post{Title: "Same", Slug: "same"})
	if _, err := s.Create(ctx, Post{Title: "Same", Slug: "same"}); !errors.Is(err, ErrSlugConflict) {
		t.Errorf("err = %v，預期 ErrSlugConflict", err)
	}
}

func TestCreateGeneratedSlugGetsSuffix(t *testing.T) {
	s, _ := newTestStore(t)
	want := []string{"same-title", "same-title-2", "same-title-3", "same-title-4"}
	for i, w := range want {
		got := mustCreate(t, s, Post{Title: "Same Title"})
		if got.Slug != w {
			t.Errorf("第 %d 篇 Slug = %q，預期 %q", i+1, got.Slug, w)
		}
	}
}

// 有一篇自訂成 "x-2"，自動產生時要跳過它，用 "x-3"。
func TestCreateGeneratedSlugSkipsTakenSuffix(t *testing.T) {
	s, _ := newTestStore(t)
	mustCreate(t, s, Post{Title: "X"})              // x
	mustCreate(t, s, Post{Title: "Y", Slug: "x-2"}) // 使用者自己佔走 x-2
	got := mustCreate(t, s, Post{Title: "X"})
	if got.Slug != "x-3" {
		t.Errorf("Slug = %q，預期跳過已被使用的 x-2，得到 x-3", got.Slug)
	}
}

func TestCreateGeneratedSlugLongTitle(t *testing.T) {
	s, _ := newTestStore(t)
	title := strings.Repeat("a", 80)
	first := mustCreate(t, s, Post{Title: title})
	second := mustCreate(t, s, Post{Title: title})

	if first.Slug != strings.Repeat("a", maxSlugLen) {
		t.Errorf("第一篇 Slug = %q（%d 字），預期截成 60 字", first.Slug, len(first.Slug))
	}
	if second.Slug != strings.Repeat("a", maxSlugLen-2)+"-2" {
		t.Errorf("第二篇 Slug = %q（%d 字），預期截短後加 -2，總長 60", second.Slug, len(second.Slug))
	}
}

func TestCreateChineseTitleSlug(t *testing.T) {
	s, _ := newTestStore(t)
	got := mustCreate(t, s, Post{Title: "純中文標題"})
	want := "post-" + strings.ToLower(got.ID[:8])
	if got.Slug != want {
		t.Errorf("Slug = %q，預期 %q", got.Slug, want)
	}
	if !ValidSlug(got.Slug) {
		t.Errorf("%q 不符合 ValidSlug", got.Slug)
	}
}

// 自動產生的 slug 試了 20 次都被用過時要回一般錯誤（500），不是 ErrSlugConflict（409）。
// 使用者根本沒填 slug，回 409 會讓他以為是自己的問題。
func TestResolveSlugGivesUp(t *testing.T) {
	s, c := newTestStore(t)
	seed(t, c, fakeID(1), map[string]any{"slug": "busy"})
	for n := 2; n <= 20; n++ {
		seed(t, c, fakeID(n), map[string]any{"slug": fmt.Sprintf("busy-%d", n)})
	}

	_, err := s.resolveSlug(ctx, "", "Busy", "aBc123XyZ9kLmN0pQrSt")
	if err == nil {
		t.Fatal("20 個候選都被用了，應該回錯誤")
	}
	if errors.Is(err, ErrSlugConflict) {
		t.Errorf("err = %v，不應該是 ErrSlugConflict", err)
	}

	// 第 20 個空出來就能用
	if _, err := c.Collection(postsCollection).Doc(fakeID(20)).Delete(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := s.resolveSlug(ctx, "", "Busy", "aBc123XyZ9kLmN0pQrSt")
	if err != nil || got != "busy-20" {
		t.Errorf("resolveSlug = %q, %v，預期 busy-20", got, err)
	}
}

func TestSlugTaken(t *testing.T) {
	s, c := newTestStore(t)
	seed(t, c, fakeID(1), map[string]any{"slug": "exists"})

	for _, tt := range []struct {
		slug string
		want bool
	}{
		{"exists", true},
		{"missing", false},
		{"exist", false},    // 前綴不算
		{"exists-2", false}, // 後綴不算
	} {
		got, err := s.slugTaken(ctx, tt.slug)
		if err != nil || got != tt.want {
			t.Errorf("slugTaken(%q) = %v, %v，預期 %v", tt.slug, got, err, tt.want)
		}
	}
}

// ── Get ────────────────────────────────────────────────────

func TestGet(t *testing.T) {
	s, _ := newTestStore(t)
	created := mustCreate(t, s, Post{Title: "Get Me", TagSlugs: []string{"go"}})

	got, err := s.Get(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != created.ID || got.Title != "Get Me" || !slices.Equal(got.TagSlugs, []string{"go"}) {
		t.Errorf("Get = %+v", got)
	}
	if !got.UpdatedAt.Equal(created.UpdatedAt) || !got.CreatedAt.Equal(created.CreatedAt) {
		t.Errorf("Create 回傳的時間要跟之後 Get 的一樣：%v/%v vs %v/%v",
			created.CreatedAt, created.UpdatedAt, got.CreatedAt, got.UpdatedAt)
	}
}

func TestGetNotFound(t *testing.T) {
	s, _ := newTestStore(t)
	for _, id := range []string{
		"aBc123XyZ9kLmN0pQrSt", // 格式正確但不存在
		"",                     // 空的
		"__reserved__",         // Firestore 保留格式
		"short",
		"has/slash/in/it/1234",
	} {
		if _, err := s.Get(ctx, id); !errors.Is(err, ErrNotFound) {
			t.Errorf("Get(%q) err = %v，預期 ErrNotFound", id, err)
		}
	}
}

func TestGetPublishedBySlug(t *testing.T) {
	s, c := newTestStore(t)
	seed(t, c, fakeID(1), map[string]any{"slug": "live", "status": "published", "publishedAt": day(1)})
	seed(t, c, fakeID(2), map[string]any{"slug": "draft-one", "status": "draft"})

	got, err := s.GetPublishedBySlug(ctx, "live")
	if err != nil || got.ID != fakeID(1) {
		t.Errorf("GetPublishedBySlug(live) = %+v, %v", got, err)
	}
	for _, slug := range []string{"draft-one", "missing"} {
		if _, err := s.GetPublishedBySlug(ctx, slug); !errors.Is(err, ErrNotFound) {
			t.Errorf("GetPublishedBySlug(%q) err = %v，草稿和不存在的都要是 ErrNotFound", slug, err)
		}
	}
}

// ── Delete ─────────────────────────────────────────────────

func TestDelete(t *testing.T) {
	s, c := newTestStore(t)
	p := mustCreate(t, s, Post{Title: "Bye"})
	keep := mustCreate(t, s, Post{Title: "Keep"})

	if err := s.Delete(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, p.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("刪除後 Get err = %v，預期 ErrNotFound", err)
	}
	if _, err := s.Get(ctx, keep.ID); err != nil {
		t.Errorf("刪錯文章了：%v", err)
	}
	if n := countDocs(t, c); n != 1 {
		t.Errorf("剩 %d 篇，預期 1", n)
	}

	// 再刪一次：Firestore 預設會當成成功，靠 firestore.Exists 才會回 NotFound
	if err := s.Delete(ctx, p.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("刪除不存在的文章 err = %v，預期 ErrNotFound", err)
	}
}

func TestDeleteInvalidID(t *testing.T) {
	s, _ := newTestStore(t)
	for _, id := range []string{"", "__reserved__", "short"} {
		if err := s.Delete(ctx, id); !errors.Is(err, ErrNotFound) {
			t.Errorf("Delete(%q) err = %v，預期 ErrNotFound", id, err)
		}
	}
}

// ── SetPublished ───────────────────────────────────────────

func TestSetPublishedLifecycle(t *testing.T) {
	s, _ := newTestStore(t)
	p := mustCreate(t, s, Post{Title: "Lifecycle"})
	nextMillisecond()

	// 第一次發布：設定 publishedAt
	pub, err := s.SetPublished(ctx, p.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if pub.Status != "published" || pub.PublishedAt == nil {
		t.Fatalf("發布後 Status/PublishedAt = %q / %v", pub.Status, pub.PublishedAt)
	}
	if !pub.UpdatedAt.After(p.UpdatedAt) {
		t.Errorf("UpdatedAt 沒有前進：%v → %v", p.UpdatedAt, pub.UpdatedAt)
	}
	first := *pub.PublishedAt

	// 回傳值要跟之後 Get 的完全一樣（寫完重新讀，不能自己用 UpdateTime 補）
	again, _ := s.Get(ctx, p.ID)
	if !again.PublishedAt.Equal(first) || !again.UpdatedAt.Equal(pub.UpdatedAt) {
		t.Errorf("回傳值跟 Get 不一致：%v/%v vs %v/%v", first, pub.UpdatedAt, again.PublishedAt, again.UpdatedAt)
	}

	// 退回草稿：publishedAt 不能清掉
	nextMillisecond()
	draft, err := s.SetPublished(ctx, p.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if draft.Status != "draft" {
		t.Errorf("Status = %q，預期 draft", draft.Status)
	}
	if draft.PublishedAt == nil || !draft.PublishedAt.Equal(first) {
		t.Errorf("退回草稿後 PublishedAt = %v，預期保持 %v", draft.PublishedAt, first)
	}

	// 重新發布：日期不能跳到現在
	nextMillisecond()
	re, err := s.SetPublished(ctx, p.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if !re.PublishedAt.Equal(first) {
		t.Errorf("重新發布後 PublishedAt = %v，預期維持第一次的 %v", re.PublishedAt, first)
	}

	// 重複發布也不能動到日期
	nextMillisecond()
	re2, _ := s.SetPublished(ctx, p.ID, true)
	if !re2.PublishedAt.Equal(first) {
		t.Errorf("連續發布兩次後 PublishedAt = %v，預期 %v", re2.PublishedAt, first)
	}
}

func TestSetPublishedDraftTwiceKeepsNull(t *testing.T) {
	s, _ := newTestStore(t)
	p := mustCreate(t, s, Post{Title: "Never"})
	got, err := s.SetPublished(ctx, p.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "draft" || got.PublishedAt != nil {
		t.Errorf("沒發布過的文章退回草稿：%q / %v，publishedAt 要維持 nil", got.Status, got.PublishedAt)
	}
}

func TestSetPublishedNotFound(t *testing.T) {
	s, _ := newTestStore(t)
	for _, id := range []string{"aBc123XyZ9kLmN0pQrSt", "__reserved__", ""} {
		if _, err := s.SetPublished(ctx, id, true); !errors.Is(err, ErrNotFound) {
			t.Errorf("SetPublished(%q) err = %v，預期 ErrNotFound", id, err)
		}
	}
}

// ── Update ─────────────────────────────────────────────────

func TestUpdateOnlyTouchesSentFields(t *testing.T) {
	s, c := newTestStore(t)
	cover := &Image{URL: "/images/a.webp", Alt: "a", Width: 10, Height: 10}
	p := mustCreate(t, s, Post{
		Title: "Original", Kind: "post", Summary: "S", Markdown: "M",
		TagSlugs: []string{"go"}, Cover: cover, Pinned: true,
	})
	keysBefore := sortedKeys(rawDoc(t, c, p.ID))
	nextMillisecond()

	got, err := s.Update(ctx, p.ID, PostPatch{Markdown: ptr("New markdown")})
	if err != nil {
		t.Fatal(err)
	}

	if got.Markdown != "New markdown" {
		t.Errorf("Markdown = %q", got.Markdown)
	}
	if got.Title != "Original" || got.Summary != "S" || got.Kind != "post" || !got.Pinned ||
		!slices.Equal(got.TagSlugs, []string{"go"}) || got.Cover == nil || got.Slug != p.Slug {
		t.Errorf("沒送的欄位被改到了：%+v", got)
	}
	if !got.UpdatedAt.After(p.UpdatedAt) {
		t.Errorf("UpdatedAt 沒有前進：%v → %v", p.UpdatedAt, got.UpdatedAt)
	}
	if !got.CreatedAt.Equal(p.CreatedAt) {
		t.Errorf("CreatedAt 不能變：%v → %v", p.CreatedAt, got.CreatedAt)
	}

	// Path 拼錯不會報錯，只會多出新欄位（之前發生過 "updateAt"）
	if keysAfter := sortedKeys(rawDoc(t, c, p.ID)); !slices.Equal(keysBefore, keysAfter) {
		t.Errorf("Update 後欄位變了：\n前 %v\n後 %v", keysBefore, keysAfter)
	}

	// 回傳值要跟之後 Get 的一樣
	again, _ := s.Get(ctx, p.ID)
	if !again.UpdatedAt.Equal(got.UpdatedAt) {
		t.Errorf("回傳的 UpdatedAt %v 跟 Get 的 %v 不一樣", got.UpdatedAt, again.UpdatedAt)
	}
}

func TestUpdateEachField(t *testing.T) {
	newImg := &Image{URL: "/images/new.webp", Alt: "new", Width: 800, Height: 600}
	tests := []struct {
		name  string
		patch PostPatch
		check func(t *testing.T, p *Post)
	}{
		{"title", PostPatch{Title: ptr("T2")}, func(t *testing.T, p *Post) {
			if p.Title != "T2" {
				t.Errorf("Title = %q", p.Title)
			}
		}},
		{"kind", PostPatch{Kind: ptr("note")}, func(t *testing.T, p *Post) {
			if p.Kind != "note" {
				t.Errorf("Kind = %q", p.Kind)
			}
		}},
		{"summary 清成空字串", PostPatch{Summary: ptr("")}, func(t *testing.T, p *Post) {
			if p.Summary != "" {
				t.Errorf("Summary = %q", p.Summary)
			}
		}},
		{"pinned 改成 false", PostPatch{Pinned: ptr(false)}, func(t *testing.T, p *Post) {
			if p.Pinned {
				t.Error("Pinned 還是 true")
			}
		}},
		{"tagSlugs 換掉", PostPatch{TagSlugs: ptr([]string{"a", "b"})}, func(t *testing.T, p *Post) {
			if !slices.Equal(p.TagSlugs, []string{"a", "b"}) {
				t.Errorf("TagSlugs = %v", p.TagSlugs)
			}
		}},
		{"換封面", PostPatch{Cover: newImg}, func(t *testing.T, p *Post) {
			if p.Cover == nil || *p.Cover != *newImg {
				t.Errorf("Cover = %+v", p.Cover)
			}
		}},
		{"拿掉封面", PostPatch{RemoveCover: true}, func(t *testing.T, p *Post) {
			if p.Cover != nil {
				t.Errorf("Cover = %+v，預期 nil", p.Cover)
			}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := newTestStore(t)
			p := mustCreate(t, s, Post{
				Title: "T", Summary: "S", TagSlugs: []string{"go"}, Pinned: true,
				Cover: &Image{URL: "/images/a.webp", Alt: "a", Width: 1, Height: 1},
			})
			got, err := s.Update(ctx, p.ID, tt.patch)
			if err != nil {
				t.Fatal(err)
			}
			tt.check(t, got)
		})
	}
}

// 清空標籤要存成 []，拿掉封面要存成 null（不是刪掉欄位）。
func TestUpdateStoresEmptyValuesCorrectly(t *testing.T) {
	s, c := newTestStore(t)
	p := mustCreate(t, s, Post{
		Title: "x", TagSlugs: []string{"go"},
		Cover: &Image{URL: "/images/a.webp", Width: 1, Height: 1},
	})

	if _, err := s.Update(ctx, p.ID, PostPatch{TagSlugs: ptr([]string{}), RemoveCover: true}); err != nil {
		t.Fatal(err)
	}
	raw := rawDoc(t, c, p.ID)
	if tags, ok := raw["tagSlugs"].([]any); !ok || len(tags) != 0 {
		t.Errorf("tagSlugs = %#v，預期 []（nil slice 會被存成 null）", raw["tagSlugs"])
	}
	if v, ok := raw["cover"]; !ok || v != nil {
		t.Errorf("cover = %#v（存在=%v），預期 null", v, ok)
	}
}

func TestUpdateNotFound(t *testing.T) {
	s, _ := newTestStore(t)
	for _, tt := range []struct {
		name  string
		id    string
		patch PostPatch
	}{
		{"不存在", "aBc123XyZ9kLmN0pQrSt", PostPatch{Title: ptr("x")}},
		{"不存在而且有送 slug", "aBc123XyZ9kLmN0pQrSt", PostPatch{Slug: ptr("x")}},
		{"保留格式的 ID", "__reserved__", PostPatch{Title: ptr("x")}},
		{"空 ID", "", PostPatch{Title: ptr("x")}},
	} {
		if _, err := s.Update(ctx, tt.id, tt.patch); !errors.Is(err, ErrNotFound) {
			t.Errorf("%s：err = %v，預期 ErrNotFound", tt.name, err)
		}
	}
}

// ── Update 的 slug 規則 ────────────────────────────────────

func TestUpdateSlugOnDraft(t *testing.T) {
	s, _ := newTestStore(t)
	p := mustCreate(t, s, Post{Title: "Draft", Slug: "old-slug"})

	got, err := s.Update(ctx, p.ID, PostPatch{Slug: ptr("new-slug")})
	if err != nil {
		t.Fatal(err)
	}
	if got.Slug != "new-slug" {
		t.Errorf("Slug = %q，從沒發布過的草稿可以改", got.Slug)
	}
	// 舊的 slug 空出來了，別篇可以用
	if taken, _ := s.slugTaken(ctx, "old-slug"); taken {
		t.Error("改名後舊 slug 應該空出來")
	}
}

func TestUpdateSameSlugIsNoop(t *testing.T) {
	s, _ := newTestStore(t)
	p := mustCreate(t, s, Post{Title: "Same", Slug: "same"})
	// 送跟原本一樣的 slug：不能因為「查到自己」而回 ErrSlugConflict
	got, err := s.Update(ctx, p.ID, PostPatch{Slug: ptr("same")})
	if err != nil {
		t.Fatalf("err = %v，送一樣的 slug 不算修改", err)
	}
	if got.Slug != "same" {
		t.Errorf("Slug = %q", got.Slug)
	}
}

func TestUpdateSameSlugOnPublishedIsAllowed(t *testing.T) {
	s, _ := newTestStore(t)
	p := mustCreate(t, s, Post{Title: "Pub", Slug: "pub"})
	if _, err := s.SetPublished(ctx, p.ID, true); err != nil {
		t.Fatal(err)
	}
	// 後台存檔時如果連同沒改過的 slug 一起送，已發布的文章也要能存
	if _, err := s.Update(ctx, p.ID, PostPatch{Slug: ptr("pub"), Title: ptr("Pub 2")}); err != nil {
		t.Errorf("送一樣的 slug 不算修改，已發布也不該被擋：%v", err)
	}
}

func TestUpdateSlugConflict(t *testing.T) {
	s, _ := newTestStore(t)
	mustCreate(t, s, Post{Title: "A", Slug: "taken"})
	p := mustCreate(t, s, Post{Title: "B", Slug: "b"})

	_, err := s.Update(ctx, p.ID, PostPatch{Slug: ptr("taken"), Title: ptr("B renamed")})
	if !errors.Is(err, ErrSlugConflict) {
		t.Fatalf("err = %v，預期 ErrSlugConflict", err)
	}
	// 同一個請求的其他欄位也不能寫進去
	got, _ := s.Get(ctx, p.ID)
	if got.Slug != "b" || got.Title != "B" {
		t.Errorf("被拒絕的 PATCH 不能只成功一半：slug=%q title=%q", got.Slug, got.Title)
	}
}

func TestUpdateSlugLockedAfterPublish(t *testing.T) {
	s, _ := newTestStore(t)
	p := mustCreate(t, s, Post{Title: "Pub", Slug: "pub"})
	if _, err := s.SetPublished(ctx, p.ID, true); err != nil {
		t.Fatal(err)
	}

	_, err := s.Update(ctx, p.ID, PostPatch{Slug: ptr("pub-new"), Title: ptr("Changed")})
	if !errors.Is(err, ErrSlugLocked) {
		t.Fatalf("err = %v，發布過的文章要回 ErrSlugLocked", err)
	}
	got, _ := s.Get(ctx, p.ID)
	if got.Slug != "pub" || got.Title != "Pub" {
		t.Errorf("被拒絕的 PATCH 不能只成功一半：slug=%q title=%q", got.Slug, got.Title)
	}

	// 退回草稿也不會解鎖：判斷依據是 publishedAt，不是 status
	if _, err := s.SetPublished(ctx, p.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update(ctx, p.ID, PostPatch{Slug: ptr("pub-new")}); !errors.Is(err, ErrSlugLocked) {
		t.Errorf("退回草稿後 err = %v，預期仍然是 ErrSlugLocked", err)
	}
}

// 鎖住和撞號同時成立時，先回 ErrSlugLocked：不管新 slug 有沒有人用，發布過就是不能改。
func TestUpdateSlugLockedTakesPriorityOverConflict(t *testing.T) {
	s, _ := newTestStore(t)
	mustCreate(t, s, Post{Title: "Other", Slug: "taken"})
	p := mustCreate(t, s, Post{Title: "Pub", Slug: "pub"})
	if _, err := s.SetPublished(ctx, p.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update(ctx, p.ID, PostPatch{Slug: ptr("taken")}); !errors.Is(err, ErrSlugLocked) {
		t.Errorf("err = %v，預期 ErrSlugLocked", err)
	}
}

// ── List（後台）────────────────────────────────────────────

// seedList 建立 6 篇文章，updatedAt 從 day(1) 到 day(6)。
//
//	ID  kind  status     title                updatedAt
//	1   post  published  Cloud Run 冷啟動     day 1
//	2   note  draft      bash 3.2 的坑         day 2
//	3   post  draft      Go 的 html/template  day 3
//	4   note  published  CLOUD SQL 筆記       day 4
//	5   post  published  Firestore 分頁       day 5
//	6   post  draft      無關的標題           day 6
func seedList(t *testing.T, c *firestore.Client) {
	t.Helper()
	rows := []struct {
		kind, status, title string
		tags                []string
	}{
		{"post", "published", "Cloud Run 冷啟動", []string{"gcp"}},
		{"note", "draft", "bash 3.2 的坑", []string{"bash"}},
		{"post", "draft", "Go 的 html/template", []string{"go"}},
		{"note", "published", "CLOUD SQL 筆記", []string{"gcp"}},
		{"post", "published", "Firestore 分頁", []string{"gcp", "go"}},
		{"post", "draft", "無關的標題", nil},
	}
	for i, r := range rows {
		n := i + 1
		fields := map[string]any{
			"kind": r.kind, "status": r.status, "title": r.title,
			"tagSlugs": r.tags, "updatedAt": day(n),
		}
		if r.tags == nil {
			fields["tagSlugs"] = []string{}
		}
		if r.status == "published" {
			fields["publishedAt"] = day(n)
		}
		seed(t, c, fakeID(n), fields)
	}
}

func TestListFiltersAndOrder(t *testing.T) {
	s, c := newTestStore(t)
	seedList(t, c)

	tests := []struct {
		name                      string
		kind, tag, search, status string
		want                      []string
	}{
		{"全部，updatedAt 新到舊", "", "", "", "",
			[]string{fakeID(6), fakeID(5), fakeID(4), fakeID(3), fakeID(2), fakeID(1)}},
		{"只有草稿", "", "", "", "draft", []string{fakeID(6), fakeID(3), fakeID(2)}},
		{"只有已發布", "", "", "", "published", []string{fakeID(5), fakeID(4), fakeID(1)}},
		{"只有筆記", "note", "", "", "", []string{fakeID(4), fakeID(2)}},
		{"已發布的文章", "post", "", "", "published", []string{fakeID(5), fakeID(1)}},
		{"標籤 gcp", "", "gcp", "", "", []string{fakeID(5), fakeID(4), fakeID(1)}},
		{"搜尋不分大小寫", "", "", "cloud", "", []string{fakeID(4), fakeID(1)}},
		{"搜尋大寫關鍵字", "", "", "CLOUD", "", []string{fakeID(4), fakeID(1)}},
		{"搜尋標題中間的字", "", "", "html", "", []string{fakeID(3)}},
		{"搜尋中文", "", "", "冷啟動", "", []string{fakeID(1)}},
		{"搜尋加上 status", "", "", "cloud", "draft", []string{}},
		{"搜尋沒有結果", "", "", "kubernetes", "", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, total, err := s.List(ctx, tt.kind, tt.tag, tt.search, tt.status, 1, 50)
			if err != nil {
				t.Fatal(err)
			}
			if gotIDs := ids(got); !slices.Equal(gotIDs, tt.want) {
				t.Errorf("得到 %v\n預期 %v", gotIDs, tt.want)
			}
			if total != len(tt.want) {
				t.Errorf("total = %d，預期 %d（過濾、搜尋之後的數量）", total, len(tt.want))
			}
			for _, p := range got {
				if p.ID == "" {
					t.Error("ID 沒有補上")
				}
			}
		})
	}
}

func TestListPaging(t *testing.T) {
	s, c := newTestStore(t)
	seedList(t, c)

	tests := []struct {
		page, perPage int
		want          []string
	}{
		{1, 2, []string{fakeID(6), fakeID(5)}},
		{2, 2, []string{fakeID(4), fakeID(3)}},
		{3, 2, []string{fakeID(2), fakeID(1)}},
		{4, 2, []string{}}, // 超出範圍：空的，不是錯誤
		{1, 4, []string{fakeID(6), fakeID(5), fakeID(4), fakeID(3)}},
		{2, 4, []string{fakeID(2), fakeID(1)}}, // 最後一頁不滿
		{1, 100, []string{fakeID(6), fakeID(5), fakeID(4), fakeID(3), fakeID(2), fakeID(1)}},
		{99, 10, []string{}},
	}
	for _, tt := range tests {
		got, total, err := s.List(ctx, "", "", "", "", tt.page, tt.perPage)
		if err != nil {
			t.Fatal(err)
		}
		if total != 6 {
			t.Errorf("page=%d perPage=%d：total = %d，預期 6（不受分頁影響）", tt.page, tt.perPage, total)
		}
		if got == nil {
			t.Errorf("page=%d perPage=%d：回傳 nil，預期空 slice", tt.page, tt.perPage)
		}
		if gotIDs := ids(got); !slices.Equal(gotIDs, tt.want) {
			t.Errorf("page=%d perPage=%d：得到 %v，預期 %v", tt.page, tt.perPage, gotIDs, tt.want)
		}
	}
}

// 搜尋要在切頁之前做，total 才是「搜尋後還剩幾篇」。
func TestListSearchBeforePaging(t *testing.T) {
	s, c := newTestStore(t)
	for n := 1; n <= 5; n++ {
		title := fmt.Sprintf("other %d", n)
		if n%2 == 1 {
			title = fmt.Sprintf("match %d", n)
		}
		seed(t, c, fakeID(n), map[string]any{"title": title, "updatedAt": day(n)})
	}
	got, total, err := s.List(ctx, "", "", "match", "", 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 {
		t.Errorf("total = %d，預期 3", total)
	}
	if !slices.Equal(ids(got), []string{fakeID(5), fakeID(3)}) {
		t.Errorf("第一頁 = %v，預期每頁都是搜尋結果，不是先切頁再過濾", ids(got))
	}
}

// updatedAt 一樣時用 ID 排序，換頁時順序才固定。
func TestListTieBreaksByID(t *testing.T) {
	s, c := newTestStore(t)
	for _, n := range []int{3, 1, 2} {
		seed(t, c, fakeID(n), map[string]any{"updatedAt": day(1)})
	}
	got, _, err := s.List(ctx, "", "", "", "", 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{fakeID(1), fakeID(2), fakeID(3)}; !slices.Equal(ids(got), want) {
		t.Errorf("得到 %v，預期 %v", ids(got), want)
	}
}

func TestListEmptyDatabase(t *testing.T) {
	s, _ := newTestStore(t)
	got, total, err := s.List(ctx, "", "", "", "", 1, 50)
	if err != nil || total != 0 || got == nil || len(got) != 0 {
		t.Errorf("List = %v, %d, %v，預期空 slice、0、nil", got, total, err)
	}
}

// ── ListPublished / CountPublished（公開頁）────────────────

func TestListPublished(t *testing.T) {
	s, c := newTestStore(t)
	seedList(t, c)
	// 跟 5 號同一天發布：要用 ID 由大到小排在 5 號前面
	seed(t, c, fakeID(7), map[string]any{
		"status": "published", "kind": "post", "title": "同一天", "publishedAt": day(5), "updatedAt": day(5),
	})

	tests := []struct {
		name          string
		kind, tag     string
		page, perPage int
		want          []string
	}{
		{"全部已發布，publishedAt 新到舊", "", "", 1, 10, []string{fakeID(7), fakeID(5), fakeID(4), fakeID(1)}},
		{"第 1 頁", "", "", 1, 2, []string{fakeID(7), fakeID(5)}},
		{"第 2 頁", "", "", 2, 2, []string{fakeID(4), fakeID(1)}},
		{"超出範圍", "", "", 3, 2, nil},
		{"只有文章", "post", "", 1, 10, []string{fakeID(7), fakeID(5), fakeID(1)}},
		{"只有筆記", "note", "", 1, 10, []string{fakeID(4)}},
		{"標籤 go（草稿 3 號不能出現）", "", "go", 1, 10, []string{fakeID(5)}},
		{"標籤 gcp 的文章", "post", "gcp", 1, 10, []string{fakeID(5), fakeID(1)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.ListPublished(ctx, tt.kind, tt.tag, tt.page, tt.perPage)
			if err != nil {
				t.Fatal(err)
			}
			if gotIDs := ids(got); !slices.Equal(gotIDs, tt.want) && !(len(gotIDs) == 0 && len(tt.want) == 0) {
				t.Errorf("得到 %v\n預期 %v", gotIDs, tt.want)
			}
			for _, p := range got {
				if p.Status != "published" {
					t.Errorf("%s 是 %s，公開頁不能出現草稿", p.ID, p.Status)
				}
			}
		})
	}
}

func TestCountPublished(t *testing.T) {
	s, c := newTestStore(t)
	seedList(t, c)

	tests := []struct {
		kind, tag string
		want      int
	}{
		{"", "", 3},
		{"post", "", 2},
		{"note", "", 1},
		{"", "gcp", 3},
		{"", "go", 1}, // 3 號也有 go，但它是草稿
		{"", "bash", 0},
		{"post", "gcp", 2},
		{"", "nope", 0},
	}
	for _, tt := range tests {
		got, err := s.CountPublished(ctx, tt.kind, tt.tag)
		if err != nil {
			t.Fatal(err)
		}
		if got != tt.want {
			t.Errorf("CountPublished(kind=%q, tag=%q) = %d，預期 %d", tt.kind, tt.tag, got, tt.want)
		}
	}
}

// ── 小工具 ─────────────────────────────────────────────────

// nextMillisecond 讓下一次寫入一定落在不同的毫秒。ServerTimestamp 只精確到毫秒，
// 模擬器又很快，Create 完馬上 Update 的話兩個時間可能一樣，「updatedAt 有前進」
// 的檢查就會偶爾失敗。正式環境一次請求要幾十毫秒，不會有這個問題。
func nextMillisecond() { time.Sleep(5 * time.Millisecond) }

func ptr[T any](v T) *T { return &v }

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
