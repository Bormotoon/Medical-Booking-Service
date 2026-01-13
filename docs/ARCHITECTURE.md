# Архитектура Medical Booking Service

## Обзор

Medical Booking Service — это монорепозиторий, содержащий два взаимосвязанных Telegram-бота для системы бронирования медицинских услуг.

```
Medical Booking Service/
├── bronivik_jr/          # Бот 1: Бронирование аппаратов (устройств)
├── bronivik_crm/         # Бот 2: Бронирование кабинетов (CRM)
├── shared/               # Общие модули (напоминания, аудит, access control)
├── docs/                 # Документация
├── features.md           # Техническое задание
├── TODO.md               # План разработки
└── README.md             # Главный README
```

## Компоненты

### Бот 1: bronivik_jr (Бронирование аппаратов)

**Назначение**: Управление бронированием медицинских аппаратов/устройств.

**Технологии**:
- Go 1.24
- SQLite (WAL mode)
- Telegram Bot API (go-telegram-bot-api/v5)
- gRPC + HTTP API
- Redis (опционально, для кэширования)
- Google Sheets API (для синхронизации)

**Структура**:
```
bronivik_jr/
├── cmd/                  # Точки входа
├── internal/
│   ├── api/              # HTTP/gRPC сервисы
│   ├── bot/              # Telegram бот логика
│   ├── config/           # Конфигурация
│   ├── database/         # SQLite репозиторий
│   ├── domain/           # Бизнес-логика
│   ├── google/           # Google Sheets интеграция
│   ├── models/           # Модели данных
│   ├── repository/       # Абстракции хранилища
│   ├── service/          # Сервисный слой
│   └── worker/           # Фоновые задачи
├── configs/              # Конфигурационные файлы
└── proto/                # gRPC proto-файлы
```

**Ключевые сущности**:
- `Item` — медицинский аппарат (id, name, total_quantity, is_active)
- `Booking` — бронирование аппарата (user_id, item_id, date, status)
- `User` — пользователь (telegram_id, is_manager, is_blacklisted)

**API эндпоинты**:
- `GET /api/v1/items` — список аппаратов
- `GET /api/v1/availability/{item}?date=YYYY-MM-DD` — доступность аппарата
- `POST /api/v1/availability/bulk` — массовая проверка доступности

### Бот 2: bronivik_crm (Бронирование кабинетов)

**Назначение**: CRM-система для бронирования кабинетов с почасовыми слотами.

**Технологии**:
- Go 1.22
- SQLite
- Telegram Bot API
- HTTP-клиент для интеграции с Ботом 1

**Структура**:
```
bronivik_crm/
├── cmd/                  # Точки входа
├── internal/
│   ├── bot/              # Telegram бот логика
│   ├── config/           # Конфигурация
│   ├── crmapi/           # Клиент API Бота 1
│   ├── db/               # SQLite репозиторий
│   ├── metrics/          # Prometheus метрики
│   └── model/            # Модели данных
└── configs/              # Конфигурационные файлы
```

**Ключевые сущности**:
- `Cabinet` — кабинет (id, name, is_active)
- `CabinetSchedule` — расписание кабинета (day_of_week, start_time, end_time, slot_duration)
- `CabinetScheduleOverride` — переопределение расписания на дату
- `HourlyBooking` — почасовое бронирование (cabinet_id, start_time, end_time, status)
- `User` — пользователь (telegram_id, is_manager, is_blacklisted)

## Интеграция между ботами

```
┌─────────────────┐         HTTP API          ┌─────────────────┐
│   bronivik_crm  │ ────────────────────────► │   bronivik_jr   │
│   (Бот 2)       │  GET /api/v1/items        │   (Бот 1)       │
│                 │  GET /api/v1/availability │                 │
│   Кабинеты      │  POST /api/v1/book-device │   Аппараты      │
└─────────────────┘                           └─────────────────┘
```

### Процесс бронирования кабинета с аппаратом:

1. Пользователь выбирает кабинет, дату, время в Боте 2
2. Бот 2 запрашивает список аппаратов у Бота 1 (`GET /api/v1/items`)
3. Пользователь выбирает аппарат
4. Бот 2 проверяет доступность кабинета (локально) и аппарата (через API Бота 1)
5. Бот 2 создает заявку со статусом `pending`
6. Менеджер подтверждает/отклоняет заявку

## Общие модули (shared/)

### Напоминания (reminders)
- Планировщик проверяет заявки за 24 часа до начала
- Отправляет уведомление пользователю
- Управление настройками напоминаний через `user_settings`

### Аудит и экспорт (audit)
- Ежемесячный экспорт в XLS (1-го числа)
- Автоудаление данных старше 31 дня
- Логирование всех действий

### Access Control (access)
- Проверка черного списка (blocklist)
- Проверка прав менеджера
- Middleware для обоих ботов

## База данных

### Общие таблицы (в каждом боте)

```sql
-- Пользователи
CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    telegram_id INTEGER UNIQUE NOT NULL,
    username TEXT,
    first_name TEXT,
    last_name TEXT,
    phone TEXT,
    is_manager BOOLEAN DEFAULT 0,
    is_blacklisted BOOLEAN DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Настройки пользователя (для напоминаний)
CREATE TABLE user_settings (
    user_id INTEGER PRIMARY KEY,
    reminders_enabled BOOLEAN DEFAULT 1,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

-- Менеджеры (можно использовать флаг в users)
CREATE TABLE managers (
    user_id INTEGER PRIMARY KEY,
    chat_id INTEGER,
    name TEXT,
    added_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

-- Черный список (можно использовать флаг в users)
CREATE TABLE blocked_users (
    user_id INTEGER PRIMARY KEY,
    blocked_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    reason TEXT,
    FOREIGN KEY (user_id) REFERENCES users(id)
);
```

