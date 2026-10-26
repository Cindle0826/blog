package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"cindle.dev/blog/internal/store"
)

// ── 假的 store ─────────────────────────────────────────────

// fakeTagStore 實作 tagsStore，跟 fakeStore 一樣記下參數、回傳測試指定的結果。
type fakeTagStore struct {
	calls []string

	// List
	listRet []store.Tag
	listErr error

	// Create
	createSlug, createName string
	createRet              *store.Tag
	createErr              error

	// Rename
	renameSlug, renameName string
	renameRet              *store.Tag
	renameErr              error
}

func (f *fakeTagStore) List(_ context.Context) ([]store.Tag, error) {
	f.calls = append(f.calls, "List")
	return f.listRet, f.listErr
}

func (f *fakeTagStore) Create(_ context.Context, slug, name string) (*store.Tag, error) {
	f.calls = append(f.calls, "Create")
	f.createSlug, f.createName = slug, name
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.createRet != nil {
		return f.createRet, nil
	}
	// 沒指定回傳值時，模擬 store：原樣回傳
	return &store.Tag{Slug: slug, Name: name}, nil
}

func (f *fakeTagStore) Rename(_ context.Context, slug, name string) (*store.Tag, error) {
	f.calls = append(f.calls, "Rename")
	f.renameSlug, f.renameName = slug, name
	// 跟真的 TagStore 一樣：格式不合法的 slug 就是不存在
	if !store.ValidSlug(slug) {
		return nil, store.ErrNotFound
	}
	if f.renameErr != nil {
		return nil, f.renameErr
	}
	if f.renameRet != nil {
		return f.renameRet, nil
	}
	return &store.Tag{Slug: slug, Name: name}, nil
}

// fakeCounter 實作 tagCounter（真的是 PostStore.CountPublished）。
// counts 沒列到的標籤回 0。
type fakeCounter struct {
	tags   []string // 依呼叫順序記下 tag 參數
	kinds  []string // 同上，記下 kind 參數
	counts map[string]int
	err    error
}

func (c *fakeCounter) CountPublished(_ context.Context, kind, tag string) (int, error) {
	c.tags = append(c.tags, tag)
	c.kinds = append(c.kinds, kind)
	if c.err != nil {
		return 0, c.err
	}
	return c.counts[tag], nil
}

// ── 測試工具 ───────────────────────────────────────────────

// newTagMux 用跟 cmd/server/main.go 一樣的路由，r.PathValue("slug") 才拿得到值。
func newTagMux(f *fakeTagStore, c *fakeCounter) *http.ServeMux {
	h := NewTagHandler(f, c)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /tags", h.List)
	mux.HandleFunc("POST /tags", h.Create)
	mux.HandleFunc("PUT /tags/{slug}", h.Update)
	return mux
}

func doTag(t *testing.T, f *fakeTagStore, c *fakeCounter, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	newTagMux(f, c).ServeHTTP(rec, req)
	return rec
}

func assertTagStoreNotCalled(t *testing.T, f *fakeTagStore) {
	t.Helper()
	if len(f.calls) != 0 {
		t.Errorf("store 被呼叫了 %v，驗證失敗時不應該碰到 store", f.calls)
	}
}

// 契約的標籤物件只有這三個欄位（API.md「GET /api/tags」）。createdAt 不能出現。
var tagKeys = []string{"count", "name", "slug"}

// assertTag 檢查一個標籤物件的欄位與值。
func assertTag(t *testing.T, m map[string]any, slug, name string, count int) {
	t.Helper()
	if got := sortedKeys(m); !slices.Equal(got, tagKeys) {
		t.Errorf("欄位 = %v\n預期    %v", got, tagKeys)
	}
	if m["slug"] != slug {
		t.Errorf("slug = %#v，預期 %q", m["slug"], slug)
	}
	if m["name"] != name {
		t.Errorf("name = %#v，預期 %q", m["name"], name)
	}
	// JSON 數字解成 float64；同時確認不是字串或 null
	if n, ok := m["count"].(float64); !ok || int(n) != count {
		t.Errorf("count = %#v，預期 %d", m["count"], count)
	}
}

// ── 請求 body 共通的錯誤 ──────────────────────────────────

