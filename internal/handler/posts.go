package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"cindle.dev/blog/internal/httpx"
	"cindle.dev/blog/internal/store"
)

type createPostRequest struct {
	Title    string       `json:"title"`
	Kind     string       `json:"kind"`
	Markdown string       `json:"markdown"`
	Slug     string       `json:"slug"`
	TagSlugs []string     `json:"tagSlugs"`
	Summary  string       `json:"summary"`
	Cover    *store.Image `json:"cover"`
	Pinned   bool         `json:"pinned"`
}

type postDetail struct {
	ID          string       `json:"id"`
	Slug        string       `json:"slug"`
	Kind        string       `json:"kind"`
	Status      string       `json:"status"`
	Title       string       `json:"title"`
	Summary     string       `json:"summary"`
	Markdown    string       `json:"markdown"`
	TagSlugs    []string     `json:"tagSlugs"`
	Cover       *store.Image `json:"cover"`
	Pinned      bool         `json:"pinned"`
	PublishedAt *time.Time   `json:"publishedAt"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}

type postSummary struct {
	ID          string     `json:"id"`
	Slug        string     `json:"slug"`
	Kind        string     `json:"kind"`
	Status      string     `json:"status"`
	Title       string     `json:"title"`
	Summary     string     `json:"summary"`
	TagSlugs    []string   `json:"tagSlugs"`
	Pinned      bool       `json:"pinned"`
	PublishedAt *time.Time `json:"publishedAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

type postPage struct {
	Items      []postSummary `json:"items"`
	Page       int           `json:"page"`
	TotalPages int           `json:"totalPages"`
	Total      int           `json:"total"`
}

type postPublish struct {
	Publish *bool `json:"publish"`
}

type updatePostRequest struct {
	Title    *string         `json:"title"`
	Kind     *string         `json:"kind"`
	Markdown *string         `json:"markdown"`
	Slug     *string         `json:"slug"`
	TagSlugs *[]string       `json:"tagSlugs"`
	Summary  *string         `json:"summary"`
	Cover    json.RawMessage `json:"cover"`
	Pinned   *bool           `json:"pinned"`
}

type postStore interface {
	List(ctx context.Context, kind, tag, search, status string, page, perPage int) ([]store.Post, int, error)
	Create(ctx context.Context, post store.Post) (*store.Post, error)
	Get(ctx context.Context, id string) (*store.Post, error)
	Delete(ctx context.Context, id string) error
	SetPublished(ctx context.Context, id string, publish bool) (*store.Post, error)
	Update(ctx context.Context, id string, patch store.PostPatch) (*store.Post, error)
}

type PostHandler struct {
	repo postStore
}

func NewPostHandler(repo postStore) *PostHandler {
	return &PostHandler{repo: repo}
}

func (p *PostHandler) Create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1048576)

	var req createPostRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		handleError(err, w)
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	req.Slug = strings.TrimSpace(req.Slug)

	if len(req.Title) == 0 {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Title 必須填寫")
		return
	}

	if len(req.Kind) != 0 && req.Kind != "post" && req.Kind != "note" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Kind 型別必須為 post 或是 note")
		return
	}

	if len(req.Slug) != 0 && !store.ValidSlug(req.Slug) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "slug 只能用小寫英數與連字號，不能超過 60 字")
		return
	}

	if req.Cover != nil {
		if err := validateCover(*req.Cover); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
	}

	post := req.toCreate()

	cp, err := p.repo.Create(r.Context(), post)
	if err != nil {
		if errors.Is(err, store.ErrSlugConflict) {
			httpx.WriteError(w, http.StatusConflict, "slug_conflict", fmt.Sprintf("網址「%s」已經被其他文章使用", req.Slug))
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "建立文章失敗，請查看詳細Log")
		httpx.WriteErrLog("伺服器回應失敗", "Create", "[posts_handler] errMsg", err)
		return
	}

	// 轉換 response
	pr := toPostDetail(cp)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(pr); err != nil {
		httpx.WriteErrLog("伺服器回應失敗", "Create", "[posts_handler] errMsg", err)
		return
	}
}

func (p *PostHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if len(id) == 0 {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "id 必須填寫")
		return
	}

	gp, err := p.repo.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "查無此文章")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "獲取文章失敗詳情請查看 Log")
		httpx.WriteErrLog("伺服器回應失敗", "Get", "[posts_handler] errMsg", err)
		return
	}

	// 轉換 response
	pr := toPostDetail(gp)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(pr); err != nil {
		httpx.WriteErrLog("伺服器回應失敗", "Get", "[posts_handler] errMsg", err)
		return
	}
}

