package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"cindle.dev/blog/internal/httpx"
	"cindle.dev/blog/internal/store"
)

type createTagRequest struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type tagDetail struct {
	Slug  string `json:"slug"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type updateTagRequest struct {
	Name string `json:"name"`
}

type tagsStore interface {
	List(ctx context.Context) ([]store.Tag, error)
	Create(ctx context.Context, slug, name string) (*store.Tag, error)
	Rename(ctx context.Context, slug, name string) (*store.Tag, error)
}

type tagCounter interface {
	CountPublished(ctx context.Context, kind, tag string) (int, error)
}

type TagHandler struct {
	repo    tagsStore
	counter tagCounter
}

func NewTagHandler(repo tagsStore, counter tagCounter) *TagHandler {
	return &TagHandler{
		repo:    repo,
		counter: counter,
	}
}

func (t *TagHandler) List(w http.ResponseWriter, r *http.Request) {
	tags, err := t.repo.List(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "查詢標籤失敗，請查看詳細Log")
		httpx.WriteErrLog("伺服器回應失敗", "List", "[tags_handler] errMsg", err)
		return
	}

	items := make([]tagDetail, 0, len(tags))
	for _, tag := range tags {
		// kind "" 同等 all
		n, err := t.counter.CountPublished(r.Context(), "", tag.Slug)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "查詢標籤失敗，請查看詳細Log")
			httpx.WriteErrLog("伺服器回應失敗", "List", "[tags_handler] errMsg", err)
			return
		}

		items = append(items, tagDetail{Slug: tag.Slug, Name: tag.Name, Count: n})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	resp := map[string][]tagDetail{
		"items": items,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		httpx.WriteErrLog("伺服器回應失敗", "List", "[tags_handler] errMsg", err)
		return
	}
}

func (t *TagHandler) Create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1048576)

	var req createTagRequest
	if err := handleDecodeBody(r.Body, &req); err != nil {
		handleError(err, w)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Slug = strings.TrimSpace(req.Slug)

	if len(req.Name) == 0 {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Name 必須填寫")
		return
	}

	if !store.ValidSlug(req.Slug) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Slug 格式有誤")
		return
	}

	tag, err := t.repo.Create(r.Context(), req.Slug, req.Name)
	if err != nil {
		if errors.Is(err, store.ErrSlugConflict) {
			httpx.WriteError(w, http.StatusConflict, "slug_conflict", fmt.Sprintf("Tag「%s」已經被建立", req.Slug))
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "建立標籤失敗，請查看詳細Log")
		httpx.WriteErrLog("伺服器回應失敗", "Create", "[tags_handler] errMsg", err)
		return
	}

	resp := tagDetail{
		Slug:  tag.Slug,
		Name:  tag.Name,
		Count: 0,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		httpx.WriteErrLog("伺服器回應失敗", "Create", "[tags_handler] errMsg", err)
		return
	}
}

func (t *TagHandler) Update(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1048576)

	slug := r.PathValue("slug")

	var req updateTagRequest
	if err := handleDecodeBody(r.Body, &req); err != nil {
		handleError(err, w)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if len(req.Name) == 0 {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Name 必須填寫")
		return
	}

	tag, err := t.repo.Rename(r.Context(), slug, req.Name)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "查無此標籤")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "更新標籤失敗詳情請查看 Log")
		httpx.WriteErrLog("伺服器回應失敗", "Update", "[tags_handler] errMsg", err)
		return
	}

	count, err := t.counter.CountPublished(r.Context(), "", tag.Slug)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "查詢 count 標籤數失敗，詳情請看 Log")
		httpx.WriteErrLog("伺服器回應失敗", "Update", "[tags_handler] errMsg", err)
		return
	}

	resp := tagDetail{
		Slug:  tag.Slug,
		Name:  tag.Name,
		Count: count,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		httpx.WriteErrLog("伺服器回應失敗", "Update", "[tags_handler] errMsg", err)
		return
	}
}