// 兩個收 JSON body 的標籤端點，對壞掉的 body 要跟文章的端點有一樣的反應。
func TestTagBodyErrors(t *testing.T) {
	endpoints := []struct {
		name, method, target string
	}{
		{"POST", "POST", "/tags"},
		{"PUT", "PUT", "/tags/go"},
	}
	bodies := []struct {
		name   string
		body   string
		status int
	}{
		{"沒有 body", "", http.StatusBadRequest},
		{"不是 JSON", "hello", http.StatusBadRequest},
		{"JSON 被截斷", `{"name":`, http.StatusBadRequest},
		{"只有左括號", `{`, http.StatusBadRequest},
		{"是陣列不是物件", `[1,2]`, http.StatusBadRequest},
		{"是字串不是物件", `"Go"`, http.StatusBadRequest},
		{"超過 1 MB", `{"name":"` + strings.Repeat("a", 1<<20) + `"}`, http.StatusRequestEntityTooLarge},
	}

	for _, ep := range endpoints {
		for _, b := range bodies {
			t.Run(ep.name+"/"+b.name, func(t *testing.T) {
				f, c := &fakeTagStore{}, &fakeCounter{}
				rec := doTag(t, f, c, ep.method, ep.target, b.body)
				assertError(t, rec, b.status, "invalid_request")
				assertTagStoreNotCalled(t, f)
			})
		}
	}
}

// ── GET /tags ──────────────────────────────────────────────

func TestTagListResponse(t *testing.T) {
	f := &fakeTagStore{listRet: []store.Tag{
		{Slug: "gcp", Name: "GCP"},
		{Slug: "go", Name: "Go"},
		{Slug: "k8s", Name: "容器編排"},
	}}
	c := &fakeCounter{counts: map[string]int{"gcp": 3, "go": 12}}
	rec := doTag(t, f, c, "GET", "/tags", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("狀態碼 = %d，預期 200；body = %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q，預期 application/json", ct)
	}

	m := decodeMap(t, rec)
	if got := sortedKeys(m); !slices.Equal(got, []string{"items"}) {
		t.Errorf("外層欄位 = %v，預期只有 items", got)
	}
	items, ok := m["items"].([]any)
	if !ok || len(items) != 3 {
		t.Fatalf("items = %#v，預期 3 筆", m["items"])
	}

	// 順序照 store 回傳的（store 依 slug 排序），handler 不能自己重排
	assertTag(t, items[0].(map[string]any), "gcp", "GCP", 3)
	assertTag(t, items[1].(map[string]any), "go", "Go", 12)
	assertTag(t, items[2].(map[string]any), "k8s", "容器編排", 0)
}

func TestTagListCountsEveryTag(t *testing.T) {
	f := &fakeTagStore{listRet: []store.Tag{{Slug: "a", Name: "A"}, {Slug: "b", Name: "B"}}}
	c := &fakeCounter{}
	doTag(t, f, c, "GET", "/tags", "")

	if !slices.Equal(c.tags, []string{"a", "b"}) {
		t.Errorf("CountPublished 的 tag 參數 = %v，預期每個標籤各一次 [a b]", c.tags)
	}
	// kind 要傳空字串：count 是文章加筆記的總數，不能只算其中一種
	for i, k := range c.kinds {
		if k != "" {
			t.Errorf("第 %d 次 CountPublished 的 kind = %q，預期空字串（不分類型）", i, k)
		}
	}
}

func TestTagListEmptyItemsIsArray(t *testing.T) {
	for _, tt := range []struct {
		name string
		ret  []store.Tag
	}{
		{"store 回 nil", nil},
		{"store 回空 slice", []store.Tag{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeTagStore{listRet: tt.ret}
			c := &fakeCounter{}
			rec := doTag(t, f, c, "GET", "/tags", "")
			if rec.Code != http.StatusOK {
				t.Fatalf("狀態碼 = %d；body = %s", rec.Code, rec.Body.String())
			}
			m := decodeMap(t, rec)
			if items, ok := m["items"].([]any); !ok || len(items) != 0 {
				t.Errorf("items = %#v，沒有標籤時要是 []，不能是 null", m["items"])
			}
			if len(c.tags) != 0 {
				t.Errorf("沒有標籤時不應該算 count，卻呼叫了 %v", c.tags)
			}
		})
	}
}

func TestTagListStoreError(t *testing.T) {
	f := &fakeTagStore{listErr: errors.New("firestore: deadline exceeded")}
	rec := doTag(t, f, &fakeCounter{}, "GET", "/tags", "")
	msg := assertError(t, rec, http.StatusInternalServerError, "internal")
	if strings.Contains(msg, "firestore") {
		t.Errorf("500 的訊息不能透露內部錯誤：%q", msg)
	}
}

