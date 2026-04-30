package domain

import (
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"strings"
	"time"
)

const (
	StatusUnknown    = 0
	StatusNew        = 1
	StatusInWork     = 2
	StatusHold       = 3
	StatusNeedReview = 4
	StatusDone       = 5
	StatusCancel     = 6
)

func StatusNames() map[int]string {
	return map[int]string{
		StatusUnknown:    "Необработана",
		StatusNew:        "Новая",
		StatusInWork:     "В работе",
		StatusHold:       "Приостановлена",
		StatusNeedReview: "На проверке",
		StatusDone:       "Завершена",
		StatusCancel:     "Отменена",
	}
}

type Task struct {
	UUID           uuid.UUID
	ID             int
	Name           string    `validate:"lte=100,gte=3"`
	Description    string    `validate:"lte=500"`
	FederationUUID uuid.UUID `validate:"required"`
	CompanyUUID    uuid.UUID `validate:"required"`
	ProjectUUID    uuid.UUID `validate:"required"`

	//задача является эпиком(контейнером для подзадач)
	IsEpic bool

	//роли на задаче
	CreatedBy     string
	ResponsibleBy string
	ImplementBy   string
	ManagedBy     string
	FinishedBy    string

	//соисполнитель и наблюдатель
	CoWorkersBy []string
	WatchBy     []string

	People   []string
	Tags     []string
	Status   int
	Priority int
	Icon     string

	//иерархмческий путь для поддержки подзадач
	Path []string

	//дедлайн
	FinishTo   *time.Time
	FinishedAt *time.Time

	//для постройки изменений activity
	Dirty map[string]any

	Fields    map[string]any
	RawFields map[string]any
	Meta      map[string]any

	ChildrenTotal int
	ChildrenUUIDs []uuid.UUID

	CommentsTotal   int
	ActivitiesTotal int64
	Activities      []Activity

	CreatedAt  time.Time
	UpdatedAt  time.Time
	ActivityAt time.Time
	DeletedAt  *time.Time
}

func NewTask(name string, federationUUID, companyUUID, projectUUID uuid.UUID, createdBy string, opts TaskCreateOptions) (Task, error) {
	if len(name) < 3 || len(name) > 100 {
		return Task{}, errors.New("название задачи должно быть от 3 до 100 символов")
	}

	if createdBy == "" {
		return Task{}, errors.New("создатель задачи обязателен")
	}

	uid := uuid.New()

	path := lo.WithoutEmpty(lo.Uniq(append(opts.ParentPath, uid.String())))

	allPeople := lo.WithoutEmpty(lo.Uniq([]string{
		createdBy,
		opts.ImplementBy,
		opts.ResponsibleBy,
		opts.ManagedBy,
	}))
	allPeople = append(allPeople, lo.WithoutEmpty(opts.CoWorkersBy)...)

	return Task{
		UUID:           uid,
		Name:           name,
		Description:    opts.Description,
		FederationUUID: federationUUID,
		CompanyUUID:    companyUUID,
		ProjectUUID:    projectUUID,
		CreatedBy:      createdBy,
		ManagedBy:      opts.ManagedBy,
		ResponsibleBy:  opts.ResponsibleBy,
		ImplementBy:    opts.ImplementBy,
		CoWorkersBy:    lo.WithoutEmpty(lo.Uniq(opts.CoWorkersBy)),
		Tags:           lo.WithoutEmpty(lo.Uniq(opts.Tags)),
		People:         allPeople,
		Path:           path,
		Priority:       opts.Priority,
		Icon:           opts.Icon,
		FinishTo:       opts.FinishTo,
		Fields:         make(map[string]any),
		RawFields:      make(map[string]any),
		Meta:           make(map[string]any),
		Dirty:          make(map[string]any),
		CreatedAt:      time.Now(),
	}, nil
}

// опциональные параметры создания задачи
// выделены в отдельную структуру чтобы не раздувать NewTask
type TaskCreateOptions struct {
	Description   string
	ParentPath    []string
	Tags          []string
	CoWorkersBy   []string
	ImplementBy   string
	ResponsibleBy string
	ManagedBy     string
	Priority      int
	Icon          string
	FinishTo      *time.Time
}

// обновляет название задачи с валидацией
// сохраняет предыдущее значение в Dirty для истории изменений
func (t *Task) PatchName(name string) error {
	if len(name) < 3 || len(name) > 100 {
		return errors.New("название должно быть от 3 до 100 символов")
	}

	t.recordDirty("name", t.Name)
	t.Name = name

	return nil
}

// переводит задачу в новый статус
// возвращает путь переходов(мжет быть несколько шагов) и ошибку
func (t *Task) PatchStatus(newStatus int, opts ProjectOptions, comment string, sg *StatusGraph) ([]string, error) {
	if newStatus == t.Status {
		return nil, errors.New("задача уже имеет этот статус")
	}

	if newStatus == StatusUnknown {
		return nil, errors.New("нельзя перевести задачу в статус 'Необработана'")
	}

	if sg == nil || len(sg.Graph) == 0 {
		sg, _ = defaultStatusGraph()
	}

	sg.Current = fmt.Sprint(t.Status)

	allowed, path := CheckPathByValue(sg, fmt.Sprint(t.Status), fmt.Sprint(newStatus))
	if !allowed {
		return nil, fmt.Errorf("переход из статуса %d в %d не разрешен", t.Status, newStatus)
	}

	if newStatus == StatusCancel && opts.RequireCancelComment() && comment == "" {
		return nil, errors.New("при отмене задачи необходимо указать причину")
	}

	if newStatus == StatusDone && opts.RequireDoneComment() && comment == "" {
		return nil, errors.New("при завершении задачи необходимо указать причину")
	}

	t.recordDirty("status", t.Status)
	t.Status = newStatus

	return path, nil
}

func (t *Task) recordDirty(field string, value any) {
	if t.Dirty == nil {
		t.Dirty = make(map[string]any)
	}

	t.Dirty[strings.ToLower(field)] = value
}

// возвращает граф переходов статусов по умолчанию
// используется если в проекте не задан кастомный граф
func defaultStatusGraph() (*StatusGraph, error) {
	sg, err := NewStatusGraph("0")
	if err != nil {
		return nil, err
	}
	sg.Graph = map[string][]string{
		"0": {"1"},
		"1": {"2"},
		"2": {"3", "4", "6"},
		"3": {"2"},
		"4": {"5", "2"},
		"5": {"2"},
		"6": {"2"},
	}

	return sg, nil
}

type Activity struct {
	UUID        uuid.UUID
	EntityUUID  uuid.UUID
	EntityType  string
	Description string
	Type        int
	Meta        map[string]any
	CreatedBy   User
	CreatedAt   time.Time
}

// фиксирует момент приостановки задачи с причиной
// используется для анализа времени простоя задачё
type Stop struct {
	UUID          uuid.UUID `json:"uuid"`
	StatusID      int       `json:"status_id"`
	StatusName    string    `json:"status_name"`
	Comment       string    `json:"comment"`
	CreatedBy     string    `json:"created_by"`
	CreatedByUUID uuid.UUID `json:"created_by_uuid"`
	CreatedAt     time.Time `json:"created_at"`
}
