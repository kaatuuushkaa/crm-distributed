# CRM Distributed

Масштабируемая микросервисная CRM-система на Go.  
Разработана в рамках ВКР по теме «Исследование эффективности языка Go
в проектировании масштабируемых распределённых систем».

### Стек технологий

| Компонент        | Технология                    |
|------------------|-------------------------------|
| HTTP Framework   | Echo v4                       |
| База данных      | PostgreSQL 16 + pgx v5        |
| Кэш / Pub-Sub    | Redis 7                       |
| Брокер сообщений | Apache Kafka 3.7 (KRaft)      |
| Object Storage   | MinIO                         |
| Межсервисный RPC | gRPC + Protocol Buffers       |
| Real-time        | WebSocket + Redis Pub/Sub     |
| Метрики          | Prometheus + Grafana          |
| Контейнеры       | Docker (distroless, ~28MB)    |
| Оркестрация      | Kubernetes (kind для local)   |

## Быстрый старт

### Требования

- Go 1.22+
- Docker Desktop
- make
- golang-migrate (`brew install golang-migrate`)

### Локальный запуск

```bash
# 1. Клонируем
git clone https://github.com/kaatuuushkaa/crm-distributed
cd crm-distributed

# 2. Создаём .env
cp .env.example .env

# 3. Запускаем инфраструктуру
make infra-up

# 4. Применяем миграции
make migrate-up

# 5. Запускаем сервисы (три терминала)
source .env && make run-task
source .env && make run-notif
source .env && make run-doc
```

### Проверка работоспособности

```bash
# Health checks
curl http://localhost:8080/healthz
curl http://localhost:8081/healthz
curl http://localhost:8082/healthz

# Регистрация пользователя
curl -s -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"name":"Катя","lname":"Торосина","email":"katya@test.com","password":"password123"}' | jq

# Логин и получение токена
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"katya@test.com","password":"password123"}' | jq -r '.access_token')

# Подключение WebSocket
wscat -c "ws://localhost:8081/ws?token=$TOKEN"
```

## Make команды

```bash
make infra-up      # запустить инфраструктуру (postgres, redis, kafka, minio)
make infra-down    # остановить инфраструктуру
make migrate-up    # применить миграции
make migrate-down  # откатить миграции
make run-task      # запустить task-service
make run-notif     # запустить notification-service
make run-doc       # запустить document-service
make build         # собрать все сервисы
make test          # запустить тесты с -race
make lint          # запустить golangci-lint
make proto         # регенерировать gRPC код
```

## API Endpoints

### task-service (:8080)

| Метод  | Путь                              | Описание                    |
|--------|-----------------------------------|-----------------------------|
| POST   | /api/v1/auth/register             | Регистрация                 |
| POST   | /api/v1/auth/login                | Вход                        |
| POST   | /api/v1/auth/refresh              | Обновление токена           |
| POST   | /api/v1/federations               | Создать федерацию           |
| POST   | /api/v1/projects                  | Создать проект              |
| POST   | /api/v1/tasks                     | Создать задачу              |
| GET    | /api/v1/tasks/:uuid               | Получить задачу             |
| PATCH  | /api/v1/tasks/:uuid/status        | Изменить статус             |
| DELETE | /api/v1/tasks/:uuid               | Удалить задачу              |
| GET    | /api/v1/projects/:uuid/tasks      | Список задач проекта        |

### notification-service (:8081)

| Метод  | Путь       | Описание                         |
|--------|------------|----------------------------------|
| GET    | /ws        | WebSocket (?token=ACCESS_TOKEN)  |
| GET    | /healthz   | Liveness probe                   |
| GET    | /readyz    | Readiness probe                  |
| GET    | /metrics   | Prometheus метрики               |

### document-service (:8082)

| Метод  | Путь                                  | Описание                |
|--------|---------------------------------------|-------------------------|
| POST   | /api/v1/legal-entities                | Создать юр. лицо        |
| GET    | /api/v1/legal-entities/:uuid          | Получить юр. лицо       |
| GET    | /api/v1/companies/:uuid/legal-entities| Список юр. лиц          |
| POST   | /api/v1/legal-entities/:uuid/accounts | Создать счёт            |
| GET    | /api/v1/legal-entities/:uuid/accounts | Список счетов           |
| POST   | /api/v1/files                         | Загрузить файл          |
| GET    | /api/v1/files/:uuid                   | Скачать файл            |
| GET    | /api/v1/files/:uuid/presigned         | Временная ссылка        |
| DELETE | /api/v1/files/:uuid                   | Удалить файл            |

## Kafka Topics

| Топик             | Публикует      | Потребляет              |
|-------------------|----------------|-------------------------|
| task.created      | task-service   | notification-service    |
| task.updated      | task-service   | notification-service    |
| task.deleted      | task-service   | notification-service    |
| project.created   | task-service   | —                       |
| company.created   | task-service   | document-service        |

## Развёртывание в Kubernetes

```bash
# Создаём кластер
make k8s-up

# Деплоим
make k8s-deploy

# Проверяем
kubectl get pods -n crm
```