func TestTagListCountError(t *testing.T) {
	// 標籤讀到了，但算 count 失敗：不能回 200 配上錯的數字
	f := &fakeTagStore{listRet: []store.Tag{{Slug: "go", Name: "Go"}}}
	c := &fakeCounter{err: errors.New("firestore: aggregation failed")}
	rec := doTag(t, f, c, "GET", "/tags", "")
	msg := assertError(t, rec, http.StatusInternalServerError, "internal")
	if strings.Contains(msg, "firestore") {
		t.Errorf("500 的訊息不能透露內部錯誤：%q", msg)
	}
}

// ── POST /tags ─────────────────────────────────────────────

func TestTagCreateSuccess(t *testing.T) {
	f, c := &fakeTagStore{}, &fakeCounter{}
	rec := doTag(t, f, c, "POST", "/tags", `{"slug":"rust","name":"Rust"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("狀態碼 = %d，預期 201；body = %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q，預期 application/json", ct)
	}
	if f.createSlug != "rust" || f.createName != "Rust" {
		t.Errorf("傳給 store 的 slug/name = %q/%q，預期 rust/Rust", f.createSlug, f.createName)
	}
	assertTag(t, decodeMap(t, rec), "rust", "Rust", 0)
}

func TestTagCreateResponseComesFromStore(t *testing.T) {
	// 回應要用 store 回傳的那份，不是把請求原樣吐回去
	f := &fakeTagStore{createRet: &store.Tag{Slug: "rust", Name: "Rust（store）"}}
	rec := doTag(t, f, &fakeCounter{}, "POST", "/tags", `{"slug":"rust","name":"Rust"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("狀態碼 = %d；body = %s", rec.Code, rec.Body.String())
	}
	assertTag(t, decodeMap(t, rec), "rust", "Rust（store）", 0)
}

func TestTagCreateTrims(t *testing.T) {
	tests := []struct {
		name, body, wantSlug, wantName string
	}{
		{"name 前後空白", `{"slug":"rust","name":"  Rust  "}`, "rust", "Rust"},
		{"name 前後有 tab 與換行", `{"slug":"rust","name":"\tRust\n"}`, "rust", "Rust"},
		// 跟 POST /posts 的 slug 一樣，先去掉前後空白再檢查格式
		{"slug 前後空白", `{"slug":"  rust  ","name":"Rust"}`, "rust", "Rust"},
		{"name 中間的空白要保留", `{"slug":"gcp","name":"Google Cloud"}`, "gcp", "Google Cloud"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeTagStore{}
			rec := doTag(t, f, &fakeCounter{}, "POST", "/tags", tt.body)
			if rec.Code != http.StatusCreated {
				t.Fatalf("狀態碼 = %d；body = %s", rec.Code, rec.Body.String())
			}
			if f.createSlug != tt.wantSlug || f.createName != tt.wantName {
				t.Errorf("傳給 store 的 slug/name = %q/%q，預期 %q/%q",
					f.createSlug, f.createName, tt.wantSlug, tt.wantName)
			}
		})
	}
}

func TestTagCreateValidSlugs(t *testing.T) {
	for _, slug := range []string{
		"a",
		"go",
		"k8s",
		"2026",
		"cloud-run",
		"a-b-c-d",
		strings.Repeat("a", 60), // 剛好 60 字
	} {
		t.Run(slug, func(t *testing.T) {
			f := &fakeTagStore{}
			rec := doTag(t, f, &fakeCounter{}, "POST", "/tags", `{"slug":"`+slug+`","name":"x"}`)
			if rec.Code != http.StatusCreated {
				t.Fatalf("slug %q 應該合法，狀態碼 = %d；body = %s", slug, rec.Code, rec.Body.String())
			}
			if f.createSlug != slug {
				t.Errorf("傳給 store 的 slug = %q，預期 %q", f.createSlug, slug)
			}
		})
	}
}

