package store

// 標籤的 store 測試，跟 posts_emulator_test.go 一樣要連 Firestore 模擬器，沒有就略過。
// 跑法：make test-store（或見 posts_emulator_test.go 開頭的說明）。

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
)

// newTestTagStore 回傳連到模擬器、而且已經清空的 TagStore。
func newTestTagStore(t *testing.T) (*TagStore, *firestore.Client) {
	t.Helper()
	_, c := newTestStore(t)
	return NewTagStore(c), c
}

// seedTag 直接寫入一份標籤文件，不經過 TagStore。
func seedTag(t *testing.T, c *firestore.Client, slug, name string) {
	t.Helper()
	doc := map[string]any{
		"name":      name,
		"createdAt": time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	if _, err := c.Collection(tagsCollection).Doc(slug).Set(ctx, doc); err != nil {
		t.Fatalf("seedTag %s 失敗：%v", slug, err)
	}
}

// rawTag 直接讀 Firestore 存的原始資料。第二個回傳值是文件存不存在。
func rawTag(t *testing.T, c *firestore.Client, slug string) (map[string]any, bool) {
	t.Helper()
	snap, err := c.Collection(tagsCollection).Doc(slug).Get(ctx)
	if err != nil {
		if !snap.Exists() {
			return nil, false
		}
		t.Fatalf("讀取 tags/%s 失敗：%v", slug, err)
	}
	return snap.Data(), true
}

func mustCreateTag(t *testing.T, s *TagStore, slug, name string) *Tag {
	t.Helper()
	got, err := s.Create(ctx, slug, name)
	if err != nil {
		t.Fatalf("Create(%q, %q) 失敗：%v", slug, name, err)
	}
	return got
}

func tagSlugs(tags []Tag) []string {
	out := make([]string, len(tags))
	for i, tag := range tags {
		out[i] = tag.Slug
	}
	return out
}

// ── Create ─────────────────────────────────────────────────

func TestTagCreateReturnsTag(t *testing.T) {
	s, _ := newTestTagStore(t)
	got := mustCreateTag(t, s, "rust", "Rust")

	// Slug 標了 firestore:"-"，不會自動填。回傳前要自己補上，handler 才拿得到
	if got.Slug != "rust" {
		t.Errorf("Slug = %q，預期 rust（document ID 要填回 struct）", got.Slug)
	}
	if got.Name != "Rust" {
		t.Errorf("Name = %q，預期 Rust", got.Name)
	}
}

func TestTagCreateStoredFields(t *testing.T) {
	s, c := newTestTagStore(t)
	before := time.Now()
	mustCreateTag(t, s, "rust", "Rust")
	after := time.Now()

	raw, ok := rawTag(t, c, "rust")
	if !ok {
		t.Fatal("tags/rust 不存在：document ID 應該就是 slug")
	}
	if raw["name"] != "Rust" {
		t.Errorf("name = %#v，預期 Rust", raw["name"])
	}

	// slug 是 document ID，count 是讀取時現算的，兩個都不能存成欄位
	for _, k := range []string{"slug", "Slug", "count", "Count"} {
		if _, has := raw[k]; has {
			t.Errorf("文件裡多了 %q 欄位：%v", k, sortedKeys(raw))
		}
	}

	created, ok := raw["createdAt"].(time.Time)
	if !ok {
		t.Fatalf("createdAt = %#v，預期是時間", raw["createdAt"])
	}
	// 容許一秒誤差：Firestore 只存到微秒，模擬器的時鐘也可能跟本機差一點
	if created.Before(before.Add(-time.Second)) || created.After(after.Add(time.Second)) {
		t.Errorf("createdAt = %v，預期在 %v 到 %v 之間", created, before, after)
	}
}

func TestTagCreateKeepsNameAsIs(t *testing.T) {
	// 去掉前後空白是 handler 的事；store 收到什麼就存什麼
	s, c := newTestTagStore(t)
	mustCreateTag(t, s, "k8s", "容器編排 Kubernetes")
	raw, _ := rawTag(t, c, "k8s")
	if raw["name"] != "容器編排 Kubernetes" {
		t.Errorf("name = %#v", raw["name"])
	}
}

func TestTagCreateConflict(t *testing.T) {
	s, c := newTestTagStore(t)
	mustCreateTag(t, s, "go", "Go")

	_, err := s.Create(ctx, "go", "改掉的名字")
	if !errors.Is(err, ErrSlugConflict) {
		t.Fatalf("重複建立的 err = %v，預期 ErrSlugConflict", err)
	}

	// 用 Set 的話第二次會把名字蓋掉；Create 必須讓原本的資料原封不動
	raw, _ := rawTag(t, c, "go")
	if raw["name"] != "Go" {
		t.Errorf("name = %#v，重複建立不能覆蓋原本的 Go", raw["name"])
	}
}

func TestTagCreateConflictWithSeededDoc(t *testing.T) {
	// 不是 TagStore 建立的文件（例如手動在 Console 建的）也一樣算撞號
	s, c := newTestTagStore(t)
	seedTag(t, c, "gcp", "GCP")
	if _, err := s.Create(ctx, "gcp", "x"); !errors.Is(err, ErrSlugConflict) {
		t.Errorf("err = %v，預期 ErrSlugConflict", err)
	}
}

func TestTagCreateDoesNotTouchOtherTags(t *testing.T) {
	s, c := newTestTagStore(t)
	seedTag(t, c, "go", "Go")
	mustCreateTag(t, s, "rust", "Rust")
	raw, _ := rawTag(t, c, "go")
	if raw["name"] != "Go" {
		t.Errorf("建立 rust 後 go 的 name 變成 %#v", raw["name"])
	}
}

// ── Get ────────────────────────────────────────────────────

func TestTagGet(t *testing.T) {
	s, c := newTestTagStore(t)
	seedTag(t, c, "go", "Go")

	got, err := s.Get(ctx, "go")
	if err != nil {
		t.Fatal(err)
	}
	if got.Slug != "go" {
		t.Errorf("Slug = %q，預期 go（要從 snap.Ref.ID 填回來）", got.Slug)
	}
	if got.Name != "Go" {
		t.Errorf("Name = %q，預期 Go", got.Name)
	}
	if !got.CreatedAt.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("CreatedAt = %v", got.CreatedAt)
	}
}

