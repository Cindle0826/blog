package store

import (
	"context"
	"errors"
	"io"
	"strconv"
	"time"

	"cloud.google.com/go/storage"
)

type ImageStore struct {
	bucket *storage.BucketHandle
}

type ImageInfo struct {
	ContentType string
	Size        int64
	ETag        string
}

func NewImageStore(client *storage.Client, bucketName string) *ImageStore {
	return &ImageStore{
		bucket: client.Bucket(bucketName),
	}
}

func (s *ImageStore) Put(ctx context.Context, object, contentType string, data []byte, width, height int) error {
	// 設置上傳 timeout 時間
	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	// 設置上傳配置, 圖片以存在就失敗
	w := s.bucket.Object(object).If(storage.Conditions{DoesNotExist: true}).NewWriter(ctx)

	// 設置 Metadata
	w.ContentType = contentType
	w.Metadata = map[string]string{
		"width":  strconv.Itoa(width),
		"height": strconv.Itoa(height),
	}

	// 寫入圖片內容
	if _, err := w.Write(data); err != nil {
		_ = w.Close()
		return err
	}

	return w.Close()
}

func (s *ImageStore) Stat(ctx context.Context, object string) (ImageInfo, error) {

	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	attrs, err := s.bucket.Object(object).Attrs(ctx)

	if err != nil {
		var errInfo ImageInfo
		if errors.Is(err, storage.ErrObjectNotExist) {
			return errInfo, ErrNotFound
		}
		return errInfo, err
	}

	result := ImageInfo{
		ContentType: attrs.ContentType,
		Size:        attrs.Size,
		ETag:        attrs.Etag,
	}
	return result, nil
}

func (s *ImageStore) Open(ctx context.Context, object string) (io.ReadCloser, error) {

	reader, err := s.bucket.Object(object).NewReader(ctx)

	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return reader, nil
}