func TestTagCreateNameIsNotLimitedToASCII(t *testing.T) {
	f := &fakeTagStore{}
	rec := doTag(t, f, &fakeCounter{}, "POST", "/tags", `{"slug":"k8s","name":"容器編排 Kubernetes"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("狀態碼 = %d；body = %s", rec.Code, rec.Body.String())
	}
	if f.createName != "容器編排 Kubernetes" {
		t.Errorf("name = %q", f.createName)
	}
}

func TestTagCreateValidation(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"空物件", `{}`},
		{"沒有 slug", `{"name":"Rust"}`},
		{"slug 空字串", `{"slug":"","name":"Rust"}`},
		{"slug 只有空白", `{"slug":"   ","name":"Rust"}`},
		{"slug 是 null", `{"slug":null,"name":"Rust"}`},
		{"slug 型別錯", `{"slug":123,"name":"Rust"}`},
		{"slug 有大寫", `{"slug":"Rust","name":"Rust"}`},
		{"slug 中間有空白", `{"slug":"rust lang","name":"Rust"}`},
		{"slug 有斜線", `{"slug":"a/b","name":"x"}`},
		{"slug 路徑穿越", `{"slug":"../x","name":"x"}`},
		{"slug 是 ..", `{"slug":"..","name":"x"}`},
		{"slug 是 Firestore 保留 ID", `{"slug":"__reserved__","name":"x"}`},
		{"slug 有底線", `{"slug":"a_b","name":"x"}`},
		{"slug 有點", `{"slug":"node.js","name":"x"}`},
		{"slug 連字號開頭", `{"slug":"-x","name":"x"}`},
		{"slug 連字號結尾", `{"slug":"x-","name":"x"}`},
		{"slug 連續連字號", `{"slug":"a--b","name":"x"}`},
		{"slug 是中文", `{"slug":"容器","name":"x"}`},
		{"slug 超過 60 字", `{"slug":"` + strings.Repeat("a", 61) + `","name":"x"}`},
		{"沒有 name", `{"slug":"rust"}`},
		{"name 空字串", `{"slug":"rust","name":""}`},
		{"name 只有空白", `{"slug":"rust","name":"   "}`},
		{"name 是 null", `{"slug":"rust","name":null}`},
		{"name 型別錯", `{"slug":"rust","name":1}`},
		{"送 count（不允許）", `{"slug":"rust","name":"Rust","count":3}`},
		{"送 order（不允許）", `{"slug":"rust","name":"Rust","order":1}`},
		{"送 createdAt（不允許）", `{"slug":"rust","name":"Rust","createdAt":"2026-01-01T00:00:00Z"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeTagStore{}
			rec := doTag(t, f, &fakeCounter{}, "POST", "/tags", tt.body)
			assertError(t, rec, http.StatusBadRequest, "invalid_request")
			assertTagStoreNotCalled(t, f)
		})
	}
}

func TestTagCreateUnknownFieldMessageNamesTheField(t *testing.T) {
	rec := doTag(t, &fakeTagStore{}, &fakeCounter{}, "POST", "/tags", `{"slug":"rust","name":"Rust","order":1}`)
	msg := assertError(t, rec, http.StatusBadRequest, "invalid_request")
	if !strings.Contains(msg, "order") {
		t.Errorf("訊息 %q 應該指出是哪個欄位", msg)
	}
}

func TestTagCreateConflict(t *testing.T) {
	for _, tt := range []struct {
		name string
		err  error
	}{
		{"ErrSlugConflict", store.ErrSlugConflict},
		// store 之後如果用 fmt.Errorf("...: %w", ErrSlugConflict) 包一層，handler 也要認得
		{"包了一層", errors.Join(errors.New("context"), store.ErrSlugConflict)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeTagStore{createErr: tt.err}
			rec := doTag(t, f, &fakeCounter{}, "POST", "/tags", `{"slug":"go","name":"Go"}`)
			msg := assertError(t, rec, http.StatusConflict, "slug_conflict")
			if !strings.Contains(msg, "go") {
				t.Errorf("訊息 %q 應該包含撞到的 slug", msg)
			}
		})
	}
}

func TestTagCreateStoreError(t *testing.T) {
	f := &fakeTagStore{createErr: errors.New("firestore: deadline exceeded")}
	rec := doTag(t, f, &fakeCounter{}, "POST", "/tags", `{"slug":"rust","name":"Rust"}`)
	msg := assertError(t, rec, http.StatusInternalServerError, "internal")
	if strings.Contains(msg, "firestore") {
		t.Errorf("500 的訊息不能透露內部錯誤：%q", msg)
	}
}

// ── PUT /tags/{slug} ───────────────────────────────────────

