# Medical Booking Service

> Russian version: [README.md](README.md)

[![CI](https://github.com/gerruda/bronivik/actions/workflows/ci.yml/badge.svg)](https://github.com/gerruda/bronivik/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://go.dev/)
[![Docker](https://img.shields.io/badge/Docker-ready-2496ED?logo=docker)](https://www.docker.com/)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)

Medical Booking Service is a monorepo for Telegram-based medical resource booking. It combines two domain services, shared libraries, monitoring assets, and operational documentation.

## Repository Contents

- `bronivik_jr` — primary equipment booking service with a Telegram bot, HTTP/gRPC API, and background workers.
- `bronivik_crm` — hourly cabinet booking service integrated with `bronivik_jr` for device availability and reservation flows.
- `shared` — shared access, audit, reminder, and metric packages.
- `docs` — architecture, database schema, OpenAPI, manager guide, and rollback documentation.
- `monitoring` — Prometheus and Grafana configuration.

## Key Capabilities

- Telegram-based equipment booking with manager approval.
- Hourly cabinet booking with slot selection and optional device binding.
- Google Sheets synchronization and export workflows.
- Reminders, audit exports, and manager-side operational tools.
- REST and gRPC integration points.
- Metrics, health endpoints, and Docker-first local deployment.

## High-Level Architecture

```text
Medical Booking Service
├── bronivik_jr
│   ├── cmd/bot          Telegram bot and worker mode
│   ├── cmd/api          HTTP/gRPC API
│   ├── internal/api     API handlers and transport
│   ├── internal/bot     Telegram UI, FSM, manager flows
│   ├── internal/database SQLite persistence
│   ├── internal/service Domain services
│   └── configs          Service and item configuration
├── bronivik_crm
│   ├── cmd/bot          CRM Telegram bot
│   ├── internal/booking Slot-booking FSM
│   ├── internal/crmapi  Integration client for bronivik_jr
│   ├── internal/db      SQLite persistence
│   └── configs          Cabinet and environment configuration
├── shared               Shared packages
├── docs                 Project documentation
├── monitoring           Prometheus and Grafana assets
└── docker-compose.yml   Top-level orchestration
```

## Supported Runtime

- Go `1.24+` for local development.
- Docker Engine `20.10+` and Docker Compose `2+` for the recommended setup.
- Redis `7+` for cache, state, and rate-limiting support.
- SQLite as the only supported runtime database in the current repository state.

PostgreSQL is not a supported runtime target in the current codebase.

## Quick Start

### 1. Clone

```bash
git clone https://github.com/gerruda/bronivik.git
cd bronivik
```

### 2. Configure local environment

```bash
cp .env.example .env
```

Fill in at least the required values:

```env
BOT_TOKEN=
CRM_BOT_TOKEN=
CRM_API_KEY=
CRM_API_EXTRA=
LOG_LEVEL=info
```

If Google Sheets support is needed, place a service account JSON file in `credentials/google-credentials.json` and set spreadsheet IDs in `.env`.

### 3. Start with Docker Compose

```bash
docker compose up -d --build
```

Start with monitoring enabled:

```bash
docker compose --profile monitoring up -d --build
```

Stop the stack:

```bash
docker compose down
```

### 4. Verify health

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
curl http://localhost:8090/healthz
docker compose exec redis redis-cli ping
curl http://localhost:9090/-/healthy
curl http://localhost:3000/api/health
```

## End-to-End Manual Validation

Use two Telegram accounts for realistic manual testing: one regular user and one manager.

### Bronivik Jr

1. A user sends `/start` and creates a booking request.
2. A manager opens the request queue and approves or rejects the booking.
3. The manager optionally changes the device or booking date before approval.
4. The user verifies notifications and booking visibility.

### Bronivik CRM

1. A user completes the cabinet booking wizard.
2. A manager reviews pending requests and confirms the booking.
3. Device reservation is verified through the CRM -> Jr integration.
4. Cancellation and slot release are verified.

Detailed operational guidance is available in [docs/MANAGER_GUIDE.md](docs/MANAGER_GUIDE.md) and [docs/TELEGRAM_UI_SPEC.md](docs/TELEGRAM_UI_SPEC.md).

## Development Commands

The top-level `Makefile` provides aggregated checks:

```bash
make ci
make test-jr
make test-crm
make build-jr
make build-crm
```

Run services locally without Docker:

```bash
cd bronivik_jr && go run ./cmd/bot
cd bronivik_jr && go run ./cmd/api
cd bronivik_crm && go run ./cmd/bot
```

## Documentation

- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)
- [docs/DATABASE_SCHEMA.md](docs/DATABASE_SCHEMA.md)
- [docs/MANAGER_GUIDE.md](docs/MANAGER_GUIDE.md)
- [docs/ROLLBACK.md](docs/ROLLBACK.md)
- [docs/openapi.yaml](docs/openapi.yaml)

## Security And Publication Notes

Before publishing the repository to GitHub, verify that:

- no real tokens, keys, or service-account JSON files were ever committed into Git history;
- `.env`, local databases, export artifacts, and binaries stay outside Git tracking;
- GitHub Secrets are configured separately from the repository contents;
- contribution, security, and conduct policies are present and current;
- the README reflects the actual supported runtime scenarios.

Local `.env` files and `credentials/` are already ignored by `.gitignore`, but that does not protect previously published history. Any exposed credentials must be rotated.

## GitHub Community Files

This repository now includes:

- `LICENSE`
- `CODE_OF_CONDUCT.md`
- `CONTRIBUTING.md`
- `SECURITY.md`
- `.github/PULL_REQUEST_TEMPLATE.md`
- `.github/ISSUE_TEMPLATE/*`

## Support the Project

If this service saves time for you and your clients — support development:

[![Support the project](docs/images/donate_banner.png)](https://dalink.to/bormotoon)

## License

This repository is prepared for publication under the [GNU General Public License v3.0](LICENSE).

If the repository history includes external contributors, actual relicensing must take contributor copyright ownership into account.
