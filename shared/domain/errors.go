package domain

import "errors"

var (
	ErrTaskNotFound             = errors.New("задача не найдена")
	ErrNotFound                 = errors.New("запись не найдена")
	ErrPermissionDenied         = errors.New("доступ запрещён")
	ErrAlreadyExists            = errors.New("запись уже существует")
	ErrLegalEntityAlreadyExists = errors.New("юридическое лицо с таким ИНН уже существует")
)
