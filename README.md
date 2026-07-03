# Medical Booking Service

> English version: [README.en.md](README.en.md)

[![CI](https://github.com/gerruda/bronivik/actions/workflows/ci.yml/badge.svg)](https://github.com/gerruda/bronivik/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://go.dev/)
[![Docker](https://img.shields.io/badge/Docker-ready-2496ED?logo=docker)](https://www.docker.com/)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)

Комплексная система для бронирования медицинского оборудования и кабинетов через Telegram-ботов. Репозиторий объединяет два связанных сервиса, общие библиотеки, инфраструктуру мониторинга и документацию для эксплуатации.

## Что В Репозитории

- `bronivik_jr` — основной сервис бронирования оборудования на дату, с Telegram-ботом, HTTP/gRPC API и фоновой обработкой.
- `bronivik_crm` — сервис почасового бронирования кабинетов с интеграцией в `bronivik_jr` для проверки и резервирования оборудования.
- `shared` — общие компоненты доступа, аудита, напоминаний и метрик.
- `docs` — архитектура, схема БД, OpenAPI, инструкции менеджера и rollback-документы.
- `monitoring` — конфигурация Prometheus и Grafana.

## Основные Возможности

- Бронирование оборудования через Telegram с подтверждением менеджером.
- Почасовое бронирование кабинетов с выбором слотов и опциональной привязкой оборудования.
- Интеграция с Google Sheets для выгрузки и синхронизации.
- Напоминания, аудит, экспорт данных и базовые управленческие сценарии.
- REST и gRPC интерфейсы для внешних интеграций.
- Метрики, health-check endpoints и контейнеризированный запуск.

## Архитектура На Высоком Уровне

```text
Medical Booking Service
├── bronivik_jr
│   ├── cmd/bot          Telegram-бот и worker режим
│   ├── cmd/api          HTTP/gRPC API
│   ├── internal/api     API handlers и transport logic
│   ├── internal/bot     Telegram UI, FSM, manager flows
│   ├── internal/database SQLite persistence
│   ├── internal/service Domain services
│   └── configs          Конфигурация и список оборудования
├── bronivik_crm
│   ├── cmd/bot          CRM Telegram-бот
│   ├── internal/booking FSM записи по слотам
│   ├── internal/crmapi  Интеграция с bronivik_jr
│   ├── internal/db      SQLite persistence
│   └── configs          Конфигурация кабинетов и окружения
├── shared               Общие пакеты
├── docs                 Документация
├── monitoring           Prometheus и Grafana
└── docker-compose.yml   Общая оркестрация
```

## Поддерживаемый Runtime

- Go `1.24+` для локальной разработки.
- Docker Engine `20.10+` и Docker Compose `2+` для рекомендованного запуска.
- Redis `7+` для state, rate limiting и cache.
- SQLite как единственный поддерживаемый runtime storage в текущей поставке.

PostgreSQL в текущем состоянии репозитория не является поддерживаемым runtime-сценарием.

## Быстрый Старт

### 1. Клонирование

```bash
git clone https://github.com/gerruda/bronivik.git
cd bronivik
```

### 2. Локальное окружение

```bash
cp .env.example .env
```

Заполните обязательные переменные:

```env
BOT_TOKEN=
CRM_BOT_TOKEN=
CRM_API_KEY=
CRM_API_EXTRA=
LOG_LEVEL=info
```

Если используется Google Sheets, добавьте сервисный JSON в `credentials/google-credentials.json` и укажите spreadsheet IDs в `.env`.

### 3. Запуск через Docker Compose

```bash
docker compose up -d --build
```

Запуск с мониторингом:

```bash
docker compose --profile monitoring up -d --build
```

Остановка:

```bash
docker compose down
```

### 4. Проверка доступности

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
curl http://localhost:8090/healthz
docker compose exec redis redis-cli ping
curl http://localhost:9090/-/healthy
curl http://localhost:3000/api/health
```

## Ручная Проверка End-to-End

Для полного прогона лучше использовать два Telegram-аккаунта: пользователь и менеджер.

### Bronivik Jr

1. Пользователь отправляет `/start` и создаёт заявку.
2. Менеджер открывает список заявок и подтверждает или отклоняет заявку.
3. Менеджер при необходимости меняет аппарат или дату до подтверждения.
4. Пользователь проверяет список своих заявок и уведомления.

### Bronivik CRM

1. Пользователь проходит мастер выбора кабинета, даты, слота, длительности и оборудования.
2. Менеджер просматривает pending-заявки и подтверждает запись.
3. Проверяется интеграция CRM -> Jr по резервированию оборудования.
4. Проверяется отмена записи и обновление доступности слотов.

Подробные операционные инструкции находятся в [docs/MANAGER_GUIDE.md](docs/MANAGER_GUIDE.md) и [docs/TELEGRAM_UI_SPEC.md](docs/TELEGRAM_UI_SPEC.md).

## Команды Разработки

Корневой `Makefile` агрегирует основные проверки:

```bash
make ci
make test-jr
make test-crm
make build-jr
make build-crm
```

Локальный запуск без Docker:

```bash
cd bronivik_jr && go run ./cmd/bot
cd bronivik_jr && go run ./cmd/api
cd bronivik_crm && go run ./cmd/bot
```

## Документация

- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)
- [docs/DATABASE_SCHEMA.md](docs/DATABASE_SCHEMA.md)
- [docs/MANAGER_GUIDE.md](docs/MANAGER_GUIDE.md)
- [docs/ROLLBACK.md](docs/ROLLBACK.md)
- [docs/openapi.yaml](docs/openapi.yaml)

## Безопасность И Публикация

Перед публикацией на GitHub рекомендуется проверить:

- что в истории Git отсутствуют реальные токены, ключи и служебные JSON credentials;
- что `.env`, локальные БД, экспортные артефакты и бинарные файлы не попадают в индекс;
- что GitHub Secrets настроены отдельно от репозитория;
- что описаны правила contribution, security disclosure и кодекс поведения;
- что README отражает реальное состояние поддерживаемых сценариев.

В этом репозитории локальные `.env` и `credentials/` уже игнорируются `.gitignore`, но это не защищает от ранее опубликованной истории. При наличии старых секретов их нужно перевыпустить.

## GitHub Community Files

Для удобной публикации в репозиторий включены:

- `LICENSE`
- `CODE_OF_CONDUCT.md`
- `CONTRIBUTING.md`
- `SECURITY.md`
- `.github/PULL_REQUEST_TEMPLATE.md`
- `.github/ISSUE_TEMPLATE/*`

## Поддержать проект

Если сервис экономит время вам и вашим клиентам — поддержите разработку:

[![Поддержать проект](docs/images/donate_banner.png)](https://dalink.to/bormotoon)

## Лицензия

Проект подготовлен к публикации под лицензией [GNU General Public License v3.0](LICENSE).

Если в истории репозитория были внешние контрибьюторы, фактическое перелицензирование должно учитывать права всех правообладателей.