## Конфигурация

### Переменные окружения

```bash
# Общие
BOT_TOKEN=your_telegram_bot_token
DATABASE_PATH=./data/bot.db
LOG_LEVEL=info

# Бот 1 (bronivik_jr)
API_PORT=8080
API_KEY=secret_api_key

# Бот 2 (bronivik_crm)
BOT1_API_URL=http://localhost:8080
BOT1_API_KEY=secret_api_key
ROOMS_COUNT=3
```

## Статусы заявок

| Статус | Описание |
|--------|----------|
| `pending` | Ожидает подтверждения менеджером |
| `approved` / `confirmed` | Подтверждена |
| `rejected` | Отклонена |
| `canceled` | Отменена пользователем |
| `needs_revision` | Возвращена на доработку |
| `completed` | Завершена |
| `changed` | Изменена |

## Мониторинг

- Prometheus метрики на `/metrics`
- Health check на `/healthz`
- Readiness check на `/readyz`

## Безопасность

- API-ключи для межсервисного взаимодействия
- Rate limiting
- Валидация входных данных
- Проверка прав доступа

---

## Архитектурные решения

### Политика напоминаний

#### Типы напоминаний
Система поддерживает следующие типы напоминаний:

| Тип | Код | Описание |
|-----|-----|----------|
| За 24 часа | `24h_before` | Отправляется за 24 часа до начала бронирования |
| В день записи | `day_of_booking` | Отправляется в 09:00 в день бронирования |
| Пользовательский | `custom` | Зарезервировано для будущих расширений |

**MVP**: Используется только `24h_before`. Остальные типы — для расширения.

#### Таймзона
- **Рабочая таймзона**: `Europe/Moscow` (МСК, UTC+3)
- **Хранение в БД**: Все времена хранятся в UTC
- **Отображение**: Конвертируется в МСК при показе пользователю
- **Переменная окружения**: `TZ=Europe/Moscow`

#### Статусы уведомлений

| Статус | Описание |
|--------|----------|
| `pending` | Ожидает отправки |
| `scheduled` | Запланировано к отправке |
| `processing` | В процессе отправки (захвачено воркером) |
| `sent` | Успешно отправлено |
| `failed` | Ошибка отправки (после всех retry) |
| `cancelled` | Отменено (бронирование отменено) |

#### Критерии успешной отправки
- **Успех**: HTTP 200 от Telegram API
- **Хранение**: `sent_at` timestamp + `status='sent'`
- **Retry**: При ошибках — до 3 попыток с exponential backoff

### Ограничения Telegram API

#### Rate Limits
| Параметр | Значение | Описание |
|----------|----------|----------|
| `RATE_LIMIT_PER_SECOND` | 20 | Максимум сообщений в секунду |
| `RATE_LIMIT_BURST` | 30 | Максимальный burst |
| `JITTER_MIN_MS` | 50 | Минимальная случайная задержка |
| `JITTER_MAX_MS` | 150 | Максимальная случайная задержка |

#### Политика повторных попыток (Retry)
| Параметр | Значение |
|----------|----------|
| `MAX_RETRIES` | 3 |
| `RETRY_DELAYS` | [1s, 5s, 30s] |

**Обработка ошибок**:
- `429 Too Many Requests` — ждать `Retry-After` из заголовка
- `403 Forbidden` (blocked by user) — не повторять, статус `failed`
- `400 Bad Request` — не повторять, статус `failed`
- `5xx` — повторить с exponential backoff

#### Дедупликация
- **Уникальный ключ**: `(user_id, booking_id, reminder_type)`
- **Механизм**: UNIQUE constraint в БД + статус `sent` блокирует повторную отправку

### База данных

#### Текущее решение
- **bronivik_jr**: SQLite (WAL mode)
- **bronivik_crm**: SQLite

#### Миграция на PostgreSQL
**Решение**: Отложить миграцию для MVP, подготовить абстракции.

**Причины**:
- SQLite достаточен для текущей нагрузки
- Миграция требует downtime и отдельного планирования
- Абстракция `database.Repository` позволяет легко мигрировать позже

**Подготовка к миграции**:
- Все SQL запросы совместимы с PostgreSQL
- Database interface для абстракции

#### Стратегия TTL (Time-To-Live)
**Механизм**: Приложение (cron-задача)

```sql
-- Очистка старых бронирований (запускать ежедневно)
DELETE FROM bookings 
WHERE created_at < datetime('now', '-31 days')
  AND status IN ('completed', 'canceled', 'rejected');

-- Очистка отправленных напоминаний (запускать ежедневно)
DELETE FROM reminders
WHERE status = 'sent' AND sent_at < datetime('now', '-1 day');

-- Очистка failed напоминаний (запускать еженедельно)
DELETE FROM reminders
WHERE status = 'failed' AND updated_at < datetime('now', '-3 days');
```

**Частота запуска**: 
- Бронирования: ежедневно в 03:00 МСК
- Напоминания: ежедневно после основной рассылки

---

*Документ создан: 13 января 2026*
*Последнее обновление: 13 января 2026*