func (p *PostHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	status := q.Get("status")
	status = strings.TrimSpace(status)

	if len(status) != 0 && status != "draft" && status != "published" && status != "all" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "status 狀態不存在")
		return
	}

	if status == "all" {
		status = ""
	}

	kind := q.Get("kind")
	kind = strings.TrimSpace(kind)
	if len(kind) != 0 && kind != "post" && kind != "note" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "kind 狀態不存在")
		return
	}

	page, ok := parsePositive(q.Get("page"), 1)
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "page 參數有誤")
		return
	}

	limit, ok := parsePositive(q.Get("limit"), 50)
	limit = min(limit, 100)
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "limit 參數有誤")
		return
	}

	search := q.Get("search")
	search = strings.TrimSpace(search)

	posts, total, err := p.repo.List(r.Context(), kind, "", search, status, page, limit)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "查詢文章失敗，請查看詳細Log")
		httpx.WriteErrLog("伺服器回應失敗", "List", "[posts_handler] errMsg", err)
		return
	}

	var ps = make([]postSummary, 0)

	for _, post := range posts {
		pr := toPostSummary(&post)
		ps = append(ps, *pr)
	}

	var rs = postPage{
		Items:      ps,
		Page:       page,
		TotalPages: (total + limit - 1) / limit,
		Total:      total,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(rs); err != nil {
		httpx.WriteErrLog("伺服器回應失敗", "List", "[posts_handler] errMsg", err)
		return
	}
}

func (p *PostHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if len(id) == 0 {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "id 必須填寫")
		return
	}

	err := p.repo.Delete(r.Context(), id)

	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "查無此文章，刪除失敗")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "刪除文章失敗詳情請查看 Log")
		httpx.WriteErrLog("伺服器回應失敗", "Delete", "[posts_handler] errMsg", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (p *PostHandler) SetPublished(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1048576)

	id := r.PathValue("id")

	if len(id) == 0 {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "id 必須填寫")
		return
	}

	var pp postPublish

	if err := json.NewDecoder(r.Body).Decode(&pp); err != nil {
		handleError(err, w)
		return
	}

	if pp.Publish == nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "publish 必須填寫")
		return
	}

	post, err := p.repo.SetPublished(r.Context(), id, *pp.Publish)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "查無此資料")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "查詢文章失敗詳情請查看 Log")
		httpx.WriteErrLog("伺服器回應失敗", "SetPublished", "[posts_handler] errMsg", err)
		return
	}

	rp := toPostDetail(post)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(rp); err != nil {
		httpx.WriteErrLog("伺服器回應失敗", "SetPublished", "[posts_handler] errMsg", err)
		return
	}
}

func (p *PostHandler) Update(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1048576)

	id := r.PathValue("id")

	if len(id) == 0 {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "id 必須填寫")
		return
	}

	var req updatePostRequest

	decoder := json.NewDecoder(r.Body)
	// 嚴格判斷參數
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		handleError(err, w)
		return
	}

	if req.Title != nil && len(strings.TrimSpace(*req.Title)) == 0 {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Title 必須填寫")
		return
	}

	if req.Kind != nil && *req.Kind != "post" && *req.Kind != "note" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Kind 型別必須為 post 或是 note")
		return
	}
	if req.Slug != nil && !store.ValidSlug(strings.TrimSpace(*req.Slug)) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "slug 只能用小寫英數與連字號，不能超過 60 字")
		return
	}

	patch, err := req.toPatch()
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Cover 解析錯誤")
		return
	}

	if patch == (store.PostPatch{}) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "沒有要更新的欄位")
		return
	}

	up, err := p.repo.Update(r.Context(), id, patch)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			httpx.WriteError(w, http.StatusNotFound, "not_found", "查無此文章")
			return
		case errors.Is(err, store.ErrSlugLocked):
			httpx.WriteError(w, http.StatusConflict, "slug_locked", "發布過的文章不能修改網址")
			return
		case errors.Is(err, store.ErrSlugConflict):
			httpx.WriteError(w, http.StatusConflict, "slug_conflict", fmt.Sprintf("網址 「%s」已經被其他文章使用", *req.Slug))
			return
		default:
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "更新文章失敗詳情請查看 Log")
			httpx.WriteErrLog("伺服器回應失敗", "Update", "[posts_handler] errMsg", err)
			return
		}
	}
	// 轉換 response
	pr := toPostDetail(up)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(pr); err != nil {
		httpx.WriteErrLog("伺服器回應失敗", "Update", "[posts_handler] errMsg", err)
		return
	}
}

