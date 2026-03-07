# Medical Booking Service

> 🇷🇺 [Русская версия](README.md)

A comprehensive medical equipment and cabinet booking system based on Telegram bots.

[![License: MPL 2.0](https://img.shields.io/badge/License-MPL%202.0-brightgreen.svg)](https://opensource.org/licenses/MPL-2.0)
[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://golang.org/)
[![Docker](https://img.shields.io/badge/Docker-ready-2496ED?logo=docker)](https://www.docker.com/)

---

## 📋 Table of Contents

- [Overview](#-overview)
- [Architecture](#️-architecture)
- [Key Features](#-key-features)
- [Requirements](#-requirements)
- [Quick Start](#-quick-start)
- [Configuration](#️-configuration)
- [Bot Commands](#-bot-commands)
- [API](#-api)
- [Monitoring](#-monitoring)
- [Development](#️-development)
- [Updating Subprojects](#-updating-subprojects-git-subtree)
- [License](#-license)

---

## 🎯 Overview

Medical Booking Service is a monorepo that combines two interconnected Telegram bots for automating medical resource booking processes:

1. **Bronivik Jr** (`bronivik_jr/`) — the main service for booking medical equipment for full days
2. **Bronivik CRM** (`bronivik_crm/`) — a specialized bot for hourly booking of physical cabinets with automatic equipment availability verification

The system provides:
- 🤖 Convenient interface via Telegram
- 📊 Integration with Google Sheets for reporting
- 🔄 Automatic data synchronization between bots
- 🔔 Reminder system for upcoming bookings
- 📈 Monitoring and metrics (Prometheus + Grafana)
- 🔒 Access control and manager permissions

---

## 🏗️ Architecture

```
Medical-Booking-Service/
├── bronivik_jr/          # Bot 1: Equipment Booking
│   ├── cmd/
│   │   ├── bot/          # Telegram Bot
│   │   └── api/          # REST API & gRPC Server
│   ├── internal/
│   │   ├── api/          # HTTP & gRPC API
│   │   ├── bot/          # Bot business logic
│   │   ├── database/     # SQLite (WAL)
│   │   ├── google/       # Google Sheets Worker
│   │   └── worker/       # Background Jobs
│   └── configs/          # Configuration (config.yaml, items.yaml)
│
├── bronivik_crm/         # Bot 2: Cabinet Booking
│   ├── cmd/
│   │   └── bot/          # Telegram Bot
│   ├── internal/
│   │   ├── bot/          # Bot logic
│   │   ├── booking/      # Booking FSM
│   │   ├── crmapi/       # HTTP client for Bronivik Jr API
│   │   ├── db/           # SQLite database
│   │   └── manager/      # Manager panel
│   └── configs/          # Configuration (config.yaml, cabinets.yaml)
│
├── shared/               # Shared modules
│   ├── access/          # Access control (blocklist, managers)
│   ├── audit/           # Audit and data export
│   ├── reminders/       # Reminder system
│   └── utils/           # Common utilities
│
├── monitoring/          # Monitoring configuration
│   ├── prometheus.yml   # Prometheus config
│   ├── alerts.yml       # Alert rules
│   └── grafana/         # Dashboards and datasources
│
├── docs/                # Documentation
│   ├── ARCHITECTURE.md  # System architecture
│   ├── DATABASE_SCHEMA.md # Database schema
│   ├── MANAGER_GUIDE.md # Manager guide
│   ├── ROLLBACK.md      # Rollback plan
│   └── openapi.yaml     # OpenAPI specification
│
├── scripts/             # Utilities and scripts
│   └── migrate.sh       # Database migration system
│
├── docker-compose.yml   # Service orchestration
└── .env.example         # Environment variables template
```

### Bronivik Jr Components (Main Service)

- **Telegram Bot** — interface for users and managers
- **REST API & gRPC** — integration points for external services (including CRM bot)
- **SQLite (WAL)** — main data storage (bookings, users, equipment)
- **Google Sheets Worker** — asynchronous synchronization with Google Sheets
- **Event Bus** — internal event bus for logic separation
- **Redis** — caching and rate limiting
- **Reminder Worker** — background reminder sending (cron job)

### Bronivik CRM Components (Cabinets)

- **Telegram Bot** — interface for hourly booking
- **FSM Engine** — finite state machine for booking dialogs
- **API Client** — integration with Bronivik Jr for equipment verification
- **SQLite** — local database for schedules and bookings
- **Redis** — API request caching

---

## ✨ Key Features

### Common for Both Systems

- ✅ **Automatic reminders** — 24 hours before booking with customizable notifications
- ✅ **Monthly audit** — export all data to Excel (on the 1st of each month)
- ✅ **Access control** — user blocklist and manager list
- ✅ **TTL policy** — automatic deletion of data older than 31 days
- ✅ **Metrics and monitoring** — Prometheus + Grafana for observability
- ✅ **Health checks** — availability checks for all components

### Bronivik Jr (Equipment)

- 📱 Booking medical equipment for full days
- 🔄 Range bookings ("permanent rental" for CRM)
- 📊 Synchronization with Google Sheets
- 🔌 REST API and gRPC for integration
- ✅ Manager approval for requests
- 📈 Statistics and reports
- 🔍 Availability check via API

### Bronivik CRM (Cabinets)

- 🏥 Hourly booking of physical cabinets
- ⏰ 30-minute slots with option to select multiple consecutive slots
- 🔗 Automatic equipment availability check via Bronivik Jr API
- 📅 Flexible schedule management (working hours, lunch breaks, holidays)
- 👥 Client data collection (full name, phone)
- 🎛️ Extended manager panel for request management
- 📋 Manual request creation by manager (phone bookings)

---

## 📦 Requirements

- **Go** 1.24+ (for local development)
- **Docker** 20.10+ and **Docker Compose** 2.0+
- **Redis** 7+ (automatically started in docker-compose)
- **SQLite3** (embedded in Docker images)
- **Google Cloud Service Account** (optional, for Google Sheets sync)

PostgreSQL is not part of the supported runtime scenarios; both `bronivik_jr` and `bronivik_crm` run on SQLite only.

---

## 🚀 Quick Start

### 1. Clone the Repository

```bash
git clone https://github.com/Bormotoon/Medical-Booking-Service.git
cd Medical-Booking-Service
```

### 2. Configure Environment Variables

```bash
cp .env.example .env
```

Edit `.env` and fill in the required variables:

```env
# Telegram tokens (get from @BotFather)
BOT_TOKEN=your_bot_token_here
CRM_BOT_TOKEN=your_crm_bot_token_here

# API authorization
API_AUTH_KEYS=key1:extra1,key2:extra2
CRM_API_KEY=key1
CRM_API_EXTRA=extra1

# Managers (Telegram User IDs, comma-separated)
MANAGERS=123456789,987654321

# Google Sheets (optional)
GOOGLE_SPREADSHEET_ID=your_spreadsheet_id
CRM_GOOGLE_SPREADSHEET_ID=your_crm_spreadsheet_id
```

### 3. (Optional) Configure Google Sheets

If Google Sheets synchronization is required:

1. Create a Service Account in [Google Cloud Console](https://console.cloud.google.com/)
2. Download the JSON key
3. Place the file in `credentials/google-credentials.json`
4. Grant the Service Account access to your spreadsheet

### 4. Start Services

```bash
# Start all services
docker compose up -d --build

# Start with monitoring (Prometheus + Grafana)
docker compose --profile monitoring up -d --build

# View logs
docker compose logs -f

# Stop services
docker compose down
```

### 5. Verify Health

```bash
# Bronivik Jr API
curl http://localhost:8080/healthz

# Bronivik CRM
curl http://localhost:8090/healthz

# Redis
docker compose exec redis redis-cli ping

# Prometheus (if started)
curl http://localhost:9090/-/healthy

# Grafana (if started)
open http://localhost:3000  # admin/admin
```

---

## ⚙️ Configuration

### Main Service (Bronivik Jr)

File: `bronivik_jr/configs/config.yaml`

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

### Equipment List

File: `bronivik_jr/configs/items.yaml`

```yaml
items:
  - id: 1
    name: "Ultrasound Philips"
    category: "Ultrasound"
    quantity: 2
    order: 1
  - id: 2
    name: "X-ray GE"
    category: "X-ray"
    quantity: 1
    order: 2
```

### CRM Bot

File: `bronivik_crm/configs/config.yaml`

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
  max_active_per_user: 0  # 0 = no limit

managers:
  - 123456789

monitoring:
  prometheus_enabled: true
  health_check_port: 8090
```

### Cabinet Configuration

File: `bronivik_crm/configs/cabinets.yaml`

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
    name: "Cabinet #1"
    number: "101"
    floor: 1
    capacity: 2
    enabled: true
    
  - id: 2
    name: "Cabinet #2"
    number: "102"
    floor: 1
    capacity: 3
    enabled: true

holidays:
  - date: "2026-01-01"
    name: "New Year"
  - date: "2026-05-01"
    name: "Labor Day"
```

---

## 💬 Bot Commands

### Bronivik Jr (Main Bot)

#### User Commands

- `/start` — start, registration
- `/book` — start equipment booking wizard
- `/my_bookings` — list of my active bookings
- `/cancel_booking <ID>` — cancel booking by ID
- `/help` — command help

#### Manager Commands

- `/approve <ID>` — approve booking
- `/reject <ID>` — reject booking
- `/stats [period]` — statistics for period
- `/export_bookings` — manual sync with Google Sheets
- `/pending` — list of pending requests

### Bronivik CRM (Cabinet Bot)

#### User Commands

- `/start` — start
- `/book` — start cabinet booking process
  - Select cabinet
  - Select date (interactive calendar)
  - Select time slot
  - Select duration (number of 30-minute slots)
  - Select equipment (or "No equipment")
  - Enter client data (full name, phone)
- `/my_bookings` — my cabinet bookings
- `/cancel_booking <ID>` — cancel booking

#### Manager Commands

- `/pending` — list of pending requests
- `/today_schedule` — today's cabinet schedule
- `/tomorrow_schedule` — tomorrow's schedule
- `/add_cabinet <name>` — add new cabinet
- `/list_cabinets` — view all cabinets
- `/set_schedule <cab_id> <day> <start> <end>` — configure schedule

---

## 🔌 API

### REST API (Bronivik Jr)

Port: `8080` (HTTP), `8081` (gRPC)

#### Main Endpoints

```bash
# List all equipment
GET /api/v1/items

# Check equipment availability
GET /api/v1/availability/{item_name}?date=YYYY-MM-DD

# Bulk availability check
POST /api/v1/availability/bulk
Content-Type: application/json
{
  "items": ["Ultrasound Philips", "X-ray GE"],
  "start_date": "2026-01-20",
  "end_date": "2026-01-25"
}

# List devices for CRM
GET /api/devices?date=YYYY-MM-DD&include_reserved=true

# Book device (for CRM)
POST /api/book-device
Content-Type: application/json
x-api-key: your_api_key
x-api-extra: your_api_extra
{
  "device_id": 1,
  "date": "2026-01-20",
  "external_booking_id": "crm-12345",
  "client_name": "John Doe",
  "client_phone": "+1234567890"
}

# Cancel external booking
DELETE /api/book-device/{external_id}

# Health Check
GET /healthz
GET /readyz
```

#### Authentication

API uses header-based authentication:

```bash
curl -H "x-api-key: your_key" \
     -H "x-api-extra: your_extra" \
     http://localhost:8080/api/v1/items
```

Full OpenAPI specification: [`docs/openapi.yaml`](docs/openapi.yaml)

---

## 📊 Monitoring

### Health Checks

- **Bronivik Jr API**: `http://localhost:8080/healthz`
- **Bronivik CRM**: `http://localhost:8090/healthz`
- **Redis**: `docker compose exec redis redis-cli ping`

### Prometheus Metrics

Port: `9090` (when started with `monitoring` profile)

```bash
# View metrics
open http://localhost:9090/metrics
open http://localhost:9090/graph
```

#### Key Metrics

- `reminders_sent_total` — reminders sent
- `reminders_queue_size` — reminder queue size
- `reminder_send_duration_seconds` — reminder sending time
- `api_requests_total` — total API requests
- `api_request_duration_seconds` — API request duration
- `booking_operations_total` — booking operations

### Grafana Dashboards

Port: `3000` (when started with `monitoring` profile)

- URL: `http://localhost:3000`
- Login: `admin`
- Password: `admin` (change on first login)

Pre-configured dashboards in `monitoring/grafana/provisioning/dashboards/`

### Alerting

Alert configuration: `monitoring/alerts.yml`

Alert groups:
- **reminders** — reminder issues
- **api** — API issues
- **bots** — bot issues
- **database** — database issues
- **redis** — Redis issues
- **resources** — resource usage

---

## 🛠️ Development

### Local Run (without Docker)

#### Requirements

```bash
# Install Go 1.24+
go version

# Install dependencies for both modules
cd bronivik_jr && go mod download && cd ..
cd bronivik_crm && go mod download && cd ..

# Install linter (optional)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

#### Start Redis

```bash
docker compose up -d redis
```

#### Run Bronivik Jr

```bash
# Terminal 1: Bot
cd bronivik_jr
go run ./cmd/bot --config=configs/config.yaml

# Terminal 2: API Server
cd bronivik_jr
go run ./cmd/api --config=configs/config.yaml

# Terminal 3: Worker (reminders)
cd bronivik_jr
go run ./cmd/bot worker --job=reminders
```

#### Run Bronivik CRM

```bash
cd bronivik_crm
go run ./cmd/bot --config=configs/config.yaml
```

### Testing

```bash
# Run all tests for both modules
make test

# Run tests with coverage
make test-coverage

# Tests for specific module
cd bronivik_jr && go test ./... -v
cd bronivik_crm && go test ./... -v

# Integration tests
go test ./internal/api/... -tags=integration -v
```

### Linting

```bash
# Run linter
make lint

# Or directly
cd bronivik_jr && golangci-lint run
cd bronivik_crm && golangci-lint run
```

### Database Migrations

```bash
# Apply all migrations
./scripts/migrate.sh up all

# Rollback one migration
./scripts/migrate.sh down bronivik_jr 1

# Check current version
./scripts/migrate.sh version all

# Create backup before migration
./scripts/migrate.sh backup all
```

Details: [`docs/ROLLBACK.md`](docs/ROLLBACK.md)

### Project Structure

Detailed architecture description: [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md)

---

## 🔄 Updating Subprojects (git subtree)

The project uses git subtree for subproject management.

### Initial Setup

If remotes are not yet added:

```bash
git remote add bronivik_jr https://github.com/Bormotoon/bronivik_jr.git
git remote add bronivik_crm https://github.com/Bormotoon/bronivik_crm.git
```

### Pull Changes from Subprojects

```bash
# Update bronivik_jr (master branch)
git fetch bronivik_jr master
git subtree pull --prefix=bronivik_jr bronivik_jr master

# Update bronivik_crm (main branch)
git fetch bronivik_crm main
git subtree pull --prefix=bronivik_crm bronivik_crm main
```

### Push Changes Back to Subprojects

```bash
# Push changes to bronivik_jr
git subtree push --prefix=bronivik_jr bronivik_jr master

# Push changes to bronivik_crm
git subtree push --prefix=bronivik_crm bronivik_crm main
```

### Notes

- Use `--squash` with `git subtree pull` for a single merge commit
- Ensure you run commands from the repository root
- Changes in `bronivik_jr/` and `bronivik_crm/` subdirectories are automatically included in monorepo history

---

## 📚 Additional Documentation

- [📖 System Architecture](docs/ARCHITECTURE.md)
- [💾 Database Schema](docs/DATABASE_SCHEMA.md)
- [👨‍💼 Manager Guide](docs/MANAGER_GUIDE.md)
- [🔙 Rollback Plan](docs/ROLLBACK.md)
- [🔌 OpenAPI Specification](docs/openapi.yaml)

---

## 📄 License

This project is licensed under the [Mozilla Public License 2.0](LICENSE).

---

## 👥 Contacts and Support

- **GitHub**: [@Bormotoon](https://github.com/Bormotoon)
- **Main Project (Jr)**: [bronivik_jr](https://github.com/Bormotoon/bronivik_jr)
- **CRM Project**: [bronivik_crm](https://github.com/Bormotoon/bronivik_crm)

---

## 🙏 Acknowledgments

The project uses the following open-source libraries:
- [Telegram Bot API](https://github.com/go-telegram-bot-api/telegram-bot-api)
- [Echo Framework](https://echo.labstack.com/)
- [SQLite](https://www.sqlite.org/)
- [Redis](https://redis.io/)
- [Prometheus](https://prometheus.io/)
- [Grafana](https://grafana.com/)

---

**Made with ❤️ for medical professionals**
