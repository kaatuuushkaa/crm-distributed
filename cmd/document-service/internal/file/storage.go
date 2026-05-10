package file

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	mn "crm-distributed/shared/pkg/minio"
)

type Storage struct {
	mc *mn.Client
}

func NewStorage(mc *mn.Client) *Storage {
	return &Storage{mc: mc}
}

func BuildKey(ownerType string, ownerUUID uuid.UUID, fileID uuid.UUID, originalName string) string {
	ext := filepath.Ext(originalName)
	year := time.Now().Year()

	return fmt.Sprintf("%s/%s/%d/%s%s",
		strings.ToLower(ownerType),
		ownerUUID.String(),
		year,
		fileID.String(),
		ext,
	)
}

func (s *Storage) Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	return s.mc.Put(ctx, key, reader, size, contentType)
}

func (s *Storage) Download(ctx context.Context, key string) (io.ReadCloser, int64, error) {
	return s.mc.Get(ctx, key)
}

func (s *Storage) Delete(ctx context.Context, key string) error {
	return s.mc.Delete(ctx, key)
}

func (s *Storage) PresignedURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	return s.mc.PresignedURL(ctx, key, expires)
}