func TestTagGetNotFound(t *testing.T) {
	s, _ := newTestTagStore(t)
	if _, err := s.Get(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v，預期 ErrNotFound", err)
	}
}

// 格式不合法的 slug 不能送到 Firestore：
//   - ""、"a/b"、".."、"__reserved__" 會讓 Firestore 回 InvalidArgument，handler 只能回 500
//   - "a/b/c" 是合法的文件路徑，會去讀 tags/a/b/c 這份子集合裡的文件
//
// 這些都不可能是存在的標籤，Get 一開始就用 ValidSlug 擋掉、回 ErrNotFound，
// 跟 PostStore.Get 用 ValidID 是同樣的做法。呼叫 Get 的地方（PostHandler 檢查 tagSlugs、
// Rename、之後公開站的 /tags/{slug}）就都不用自己檢查格式。
var invalidTagSlugs = []string{
	"",
	"a/b",
	"a/b/c",
	"..",
	"__reserved__",
	"Go",
	"Bad Slug",
	"a_b",
	strings.Repeat("a", 61),
}

func TestTagGetInvalidSlug(t *testing.T) {
	s, _ := newTestTagStore(t)
	for _, slug := range invalidTagSlugs {
		t.Run(slug, func(t *testing.T) {
			if _, err := s.Get(ctx, slug); !errors.Is(err, ErrNotFound) {
				t.Errorf("Get(%q) err = %v，預期 ErrNotFound", slug, err)
			}
		})
	}
}

// ── List ───────────────────────────────────────────────────

func TestTagListEmpty(t *testing.T) {
	s, _ := newTestTagStore(t)
	got, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("空資料庫回傳 %v，預期沒有標籤", got)
	}
}

func TestTagListFillsSlugAndName(t *testing.T) {
	s, c := newTestTagStore(t)
	seedTag(t, c, "go", "Go")
	seedTag(t, c, "k8s", "容器編排")

	got, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"go": "Go", "k8s": "容器編排"}
	if len(got) != len(want) {
		t.Fatalf("回傳 %d 筆，預期 %d 筆：%+v", len(got), len(want), got)
	}
	for _, tag := range got {
		if want[tag.Slug] != tag.Name {
			t.Errorf("slug %q 的 name = %q，預期 %q", tag.Slug, tag.Name, want[tag.Slug])
		}
	}
}

// GET /api/tags 依 slug 排序（後台的勾選框、標籤頁都照這個順序）。
// Firestore 不指定排序時就是依 document ID，所以 store 不用多做事，這個測試是防止之後
// 有人加了別的 OrderBy 卻沒發現順序變了。
func TestTagListOrderedBySlug(t *testing.T) {
	s, c := newTestTagStore(t)
	// 故意不照字母順序建立
	for _, slug := range []string{"seo", "bash", "go", "gcp", "css", "2026"} {
		seedTag(t, c, slug, strings.ToUpper(slug))
	}

	got, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"2026", "bash", "css", "gcp", "go", "seo"}
	if !slices.Equal(tagSlugs(got), want) {
		t.Errorf("順序 = %v\n預期    %v", tagSlugs(got), want)
	}
}

func TestTagListOnlyReadsTagsCollection(t *testing.T) {
	// 文章的 tagSlugs 裡有 rust，但沒有 tags/rust：它不是標籤，不能出現在列表裡
	s, c := newTestTagStore(t)
	seedTag(t, c, "go", "Go")
	seed(t, c, fakeID(1), map[string]any{"tagSlugs": []string{"go", "rust"}, "status": "published"})

	got, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(tagSlugs(got), []string{"go"}) {
		t.Errorf("List = %v，預期只有 tags collection 裡的 [go]", tagSlugs(got))
	}
}

