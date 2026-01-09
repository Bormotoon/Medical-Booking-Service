# Bronivik CRM Bot

Telegram-бот для почасового бронирования кабинетов с интеграцией в основной сервис [Bronivik Jr](https://github.com/Bormotoon/bronivik_jr).

## Обзор

Bronivik CRM — это специализированный Telegram-бот для управления арендой кабинетов по часам. При бронировании бот автоматически проверяет доступность выбранного оборудования через API основного сервиса Bronivik Jr, гарантируя корректное распределение ресурсов.

## Основные возможности

- **Почасовое бронирование**: выбор временных слотов по расписанию кабинета
- **Синхронизация оборудования**: опциональная проверка доступности аппаратов через API Bronivik Jr
- **Автоматические бэкапы**: фоновое резервное копирование базы данных SQLite (интервал 24ч, TTL 14 дней)
- **Интерактивный календарь**: удобный выбор даты через инлайн-клавиатуру
- **Управление клиентами**: сбор ФИО и телефона при бронировании
- **Панель менеджера**: быстрое подтверждение или отклонение заявок
- **Redis-кэширование**: ускорение проверок через API
- **Prometheus метрики**: мониторинг работы бота

## Требования

- Go 1.22+
- SQLite3
- Redis 7+ (опционально, рекомендуется для кэширования)
- Доступ к API Bronivik Jr (опционально, настраивается в конфиге)

## Быстрый старт

### Установка

```bash
git clone https://github.com/Bormotoon/bronivik_crm.git
cd bronivik_crm
```

### Конфигурация

1. Скопируйте файл с примером переменных окружения:

```bash
cp .env.example .env
```

2. Отредактируйте `.env`:

```env
CRM_BOT_TOKEN=your_telegram_bot_token
CRM_API_KEY=your_api_key
CRM_API_EXTRA=your_extra_key
MANAGERS=123456789
```

3. При необходимости настройте `configs/config.yaml`

### Запуск с Docker Compose (рекомендуется)

```bash
docker compose up -d --build
```

Это запустит:
- `bronivik-crm-bot`: основной бот (порт 8090 для health check)
- `bronivik-crm-redis`: Redis для кэширования

### Локальный запуск

```bash
# Установка зависимостей
go mod download

# Запуск бота
go run ./cmd/bot

# Или сборка и запуск
make build
./bin/bronivik-crm
```

## Архитектура

```
bronivik_crm/
├── cmd/
│   └── bot/          # Точка входа приложения
├── configs/          # Конфигурационные файлы
├── internal/
│   ├── bot/          # Логика Telegram бота
│   ├── config/       # Управление конфигурацией
│   ├── crmapi/       # HTTP-клиент для Bronivik Jr API
│   ├── db/           # Работа с SQLite (кабинеты, брони)
│   ├── metrics/      # Prometheus метрики
│   └── model/        # Доменные модели данных
├── Dockerfile
├── docker-compose.yml
└── Makefile
```

## Команды бота

### Пользовательские команды

- `/start` — приветствие и инструкции
- `/book` — начать процесс бронирования
  - Выбор кабинета
  - Выбор аппарата (или "Без аппарата")
  - Выбор даты
  - Выбор временного слота
  - Ввод данных клиента (ФИО, телефон)
  - Подтверждение брони
- `/my_bookings` — список активных бронирований
- `/cancel_booking <ID>` — отмена бронирования
- `/help` — справка по командам

### Команды менеджера

- `/pending` — список заявок, ожидающих подтверждения
- `/today_schedule` — расписание кабинетов на сегодня
- `/tomorrow_schedule` — расписание на завтра
- `/add_cabinet <name>` — добавить новый кабинет
- `/list_cabinets` — просмотр всех кабинетов
- `/set_schedule <cab_id> <day> <start> <end>` — настройка расписания

## Конфигурация

Основные разделы `configs/config.yaml`:

```yaml
telegram:
  bot_token: ${CRM_BOT_TOKEN}  # Токен Telegram бота

api:
  base_url: "http://localhost:8080"  # URL API Bronivik Jr
  api_key: ${CRM_API_KEY}
  api_extra: ${CRM_API_EXTRA}
  cache_ttl_seconds: 300  # TTL кэша Redis

booking:
  min_advance_minutes: 60    # Минимум за час до начала
  max_advance_days: 30       # Максимум 30 дней вперёд
  max_active_per_user: 0     # 0 = без лимита

managers:  # Telegram ID менеджеров
  - 123456789

monitoring:
  prometheus_enabled: true
  prometheus_port: 9090
  health_check_port: 8090
```

## Интеграция с Bronivik Jr

Бот использует REST API основного сервиса:

- `GET /api/v1/items` — получение списка оборудования
- `GET /api/v1/availability/{name}?date=YYYY-MM-DD` — проверка доступности
- `POST /api/v1/availability/bulk` — массовая проверка

Авторизация: заголовки `x-api-key` и `x-api-extra`

## Разработка

```bash
# Запуск тестов
make test

# Отчёт о покрытии
make test-coverage

# Линтинг
make lint

# Сборка
make build

# Очистка
make clean
```

## Мониторинг

- **Health Check**: `http://localhost:8090/healthz`
- **Readiness Check**: `http://localhost:8090/readyz`
- **Prometheus Metrics**: `http://localhost:9090/metrics` (если включено)

## База данных

SQLite с включённым режимом WAL (Write-Ahead Logging) для конкурентного доступа.

Основные таблицы:
- `users` — пользователи Telegram
- `cabinets` — физические кабинеты
- `cabinet_schedules` — расписание работы кабинетов
- `hourly_bookings` — почасовые бронирования

## Лицензия

MIT License

## Контакты

- GitHub: [Bormotoon](https://github.com/Bormotoon)
- Основной проект: [Bronivik Jr](https://github.com/Bormotoon/bronivik_jr)

