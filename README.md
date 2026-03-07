# Medical Booking Service

> 🇬🇧 [English version](README.en.md)

Комплексная система бронирования медицинского оборудования и кабинетов на базе Telegram-ботов.

[![License: MPL 2.0](https://img.shields.io/badge/License-MPL%202.0-brightgreen.svg)](https://opensource.org/licenses/MPL-2.0)
[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://golang.org/)
[![Docker](https://img.shields.io/badge/Docker-ready-2496ED?logo=docker)](https://www.docker.com/)

---

## 📋 Содержание

- [Обзор](#-обзор)
- [Архитектура](#-архитектура)
- [Основные возможности](#-основные-возможности)
- [Требования](#-требования)
- [Быстрый старт](#-быстрый-старт)
- [Конфигурация](#️-конфигурация)
- [Команды ботов](#-команды-ботов)
- [API](#-api)
- [Мониторинг](#-мониторинг)
- [Разработка](#-разработка)
- [Обновление подпроектов](#-обновление-подпроектов-git-subtree)
- [Лицензия](#-лицензия)

---

## 🎯 Обзор

Medical Booking Service — это монорепозиторий, объединяющий два взаимосвязанных Telegram-бота для автоматизации процесса бронирования медицинских ресурсов:

1. **Bronivik Jr** (`bronivik_jr/`) — основной сервис для бронирования медицинского оборудования на полный день
2. **Bronivik CRM** (`bronivik_crm/`) — специализированный бот для почасового бронирования физических кабинетов с автоматической проверкой доступности оборудования

Система обеспечивает:
- 🤖 Удобный интерфейс через Telegram
- 📊 Интеграцию с Google Sheets для отчётности
- 🔄 Автоматическую синхронизацию данных между ботами
- 🔔 Систему напоминаний о предстоящих бронированиях
- 📈 Мониторинг и метрики (Prometheus + Grafana)
- 🔒 Управление доступом и права менеджеров

---

## 🏗️ Архитектура

```
Medical-Booking-Service/
├── bronivik_jr/          # Бот 1: Бронирование оборудования
│   ├── cmd/
│   │   ├── bot/          # Telegram Bot
│   │   └── api/          # REST API & gRPC Server
│   ├── internal/
│   │   ├── api/          # HTTP & gRPC API
│   │   ├── bot/          # Бизнес-логика бота
│   │   ├── database/     # SQLite (WAL)
│   │   ├── google/       # Google Sheets Worker
│   │   └── worker/       # Background Jobs
│   └── configs/          # Конфигурация (config.yaml, items.yaml)
│
├── bronivik_crm/         # Бот 2: Бронирование кабинетов
│   ├── cmd/
│   │   └── bot/          # Telegram Bot
│   ├── internal/
│   │   ├── bot/          # Логика бота
│   │   ├── booking/      # FSM бронирования
│   │   ├── crmapi/       # HTTP-клиент для Bronivik Jr API
│   │   ├── db/           # SQLite база данных
│   │   └── manager/      # Панель менеджера
│   └── configs/          # Конфигурация (config.yaml, cabinets.yaml)
│
├── shared/               # Общие модули
│   ├── access/          # Управление доступом (blocklist, managers)
│   ├── audit/           # Аудит и экспорт данных
│   ├── reminders/       # Система напоминаний
│   └── utils/           # Общие утилиты
│
├── monitoring/          # Конфигурация мониторинга
│   ├── prometheus.yml   # Prometheus конфигурация
│   ├── alerts.yml       # Правила алертинга
│   └── grafana/         # Dashboards и datasources
│
├── docs/                # Документация
│   ├── ARCHITECTURE.md  # Архитектура системы
│   ├── DATABASE_SCHEMA.md # Схема базы данных
│   ├── MANAGER_GUIDE.md # Руководство менеджера
│   ├── ROLLBACK.md      # План отката изменений
│   └── openapi.yaml     # OpenAPI спецификация
│
├── scripts/             # Утилиты и скрипты
│   └── migrate.sh       # Система миграций БД
│
├── docker-compose.yml   # Оркестрация сервисов
└── .env.example         # Шаблон переменных окружения
```

### Компоненты Bronivik Jr (Основной сервис)

- **Telegram Bot** — интерфейс для пользователей и менеджеров
- **REST API & gRPC** — точки интеграции для внешних сервисов (включая CRM бот)
- **SQLite (WAL)** — основное хранилище данных (брони, пользователи, оборудование)
- **Google Sheets Worker** — асинхронная синхронизация с Google Таблицами
- **Event Bus** — внутренняя шина событий для разделения логики
- **Redis** — кэширование и Rate Limiting
- **Reminder Worker** — фоновая отправка напоминаний (cron-задача)

### Компоненты Bronivik CRM (Кабинеты)

- **Telegram Bot** — интерфейс для почасового бронирования
- **FSM Engine** — конечный автомат для диалогов бронирования
- **API Client** — интеграция с Bronivik Jr для проверки оборудования
- **SQLite** — локальная БД для расписания и броней
- **Redis** — кэширование API-запросов

---

## ✨ Основные возможности

### Общие для обеих систем

- ✅ **Автоматические напоминания** — за 24 часа до брони с настраиваемыми уведомлениями
- ✅ **Ежемесячный аудит** — экспорт всех данных в Excel (1-го числа каждого месяца)
- ✅ **Управление доступом** — чёрный список пользователей и список менеджеров
- ✅ **TTL политика** — автоматическое удаление данных старше 31 дня
- ✅ **Метрики и мониторинг** — Prometheus + Grafana для observability
- ✅ **Health Checks** — проверка работоспособности всех компонентов

### Bronivik Jr (Оборудование)

- 📱 Бронирование медицинского оборудования на полный день
- 🔄 Диапазонные бронирования ("вечная аренда" для CRM)
- 📊 Синхронизация с Google Sheets
- 🔌 REST API и gRPC для интеграции
- ✅ Подтверждение заявок менеджером
- 📈 Статистика и отчёты
- 🔍 Проверка доступности через API

### Bronivik CRM (Кабинеты)

- 🏥 Почасовое бронирование физических кабинетов
- ⏰ Слоты по 30 минут с возможностью выбора нескольких подряд
- 🔗 Автоматическая проверка доступности оборудования через API Bronivik Jr
- 📅 Гибкое управление расписанием (рабочие часы, обеды, праздники)
- 👥 Сбор данных клиента (ФИО, телефон)
- 🎛️ Расширенная панель менеджера для управления заявками
- 📋 Ручное создание заявок менеджером (запись по телефону)

---

## 📦 Требования

- **Go** 1.24+ (для локальной разработки)
- **Docker** 20.10+ и **Docker Compose** 2.0+
- **Redis** 7+ (автоматически запускается в docker-compose)
- **SQLite3** (встроен в Docker-образы)
- **Google Cloud Service Account** (опционально, для синхронизации с Google Sheets)

PostgreSQL не входит в поддерживаемые runtime-сценарии; `bronivik_jr` и `bronivik_crm` используют только SQLite.

---

## 🚀 Быстрый старт

### 1. Клонирование репозитория

```bash
git clone https://github.com/Bormotoon/Medical-Booking-Service.git
cd Medical-Booking-Service
```

### 2. Настройка переменных окружения

```bash
cp .env.example .env
```

Отредактируйте `.env` и заполните обязательные переменные:

```env
# Telegram токены (получите у @BotFather)
BOT_TOKEN=your_bot_token_here
CRM_BOT_TOKEN=your_crm_bot_token_here

# API авторизация
API_AUTH_KEYS=key1:extra1,key2:extra2
CRM_API_KEY=key1
CRM_API_EXTRA=extra1

# Менеджеры (Telegram User IDs через запятую)
MANAGERS=123456789,987654321

# Google Sheets (опционально)
GOOGLE_SPREADSHEET_ID=your_spreadsheet_id
CRM_GOOGLE_SPREADSHEET_ID=your_crm_spreadsheet_id
```

### 3. (Опционально) Настройка Google Sheets

Если требуется синхронизация с Google Таблицами:

1. Создайте Service Account в [Google Cloud Console](https://console.cloud.google.com/)
2. Скачайте JSON-ключ
3. Поместите файл в `credentials/google-credentials.json`
4. Предоставьте доступ Service Account к вашей таблице

### 4. Запуск сервисов

```bash
# Запуск всех сервисов
docker compose up -d --build

# Запуск с мониторингом (Prometheus + Grafana)
docker compose --profile monitoring up -d --build

# Просмотр логов
docker compose logs -f

# Остановка сервисов
docker compose down
```

### 5. Проверка работоспособности

```bash
# Bronivik Jr API
curl http://localhost:8080/healthz

# Bronivik CRM
curl http://localhost:8090/healthz

# Redis
docker compose exec redis redis-cli ping

# Prometheus (если запущен)
curl http://localhost:9090/-/healthy

# Grafana (если запущен)
open http://localhost:3000  # admin/admin
```

---

## ⚙️ Конфигурация

### Основной сервис (Bronivik Jr)

Файл: `bronivik_jr/configs/config.yaml`

```yaml
app:
  name: "bronivik-jr"
  environment: "production"
  version: "1.0.0"

telegram:
  bot_token: ${BOT_TOKEN}
  debug: false

database:
  path: "./data/bronivik_jr.db"

google:
  credentials_file: ${GOOGLE_CREDENTIALS_FILE}
  bookings_spreadsheet_id: ${GOOGLE_SPREADSHEET_ID}

api:
  enabled: true
  grpc_port: 8081
  http:
    enabled: true
    port: 8080
  auth:
    enabled: true
    keys: ["${API_AUTH_KEYS}"]

monitoring:
  prometheus_enabled: true
  prometheus_port: 9090
```

### Список оборудования

Файл: `bronivik_jr/configs/items.yaml`

```yaml
items:
  - id: 1
    name: "УЗИ аппарат Philips"
    category: "УЗИ"
    quantity: 2
    order: 1
  - id: 2
    name: "Рентген аппарат GE"
    category: "Рентген"
    quantity: 1
    order: 2
```

### CRM Бот

Файл: `bronivik_crm/configs/config.yaml`

```yaml
telegram:
  bot_token: ${CRM_BOT_TOKEN}

api:
  base_url: "http://bronivik-jr-api:8080"
  api_key: ${CRM_API_KEY}
  api_extra: ${CRM_API_EXTRA}
  cache_ttl_seconds: 300

booking:
  min_advance_minutes: 60
  max_advance_days: 30
  max_active_per_user: 0  # 0 = без лимита

managers:
  - 123456789

monitoring:
  prometheus_enabled: true
  health_check_port: 8090
```

### Конфигурация кабинетов

Файл: `bronivik_crm/configs/cabinets.yaml`

```yaml
defaults:
  schedule:
    start_time: "10:00"
    end_time: "22:00"
    slot_duration: 30
    lunch_start: null
    lunch_end: null

cabinets:
  - id: 1
    name: "Кабинет №1"
    number: "101"
    floor: 1
    capacity: 2
    enabled: true
    
  - id: 2
    name: "Кабинет №2"
    number: "102"
    floor: 1
    capacity: 3
    enabled: true

holidays:
  - date: "2026-01-01"
    name: "Новый год"
  - date: "2026-05-01"
    name: "День труда"
```

---

## 💬 Команды ботов

### Bronivik Jr (Основной бот)

#### Пользовательские команды

- `/start` — начало работы, регистрация
- `/book` — запустить мастер бронирования оборудования
- `/my_bookings` — список моих активных броней
- `/cancel_booking <ID>` — отмена брони по ID
- `/help` — справка по командам

#### Команды менеджера

- `/approve <ID>` — подтвердить бронь
- `/reject <ID>` — отклонить бронь
- `/stats [период]` — статистика за период
- `/export_bookings` — ручная синхронизация с Google Sheets
- `/pending` — список заявок на подтверждение

### Bronivik CRM (Бот кабинетов)

#### Пользовательские команды

- `/start` — начало работы
- `/book` — начать процесс бронирования кабинета
  - Выбор кабинета
  - Выбор даты (интерактивный календарь)
  - Выбор временного слота
  - Выбор длительности (количество слотов по 30 минут)
  - Выбор оборудования (или "Без аппарата")
  - Ввод данных клиента (ФИО, телефон)
- `/my_bookings` — мои записи в кабинеты
- `/cancel_booking <ID>` — отмена записи

#### Команды менеджера

- `/pending` — список заявок, ожидающих подтверждения
- `/today_schedule` — расписание кабинетов на сегодня
- `/tomorrow_schedule` — расписание на завтра
- `/add_cabinet <name>` — добавить новый кабинет
- `/list_cabinets` — просмотр всех кабинетов
- `/set_schedule <cab_id> <day> <start> <end>` — настройка расписания

---

## 🔌 API

### REST API (Bronivik Jr)

Порт: `8080` (HTTP), `8081` (gRPC)

#### Основные эндпоинты

```bash
# Список всего оборудования
GET /api/v1/items

# Проверка доступности оборудования
GET /api/v1/availability/{item_name}?date=YYYY-MM-DD

# Массовая проверка доступности
POST /api/v1/availability/bulk
Content-Type: application/json
{
  "items": ["УЗИ аппарат Philips", "Рентген аппарат GE"],
  "start_date": "2026-01-20",
  "end_date": "2026-01-25"
}

# Список устройств для CRM
GET /api/devices?date=YYYY-MM-DD&include_reserved=true

# Бронирование устройства (для CRM)
POST /api/book-device
Content-Type: application/json
x-api-key: your_api_key
x-api-extra: your_api_extra
{
  "device_id": 1,
  "date": "2026-01-20",
  "external_booking_id": "crm-12345",
  "client_name": "Иванов Иван",
  "client_phone": "+79991234567"
}

# Отмена внешнего бронирования
DELETE /api/book-device/{external_id}

# Health Check
GET /healthz
GET /readyz
```

#### Авторизация

API использует header-based авторизацию:

```bash
curl -H "x-api-key: your_key" \
     -H "x-api-extra: your_extra" \
     http://localhost:8080/api/v1/items
```

Полная OpenAPI спецификация: [`docs/openapi.yaml`](docs/openapi.yaml)

---

## 📊 Мониторинг

### Health Checks

- **Bronivik Jr API**: `http://localhost:8080/healthz`
- **Bronivik CRM**: `http://localhost:8090/healthz`
- **Redis**: `docker compose exec redis redis-cli ping`

### Prometheus Metrics

Порт: `9090` (при запуске с профилем `monitoring`)

```bash
# Просмотр метрик
open http://localhost:9090/metrics
open http://localhost:9090/graph
```

#### Ключевые метрики

- `reminders_sent_total` — отправленные напоминания
- `reminders_queue_size` — размер очереди напоминаний
- `reminder_send_duration_seconds` — время отправки напоминаний
- `api_requests_total` — общее количество API-запросов
- `api_request_duration_seconds` — длительность API-запросов
- `booking_operations_total` — операции бронирования

### Grafana Dashboards

Порт: `3000` (при запуске с профилем `monitoring`)

- URL: `http://localhost:3000`
- Логин: `admin`
- Пароль: `admin` (измените при первом входе)

Готовые дашборды в `monitoring/grafana/provisioning/dashboards/`

### Alerting

Конфигурация алертов: `monitoring/alerts.yml`

Группы алертов:
- **reminders** — проблемы с напоминаниями
- **api** — проблемы с API
- **bots** — проблемы с ботами
- **database** — проблемы с БД
- **redis** — проблемы с Redis
- **resources** — использование ресурсов

---

## 🛠️ Разработка

### Локальный запуск (без Docker)

#### Требования

```bash
# Установка Go 1.24+
go version

# Установка зависимостей для обоих модулей
cd bronivik_jr && go mod download && cd ..
cd bronivik_crm && go mod download && cd ..

# Установка линтера (опционально)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

#### Запуск Redis

```bash
docker compose up -d redis
```

#### Запуск Bronivik Jr

```bash
# Terminal 1: Bot
cd bronivik_jr
go run ./cmd/bot --config=configs/config.yaml

# Terminal 2: API Server
cd bronivik_jr
go run ./cmd/api --config=configs/config.yaml

# Terminal 3: Worker (напоминания)
cd bronivik_jr
go run ./cmd/bot worker --job=reminders
```

#### Запуск Bronivik CRM

```bash
cd bronivik_crm
go run ./cmd/bot --config=configs/config.yaml
```

### Тестирование

```bash
# Запуск всех тестов для обоих модулей
make test

# Запуск тестов с покрытием
make test-coverage

# Тесты для конкретного модуля
cd bronivik_jr && go test ./... -v
cd bronivik_crm && go test ./... -v

# Интеграционные тесты
go test ./internal/api/... -tags=integration -v
```

### Линтинг

```bash
# Запуск линтера
make lint

# Или напрямую
cd bronivik_jr && golangci-lint run
cd bronivik_crm && golangci-lint run
```

### Миграции БД

```bash
# Применить все миграции
./scripts/migrate.sh up all

# Откатить одну миграцию
./scripts/migrate.sh down bronivik_jr 1

# Проверить текущую версию
./scripts/migrate.sh version all

# Создать backup перед миграцией
./scripts/migrate.sh backup all
```

Подробнее: [`docs/ROLLBACK.md`](docs/ROLLBACK.md)

### Структура проекта

Подробное описание архитектуры: [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md)

---

## 🔄 Обновление подпроектов (git subtree)

Проект использует git subtree для управления подпроектами.

### Первичная настройка

Если remotes ещё не добавлены:

```bash
git remote add bronivik_jr https://github.com/Bormotoon/bronivik_jr.git
git remote add bronivik_crm https://github.com/Bormotoon/bronivik_crm.git
```

### Получение изменений из подпроектов

```bash
# Обновить bronivik_jr (ветка master)
git fetch bronivik_jr master
git subtree pull --prefix=bronivik_jr bronivik_jr master

# Обновить bronivik_crm (ветка main)
git fetch bronivik_crm main
git subtree pull --prefix=bronivik_crm bronivik_crm main
```

### Отправка изменений обратно в подпроекты

```bash
# Отправить изменения в bronivik_jr
git subtree push --prefix=bronivik_jr bronivik_jr master

# Отправить изменения в bronivik_crm
git subtree push --prefix=bronivik_crm bronivik_crm main
```

### Примечания

- Используйте `--squash` с `git subtree pull` для единого merge-коммита
- Убедитесь, что выполняете команды из корня репозитория
- Изменения в подкаталогах `bronivik_jr/` и `bronivik_crm/` автоматически включаются в историю монорепозитория

---

## 📚 Дополнительная документация

- [📖 Архитектура системы](docs/ARCHITECTURE.md)
- [💾 Схема базы данных](docs/DATABASE_SCHEMA.md)
- [👨‍💼 Руководство менеджера](docs/MANAGER_GUIDE.md)
- [🔙 План отката изменений](docs/ROLLBACK.md)
- [🔌 OpenAPI спецификация](docs/openapi.yaml)

---

## 📄 Лицензия

Этот проект лицензирован под [Mozilla Public License 2.0](LICENSE).

---

## 👥 Контакты и поддержка

- **GitHub**: [@Bormotoon](https://github.com/Bormotoon)
- **Основной проект (Jr)**: [bronivik_jr](https://github.com/Bormotoon/bronivik_jr)
- **CRM проект**: [bronivik_crm](https://github.com/Bormotoon/bronivik_crm)

---

## 🙏 Благодарности

Проект использует следующие open-source библиотеки:
- [Telegram Bot API](https://github.com/go-telegram-bot-api/telegram-bot-api)
- [Echo Framework](https://echo.labstack.com/)
- [SQLite](https://www.sqlite.org/)
- [Redis](https://redis.io/)
- [Prometheus](https://prometheus.io/)
- [Grafana](https://grafana.com/)

---

**Made with ❤️ for medical professionals**
