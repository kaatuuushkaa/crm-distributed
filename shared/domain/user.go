package domain

import (
	"errors"
	"fmt"
	"github.com/google/uuid"
	"regexp"
	"time"
)

// провайдеры аутентификации
const (
	ProviderEmail = iota
	ProviderTest
)

var reColor = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

type User struct {
	UUID  uuid.UUID
	Name  string `validate:"lte=30"`
	Lname string `validate:"lte=30"`
	Pname string `validate:"lte=30"`
	Email string `validate:"email"`
	Phone int64

	Password string
	Provider int
	IsValid  bool

	Color    string
	HasPhoto bool
	Photo    *ProfilePhotoDTO

	Preferences ProfilePreferences

	ValidationSendAt *time.Time
	ValidAt          *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        *time.Time

	Meta string
}

type ProfilePhotoDTO struct {
	Small  string `json:"small"`
	Medium string `json:"medium"`
	Large  string `json:"large"`
}

type ProfilePreferences struct {
	Timezone *string `json:"timezone,omitempty"`
}

func NewUser(name, lname, pname, email string, phone int64, hashedPassword string) (*User, error) {
	if email == "" {
		return nil, errors.New("email обязателен")
	}

	if len(name) > 30 {
		return nil, errors.New("имя не может быть больше 30 символов")
	}

	if hashedPassword == "" {
		return nil, errors.New("пароль обязателен")
	}

	return &User{
		UUID:      uuid.New(),
		Name:      name,
		Lname:     lname,
		Pname:     pname,
		Email:     email,
		Phone:     phone,
		Password:  hashedPassword,
		Provider:  ProviderEmail,
		HasPhoto:  false,
		Color:     "#3B82F6",
		Meta:      "{}",
		CreatedAt: time.Now(),
	}, nil
}

func NewUserByUUID(uid uuid.UUID) *User {
	return &User{UUID: uid}
}

func (u *User) FullName() string {
	if u.Pname != "" {
		return fmt.Sprintf("%s %s %s", u.Lname, u.Name, u.Pname)
	}

	return fmt.Sprintf("%s %s", u.Lname, u.Name)
}

func (u *User) ChangePassword(newHashedPassword string) error {
	if newHashedPassword == "" {
		return errors.New("новый пароль не может быть пустым")
	}

	u.Password = newHashedPassword

	return nil
}

func (u *User) ChangeColor(color string) error {
	if !reColor.MatchString(color) {
		return fmt.Errorf("некорректный hex-цвет %q, ожидается формат #RRGGBB", color)
	}

	u.Color = color

	return nil
}

func (u *User) ChangeFIO(name, lname, pname *string) error {
	if name != nil {
		if *name == "" {
			return errors.New("имя не может быть пустым")
		}

		if len(*name) > 30 {
			return errors.New("имя не может быть больше 30 символов")
		}

		u.Name = *name
	}

	if lname != nil {
		if len(*lname) > 30 {
			return errors.New("фамилия не может быть больше 30 символов")
		}

		u.Lname = *lname
	}

	if pname != nil {
		if len(*pname) > 30 {
			return errors.New("отчество не может быть больше 30 символов")
		}

		u.Pname = *pname
	}

	return nil
}

func (u *User) ChangePhone(phone int64) error {
	if phone < 10_000_000_000 || phone > 9_999_999_999_999 {
		return errors.New("неверный формат телефона, ожидается 79999999999")
	}

	u.Phone = phone

	return nil
}

type SearchUser struct {
	FederationUUID uuid.UUID
	CompanyUUID    *uuid.UUID `json:"company_uuid,omitempty"` // nil — искать по всей федерации
	Search         string
}