func handleError(err error, w http.ResponseWriter) {
	var (
		syntaxErr        *json.SyntaxError
		unmarshalTypeErr *json.UnmarshalTypeError
		maxBytesErr      *http.MaxBytesError
	)

	switch {
	case errors.As(err, &syntaxErr):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "資料傳遞有誤")
	case errors.As(err, &unmarshalTypeErr):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "資料解析有誤")
	case errors.Is(err, io.EOF):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "資料傳遞為空")
	case errors.Is(err, io.ErrUnexpectedEOF):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "資料不完整")
	case strings.HasPrefix(err.Error(), "json: unknown field"):
		fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", fmt.Sprintf("%s 為不合法參數", fieldName))
	case errors.As(err, &maxBytesErr):
		httpx.WriteError(w, http.StatusRequestEntityTooLarge, "invalid_request", "資料傳遞過大")
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "資料其他錯誤，請查看Log")
		httpx.WriteErrLog("接收請求失敗", "handleError", "[posts_handler] errMsg", err)

	}
}

func toPostDetail(p *store.Post) *postDetail {
	pr := postDetail{
		ID:          p.ID,
		Slug:        p.Slug,
		Kind:        p.Kind,
		Status:      p.Status,
		Title:       p.Title,
		Summary:     p.Summary,
		Markdown:    p.Markdown,
		Cover:       p.Cover,
		Pinned:      p.Pinned,
		PublishedAt: p.PublishedAt,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}

	if p.TagSlugs == nil {
		pr.TagSlugs = make([]string, 0)
	} else {
		pr.TagSlugs = p.TagSlugs
	}

	return &pr
}

func toPostSummary(p *store.Post) *postSummary {
	gi := postSummary{
		ID:          p.ID,
		Slug:        p.Slug,
		Kind:        p.Kind,
		Status:      p.Status,
		Title:       p.Title,
		Summary:     p.Summary,
		Pinned:      p.Pinned,
		PublishedAt: p.PublishedAt,
		UpdatedAt:   p.UpdatedAt,
	}

	if p.TagSlugs == nil {
		gi.TagSlugs = make([]string, 0)
	} else {
		gi.TagSlugs = p.TagSlugs
	}

	return &gi
}

func parsePositive(s string, def int) (int, bool) {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return def, true
	}

	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 0, false
	}

	return n, true
}

func (r updatePostRequest) toPatch() (store.PostPatch, error) {
	patch := store.PostPatch{
		Kind:     r.Kind,
		Summary:  r.Summary,
		Markdown: r.Markdown,
		TagSlugs: r.TagSlugs,
		Pinned:   r.Pinned,
	}
	switch {
	case r.Cover == nil:
		// 沒送
	case string(r.Cover) == "null":
		patch.RemoveCover = true
	default:
		var img store.Image
		if err := json.Unmarshal(r.Cover, &img); err != nil {
			return store.PostPatch{}, err
		}

		if err := validateCover(img); err != nil {
			return store.PostPatch{}, err
		}

		patch.Cover = &img
	}

	if r.Title != nil {
		t := strings.TrimSpace(*r.Title)
		patch.Title = &t
	}

	if r.Slug != nil {
		s := strings.TrimSpace(*r.Slug)
		patch.Slug = &s
	}

	return patch, nil
}

func (c createPostRequest) toCreate() store.Post {
	post := store.Post{
		Title:    c.Title,
		Kind:     c.Kind,
		Markdown: c.Markdown,
		Slug:     c.Slug,
		Summary:  c.Summary,
		Cover:    c.Cover,
		Pinned:   c.Pinned,
	}

	if c.TagSlugs == nil {
		post.TagSlugs = make([]string, 0)
	} else {
		post.TagSlugs = c.TagSlugs
	}

	return post
}

func validateCover(img store.Image) error {
	if !strings.HasPrefix(img.URL, "/images/") {
		return fmt.Errorf("cover URL 必須以 /images/ 開頭")
	}

	if img.Height < 1 {
		return fmt.Errorf("cover Height 不能小於 1")
	}

	if img.Width < 1 {
		return fmt.Errorf("cover Width 不能小於 1")
	}
	return nil
}
