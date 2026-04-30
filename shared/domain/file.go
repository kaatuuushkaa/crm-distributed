package domain

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

type FileOwnerType string

const (
	FileOwnerTask        FileOwnerType = "task"
	FileOwnerCompany     FileOwnerType = "company"
	FileOwnerLegalEntity FileOwnerType = "legal_entity"
)

var (
	ErrFileNotFound = errors.New("файл не найден")
	ErrFileInvalid  = errors.New("некорректные данные файла")
)

type File struct {
	UUID uuid.UUID

	OwnerUUID uuid.UUID
	OwnerType FileOwnerType

	OriginalName string

	StorageKey string

	ContentType string

	SizeBytes int64

	CreatedBy uuid.UUID
	CreatedAt time.Time
	DeletedAt *time.Time
}

func NewFile(
	ownerUUID uuid.UUID,
	ownerType FileOwnerType,
	originalName, contentType string,
	sizeBytes int64,
	createdBy uuid.UUID,
) (*File, error) {
	if ownerUUID == uuid.Nil {
		return nil, fmt.Errorf("%w: owner UUID обязателен", ErrFileInvalid)
	}

	if originalName == "" {
		return nil, fmt.Errorf("%w: имя файла обязательно", ErrFileInvalid)
	}

	if sizeBytes <= 0 {
		return nil, fmt.Errorf("%w: размер файла должен быть положительным", ErrFileInvalid)
	}

	uid := uuid.New()
	ext := filepath.Ext(originalName)
	year := time.Now().Format("2006") // YYYY

	storageKey := fmt.Sprintf("%s/%s/%s/%s%s",
		ownerType,
		ownerUUID.String(),
		year,
		uid.String(),
		ext,
	)

	return &File{
		UUID:         uid,
		OwnerUUID:    ownerUUID,
		OwnerType:    ownerType,
		OriginalName: originalName,
		StorageKey:   storageKey,
		ContentType:  contentType,
		SizeBytes:    sizeBytes,
		CreatedBy:    createdBy,
		CreatedAt:    time.Now(),
	}, nil
}

func (f *File) Extension() string {
	return strings.TrimPrefix(filepath.Ext(f.OriginalName), ".")
}

func (f *File) IsImage() bool {
	switch f.ContentType {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
		return true
	}

	return false
}
