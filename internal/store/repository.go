package store

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const Collections = "posts"

type Post struct {
	// firestore:"-" 表示不寫進文件。document ID 存在 snapshot.Ref.ID，
	// 不是欄位，所以要在讀取後自己補上。
	ID string `firestore:"-"`

	Slug   string `firestore:"slug"`
	Kind   string `firestore:"kind"`   // "post" | "note"
	Status string `firestore:"status"` // "draft" | "published"

	Title    string `firestore:"title"`
	Summary  string `firestore:"summary"`
	Markdown string `firestore:"markdown"`
	HTML     string `firestore:"html"`

	TOC        []TOCItem `firestore:"toc"`
	ReadingMin int       `firestore:"readingMin"`
	HasCode    bool      `firestore:"hasCode"`
	TagSlugs   []string  `firestore:"tagSlugs"`

	Cover  *Image `firestore:"cover"` // 指標，沒封面就是 nil
	Pinned bool   `firestore:"pinned"`

	// 指標型別才能表達「草稿還沒有發佈時間」。用 time.Time 的話
	// 零值會被寫成 0001-01-01，查詢排序時混在一起很難處理。
	PublishedAt *time.Time `firestore:"publishedAt"`

	CreatedAt time.Time `firestore:"createdAt"`

	// serverTimestamp：寫入時如果這個欄位是零值，就改用伺服器的時間。
	// 好處是不依賴本機時鐘，而且多台實例寫入時間軸一致。
	// 注意它只在 Set / Add 生效；Update 要顯式傳 firestore.ServerTimestamp。
	UpdatedAt time.Time `firestore:"updatedAt,serverTimestamp"`
}

type TOCItem struct {
	Level  int    `firestore:"level"`
	Text   string `firestore:"text"`
	Anchor string `firestore:"anchor"`
}

type Image struct {
	URL    string `firestore:"url"`
	Alt    string `firestore:"alt"`
	Width  int    `firestore:"width"`
	Height int    `firestore:"height"`
}

type PostsRepository interface {
	GetPublishedBySlug(ctx context.Context, slug string) (*Post, error)
	ListPosts(ctx context.Context, kind, tag, search, status string, page, perPage int) ([]Post, int, error)
	ListPublished(ctx context.Context, kind, tag string, page, perPage int) ([]Post, error)
	CountPublished(ctx context.Context, kind, tag string) (int, error)
	CreatePost(ctx context.Context, post Post) (*Post, error)
	GetPost(ctx context.Context, id string) (*Post, error)
}

type PostBlogRepo struct {
	client *firestore.Client
}

func NewPostBlog(client *firestore.Client) *PostBlogRepo {
	return &PostBlogRepo{client: client}
}

func (p *PostBlogRepo) CreatePost(ctx context.Context, post Post) (*Post, error) {
	ref := p.client.Collection(Collections).NewDoc()

	post.ID = ref.ID
	post.Status = "draft"
	post.PublishedAt = nil
	post.CreatedAt = time.Now()

	if len(post.Kind) == 0 {
		post.Kind = "post"

	}
	if len(post.Slug) == 0 {
		post.Slug = makeSlug(post.Title, post.ID)
	}

	wr, err := ref.Create(ctx, &post)
	if err != nil {
		return nil, err
	}

	post.UpdatedAt = wr.UpdateTime

	return &post, nil
}

func (p *PostBlogRepo) GetPost(ctx context.Context, id string) (*Post, error) {
	if len(id) == 0 {
		return nil, fmt.Errorf("id is empty")
	}

	snap, err := p.client.Collection(Collections).Doc(id).Get(ctx)
	switch status.Code(err) {
	case codes.OK:
	case codes.NotFound, codes.InvalidArgument:
		return nil, ErrNotFound
	default:
		return nil, err
	}

	var post Post
	if err := snap.DataTo(&post); err != nil {
		return nil, err
	}
	post.ID = snap.Ref.ID
	return &post, nil
}

func (p *PostBlogRepo) GetPublishedBySlug(ctx context.Context, slug string) (*Post, error) {
	iter := p.client.Collection(Collections).
		Where("slug", "==", slug).
		Where("status", "==", "published").
		Limit(1).
		Documents(ctx)

	// 忘了會漏連線
	defer iter.Stop()

	snap, err := iter.Next()
	if errors.Is(err, iterator.Done) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}

	var post Post
	if err := snap.DataTo(&post); err != nil {
		return nil, err
	}

	post.ID = snap.Ref.ID
	return &post, nil
}