func TestTagListAfterCreate(t *testing.T) {
	s, _ := newTestTagStore(t)
	mustCreateTag(t, s, "rust", "Rust")
	mustCreateTag(t, s, "go", "Go")

	got, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(tagSlugs(got), []string{"go", "rust"}) {
		t.Errorf("List = %v，預期 [go rust]", tagSlugs(got))
	}
}

// ── Rename ─────────────────────────────────────────────────

func TestTagRename(t *testing.T) {
	s, c := newTestTagStore(t)
	seedTag(t, c, "go", "go")

	got, err := s.Rename(ctx, "go", "Golang")
	if err != nil {
		t.Fatal(err)
	}
	if got.Slug != "go" || got.Name != "Golang" {
		t.Errorf("回傳 slug/name = %q/%q，預期 go/Golang", got.Slug, got.Name)
	}

	raw, _ := rawTag(t, c, "go")
	if raw["name"] != "Golang" {
		t.Errorf("存進去的 name = %#v，預期 Golang", raw["name"])
	}
}

func TestTagRenameOnlyTouchesName(t *testing.T) {
	s, c := newTestTagStore(t)
	seedTag(t, c, "go", "Go")
	before, _ := rawTag(t, c, "go")

	if _, err := s.Rename(ctx, "go", "Golang"); err != nil {
		t.Fatal(err)
	}
	after, _ := rawTag(t, c, "go")

	// 欄位不能變多或變少（例如 Path 拼錯會多出一個欄位，用 Set 會把 createdAt 洗掉）
	if !slices.Equal(sortedKeys(after), sortedKeys(before)) {
		t.Errorf("改名後的欄位 = %v，改名前是 %v", sortedKeys(after), sortedKeys(before))
	}
	if !after["createdAt"].(time.Time).Equal(before["createdAt"].(time.Time)) {
		t.Errorf("createdAt 從 %v 變成 %v，改名不能動到它", before["createdAt"], after["createdAt"])
	}
}

func TestTagRenameSameName(t *testing.T) {
	s, c := newTestTagStore(t)
	seedTag(t, c, "go", "Go")
	got, err := s.Rename(ctx, "go", "Go")
	if err != nil {
		t.Fatalf("改成一樣的名字應該成功，err = %v", err)
	}
	if got.Name != "Go" {
		t.Errorf("Name = %q", got.Name)
	}
}

func TestTagRenameNotFound(t *testing.T) {
	s, c := newTestTagStore(t)
	if _, err := s.Rename(ctx, "rust", "Rust"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v，預期 ErrNotFound", err)
	}
	// 契約：PUT 不會順便建立標籤
	if _, exists := rawTag(t, c, "rust"); exists {
		t.Error("改名一個不存在的標籤，結果把它建立出來了（要用 Update，不能用 Set）")
	}
}

func TestTagRenameInvalidSlug(t *testing.T) {
	// Rename 不管有沒有先呼叫 Get，格式不合法的 slug 都要是 ErrNotFound，不能是 InvalidArgument
	s, _ := newTestTagStore(t)
	for _, slug := range invalidTagSlugs {
		t.Run(slug, func(t *testing.T) {
			if _, err := s.Rename(ctx, slug, "x"); !errors.Is(err, ErrNotFound) {
				t.Errorf("Rename(%q) err = %v，預期 ErrNotFound", slug, err)
			}
		})
	}
}

func TestTagRenameDoesNotTouchOtherTags(t *testing.T) {
	s, c := newTestTagStore(t)
	seedTag(t, c, "go", "Go")
	seedTag(t, c, "gcp", "GCP")
	if _, err := s.Rename(ctx, "go", "Golang"); err != nil {
		t.Fatal(err)
	}
	raw, _ := rawTag(t, c, "gcp")
	if raw["name"] != "GCP" {
		t.Errorf("改 go 之後 gcp 的 name 變成 %#v", raw["name"])
	}
}

func TestTagRenameDoesNotTouchPosts(t *testing.T) {
	// 改的只是顯示名稱；文章的 tagSlugs 存的是 slug，不用（也不能）跟著改
	s, c := newTestTagStore(t)
	seedTag(t, c, "go", "Go")
	seed(t, c, fakeID(1), map[string]any{"tagSlugs": []string{"go"}})

	if _, err := s.Rename(ctx, "go", "Golang"); err != nil {
		t.Fatal(err)
	}
	raw := rawDoc(t, c, fakeID(1))
	if tags, ok := raw["tagSlugs"].([]any); !ok || len(tags) != 1 || tags[0] != "go" {
		t.Errorf("文章的 tagSlugs = %#v，預期還是 [go]", raw["tagSlugs"])
	}
}
