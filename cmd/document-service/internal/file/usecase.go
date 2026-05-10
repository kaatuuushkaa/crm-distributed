package file

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"crm-distributed/shared/domain"
)

const (
	MaxFileSize          int64 = 50 * 1024 * 1024
	PresignedURLLifetime       = 15 * time.Minute
)

type UploadCommand struct {
	OwnerUUID    uuid.UUID
	OwnerType    domain.FileOwnerType
	OriginalName string
	ContentType  string
	SizeBytes    int64
	Reader       io.Reader
	CreatedBy    uuid.UUID
}

type Usecase struct {
	repo    *Repository
	storage *Storage
	log     *slog.Logger
}

func NewUsecase(repo *Repository, storage *Storage, log *slog.Logger) *Usecase {
	return &Usecase{repo: repo, storage: storage, log: log}
}

func (u *Usecase) Upload(ctx context.Context, cmd UploadCommand) (*domain.File, error) {
	if err := validateUpload(cmd); err != nil {
		return nil, fmt.Errorf("validate upload: %w", err)
	}

	fileID := uuid.New()
	storageKey := BuildKey(string(cmd.OwnerType), cmd.OwnerUUID, fileID, cmd.OriginalName)

	if err := u.storage.Upload(ctx, storageKey, cmd.Reader, cmd.SizeBytes, cmd.ContentType); err != nil {
		return nil, fmt.Errorf("upload to storage: %w", err)
	}

	f := &domain.File{
		UUID:         fileID,
		OwnerUUID:    cmd.OwnerUUID,
		OwnerType:    cmd.OwnerType,
		OriginalName: cmd.OriginalName,
		StorageKey:   storageKey,
		ContentType:  cmd.ContentType,
		SizeBytes:    cmd.SizeBytes,
		CreatedBy:    cmd.CreatedBy,
		CreatedAt:    time.Now(),
	}

	if err := u.repo.Create(ctx, f); err != nil {
		if delErr := u.storage.Delete(ctx, storageKey); delErr != nil {
			u.log.ErrorContext(ctx, "cleanup orphan failed",
				"storage_key", storageKey,
				"error", delErr,
			)
		}

		return nil, fmt.Errorf("create file metadata: %w", err)
	}

	u.log.InfoContext(ctx, "file uploaded",
		"file_uuid", f.UUID,
		"owner_uuid", f.OwnerUUID,
		"size_bytes", f.SizeBytes,
	)

	return f, nil
}

func (u *Usecase) Download(ctx context.Context, fileUUID uuid.UUID) (*domain.File, io.ReadCloser, error) {
	f, err := u.repo.Get(ctx, fileUUID)
	if err != nil {
		return nil, nil, fmt.Errorf("get file metadata: %w", err)
	}

	reader, _, err := u.storage.Download(ctx, f.StorageKey)
	if err != nil {
		return nil, nil, fmt.Errorf("download from storage: %w", err)
	}

	return f, reader, nil
}

func (u *Usecase) PresignedURL(ctx context.Context, fileUUID uuid.UUID) (string, *domain.File, error) {
	f, err := u.repo.Get(ctx, fileUUID)
	if err != nil {
		return "", nil, fmt.Errorf("get file: %w", err)
	}

	url, err := u.storage.PresignedURL(ctx, f.StorageKey, PresignedURLLifetime)
	if err != nil {
		return "", nil, fmt.Errorf("presigned url: %w", err)
	}

	return url, f, nil
}

func (u *Usecase) ListByOwner(ctx context.Context, ownerUUID uuid.UUID, ownerType domain.FileOwnerType) ([]domain.File, error) {
	return u.repo.ListByOwner(ctx, ownerUUID, ownerType)
}

func (u *Usecase) Delete(ctx context.Context, fileUUID uuid.UUID) error {
	f, err := u.repo.Get(ctx, fileUUID)
	if err != nil {
		return fmt.Errorf("get file: %w", err)
	}

	if err := u.storage.Delete(ctx, f.StorageKey); err != nil {
		u.log.WarnContext(ctx, "delete from storage failed",
			"storage_key", f.StorageKey,
			"error", err,
		)
	}

	if err := u.repo.SoftDelete(ctx, fileUUID); err != nil {
		return fmt.Errorf("soft delete metadata: %w", err)
	}

	u.log.InfoContext(ctx, "file deleted", "file_uuid", fileUUID)

	return nil
}

func validateUpload(cmd UploadCommand) error {
	if cmd.OwnerUUID == uuid.Nil {
		return errors.New("owner uuid is required")
	}

	if cmd.OwnerType == "" {
		return errors.New("owner type is required")
	}

	if cmd.OriginalName == "" {
		return errors.New("original name is required")
	}

	if cmd.SizeBytes <= 0 {
		return errors.New("size must be positive")
	}

	if cmd.SizeBytes > MaxFileSize {
		return fmt.Errorf("file too large: %d bytes (max %d)", cmd.SizeBytes, MaxFileSize)
	}

	if cmd.Reader == nil {
		return errors.New("reader is required")
	}

	return nil
}