// ListPosts 回傳後台列表的一頁文章，以及符合條件的總篇數。
//
// 分頁刻意在 Go 裡做，不用 Firestore 的 Offset / Limit，原因有兩個：
//
//  1. search 要比對標題的任何位置、而且不分大小寫，Firestore 沒有這種查詢，
//     只能把文章讀回來在 Go 裡比對。搜尋又必須在切頁之前做，total 才會是
//     「搜尋後還剩幾篇」，所以切頁也只能在 Go 裡做。
//  2. Offset / Limit 要先排序才有意義；沒有 OrderBy 時，Firestore 是照
//     document ID 的順序切，切出來的不是最新的那幾篇。而照 updatedAt 排序
//     再加上 status、kind 過濾，每一種組合都需要一個複合索引。
//
// 第 2 點可以靠補索引解決，第 1 點不行：只要有搜尋，就只能在 Go 分頁。
//
// 所以 Firestore 只做 status、kind 的等值過濾（不需要複合索引），其餘依序
// 在 Go 裡完成：search 過濾 → 照 updatedAt 新到舊排序 → 取 total → 切出這一頁。
//
// 代價是每次都會讀出所有符合 status、kind 的文章。後台只有一個使用者、文章
// 在數百篇以內，遠低於每天 5 萬次的免費讀取額度；文章到數千篇時再考慮改用
// 搜尋服務。
//
// 公開頁面用的 ListPublished 沒有搜尋、只照 publishedAt 排序，Terraform 也
// 建好了對應的索引，所以那邊仍然在 Firestore 分頁。
func (p *PostBlogRepo) ListPosts(ctx context.Context, kind, tag, search, status string, page, perPage int) ([]Post, int, error) {
	q := p.baseQuery(status, kind, tag)

	snaps, err := q.Documents(ctx).GetAll()

	if err != nil {
		return nil, 0, err
	}

	var posts []Post

	keyword := strings.ToLower(search)
	for _, snap := range snaps {
		var post Post
		if err := snap.DataTo(&post); err != nil {
			return nil, 0, err
		}
		if len(keyword) != 0 && !strings.Contains(strings.ToLower(post.Title), keyword) {
			continue
		}
		post.ID = snap.Ref.ID
		posts = append(posts, post)
	}

	slices.SortFunc(posts, func(a, b Post) int {
		if c := b.UpdatedAt.Compare(a.UpdatedAt); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})

	total := len(posts)

	start := (page - 1) * perPage
	if start >= total {
		return []Post{}, total, nil
	}
	end := min(start+perPage, len(posts))

	return posts[start:end], total, nil
}

func (p *PostBlogRepo) ListPublished(ctx context.Context, kind, tag string, page, perPage int) ([]Post, error) {
	snaps, err := p.publishedQuery(kind, tag).
		OrderBy("publishedAt", firestore.Desc).
		OrderBy(firestore.DocumentID, firestore.Desc).
		Offset((page - 1) * perPage).
		Limit(perPage).
		Documents(ctx).GetAll()

	var posts []Post
	for _, snap := range snaps {
		var post Post
		if err := snap.DataTo(&post); err != nil {
			return nil, err
		}
		post.ID = snap.Ref.ID
		posts = append(posts, post)
	}

	return posts, err
}

func (p *PostBlogRepo) CountPublished(ctx context.Context, kind, tag string) (int, error) {
	q := p.publishedQuery(kind, tag)
	res, err := q.NewAggregationQuery().WithCount("n").Get(ctx)
	if err != nil {
		return 0, err
	}

	n, ok := res.Data()["n"].(int64)
	if !ok {
		return 0, fmt.Errorf("count 結果型別不正確: %T", res.Data()["n"])
	}

	return int(n), nil
}

func (p *PostBlogRepo) publishedQuery(kind, tag string) firestore.Query {
	q := p.baseQuery("published", kind, tag)
	return q
}

func (p *PostBlogRepo) baseQuery(status, kind, tag string) firestore.Query {
	q := p.client.Collection(Collections).Query

	if len(status) != 0 {
		q = q.Where("status", "==", status)
	}

	if len(kind) != 0 {
		q = q.Where("kind", "==", kind)
	}

	if len(tag) != 0 {
		q = q.Where("tagSlugs", "array-contains", tag)
	}

	return q
}

const maxSlugLen = 60

// makeSlug 由標題產生 URL 用的 slug；標題裡沒有任何英數字元時，
// 退回 "post-" + id 前 8 碼。
func makeSlug(title, id string) string {
	var b strings.Builder
	lastHyphen := true // 一開始就當作剛寫過連字號，這樣開頭的符號不會產生連字號

	for _, r := range strings.ToLower(title) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastHyphen = false
		} else if !lastHyphen {
			b.WriteByte('-')
			lastHyphen = true
		}
	}

	slug := strings.TrimRight(b.String(), "-")
	if len(slug) > maxSlugLen {
		slug = strings.TrimRight(slug[:maxSlugLen], "-")
	}
	if slug == "" {
		return "post-" + id[:8]
	}
	return slug
}
