package store

import (
	"context"
	"strconv"
	"time"

	"cloud.google.com/go/storage"
)

type ImageStore struct {
	bucket *storage.BucketHandle
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