func TestTagUpdateSuccess(t *testing.T) {
	f := &fakeTagStore{renameRet: &store.Tag{Slug: "go", Name: "Golang"}}
	c := &fakeCounter{counts: map[string]int{"go": 5}}
	rec := doTag(t, f, c, "PUT", "/tags/go", `{"name":"Golang"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("狀態碼 = %d，預期 200；body = %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q，預期 application/json", ct)
	}
	// slug 從路徑來，name 從 body 來
	if f.renameSlug != "go" || f.renameName != "Golang" {
		t.Errorf("傳給 store 的 slug/name = %q/%q，預期 go/Golang", f.renameSlug, f.renameName)
	}
	// 契約：回傳跟列表一樣的物件，含 count
	assertTag(t, decodeMap(t, rec), "go", "Golang", 5)
	if !slices.Equal(c.tags, []string{"go"}) || c.kinds[0] != "" {
		t.Errorf("CountPublished 參數 tag=%v kind=%v，預期 tag=[go]、kind 空字串", c.tags, c.kinds)
	}
}

func TestTagUpdateTrimsName(t *testing.T) {
	f := &fakeTagStore{}
	rec := doTag(t, f, &fakeCounter{}, "PUT", "/tags/go", `{"name":"  Go  "}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("狀態碼 = %d；body = %s", rec.Code, rec.Body.String())
	}
	if f.renameName != "Go" {
		t.Errorf("傳給 store 的 name = %q，預期去掉前後空白的 Go", f.renameName)
	}
}

func TestTagUpdateValidation(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"空物件", `{}`},
		{"name 空字串", `{"name":""}`},
		{"name 只有空白", `{"name":"   "}`},
		{"name 是 null", `{"name":null}`},
		{"name 型別錯", `{"name":1}`},
		{"送 slug（slug 不能改）", `{"name":"Go","slug":"golang"}`},
		{"送 count", `{"name":"Go","count":1}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, c := &fakeTagStore{}, &fakeCounter{}
			rec := doTag(t, f, c, "PUT", "/tags/go", tt.body)
			assertError(t, rec, http.StatusBadRequest, "invalid_request")
			assertTagStoreNotCalled(t, f)
		})
	}
}

// 路徑裡的 slug 格式不合法時，這個標籤不可能存在：回 404，不能是 500。
// 格式檢查在 TagStore 裡（Get 回 ErrNotFound，Rename 透過它也一樣），
// 見 tags_emulator_test.go 的 TestTagGetInvalidSlug、TestTagRenameInvalidSlug。
func TestTagUpdateInvalidPathSlugIs404(t *testing.T) {
	for _, slug := range []string{
		"Go",
		"__reserved__",
		"a_b",
		"node.js",
		"-x",
		strings.Repeat("a", 61),
	} {
		t.Run(slug, func(t *testing.T) {
			f := &fakeTagStore{}
			c := &fakeCounter{}
			rec := doTag(t, f, c, "PUT", "/tags/"+slug, `{"name":"x"}`)
			assertError(t, rec, http.StatusNotFound, "not_found")
			if len(c.tags) != 0 {
				t.Errorf("標籤不存在時不應該算 count，卻呼叫了 %v", c.tags)
			}
		})
	}
}

func TestTagUpdateNotFound(t *testing.T) {
	for _, tt := range []struct {
		name string
		err  error
	}{
		{"ErrNotFound", store.ErrNotFound},
		{"包了一層", errors.Join(errors.New("context"), store.ErrNotFound)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f, c := &fakeTagStore{renameErr: tt.err}, &fakeCounter{}
			rec := doTag(t, f, c, "PUT", "/tags/rust", `{"name":"Rust"}`)
			assertError(t, rec, http.StatusNotFound, "not_found")
			if len(c.tags) != 0 {
				t.Errorf("改名失敗時不應該算 count，卻呼叫了 %v", c.tags)
			}
		})
	}
}

func TestTagUpdateStoreError(t *testing.T) {
	f := &fakeTagStore{renameErr: errors.New("firestore: deadline exceeded")}
	rec := doTag(t, f, &fakeCounter{}, "PUT", "/tags/go", `{"name":"Go"}`)
	msg := assertError(t, rec, http.StatusInternalServerError, "internal")
	if strings.Contains(msg, "firestore") {
		t.Errorf("500 的訊息不能透露內部錯誤：%q", msg)
	}
}

func TestTagUpdateCountError(t *testing.T) {
	f := &fakeTagStore{renameRet: &store.Tag{Slug: "go", Name: "Go"}}
	c := &fakeCounter{err: errors.New("firestore: aggregation failed")}
	rec := doTag(t, f, c, "PUT", "/tags/go", `{"name":"Go"}`)
	msg := assertError(t, rec, http.StatusInternalServerError, "internal")
	if strings.Contains(msg, "firestore") {
		t.Errorf("500 的訊息不能透露內部錯誤：%q", msg)
	}
}
