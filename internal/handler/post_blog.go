package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
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

type postResponse struct {
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

type PostBlogApi interface {
	CreatePost(w http.ResponseWriter, r *http.Request)
}

type PostBlogController struct {
	repo *store.PostBlogRepo
}

func NewPostBlogController(repo *store.PostBlogRepo) *PostBlogController {
	return &PostBlogController{repo: repo}
}

func (p *PostBlogController) CreatePost(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1048576)

	var req createPostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		handleError(err, w)
		return
	}

	if len(strings.TrimSpace(req.Title)) == 0 {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Title 必須填寫")
		return
	}

	if len(req.Kind) != 0 && req.Kind != "post" && req.Kind != "note" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Kind 型別必須為 post 或是 note")
		return
	}

	post := store.Post{
		Title:    req.Title,
		Kind:     req.Kind,
		Markdown: req.Markdown,
		Slug:     req.Slug,
		TagSlugs: req.TagSlugs,
		Summary:  req.Summary,
		Cover:    req.Cover,
		Pinned:   req.Pinned,
	}

	cp, err := p.repo.CreatePost(r.Context(), post)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "建立文章失敗，請查看詳細Log")
		slog.Error("伺服器回應失敗", "[post_blog] errMsg", err.Error())
		return
	}

	// 轉換 response
	pr := toPostResponse(cp)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(pr); err != nil {
		slog.Error("伺服器回應失敗", "[post_blog] errMsg", err.Error())
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
	case errors.As(err, &maxBytesErr):
		httpx.WriteError(w, http.StatusRequestEntityTooLarge, "invalid_request", "資料傳遞過大")
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "資料其他錯誤，請查看Log")
		slog.Error("接收請求失敗", "[post_blog] errMsg", err.Error())
	}
}

func toPostResponse(p *store.Post) *postResponse {
	pr := postResponse{
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
		p.TagSlugs = make([]string, 0)
	}

	return &pr
}
