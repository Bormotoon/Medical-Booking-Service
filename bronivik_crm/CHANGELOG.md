# Changelog

All notable changes to this project will be documented in this file.

## [1.0.0] - 2026-01-09

### Added
- Исходный код проекта bronivik_crm, выделенный из основного репозитория bronivik_jr
- Полная документация (README.md, CONTRIBUTING.md, MIGRATION.md)
- Docker и docker-compose конфигурации
- Makefile для удобной разработки
- .golangci.yml для линтинга кода
- .env.example — шаблон переменных окружения
- MIT License

### Fixed
- Исправлены опечатки в импортах: `tgbotcrmapi` → `tgbotapi`
- Исправлены опечатки в вызовах API: `b.crmapi` → `b.api`
- Обновлен Dockerfile для корректной сборки с SQLite (CGO_ENABLED=1)
- Добавлены недостающие зависимости в go.mod (stretchr/testify)

### Changed
- Обновлен go.mod для Go 1.24
- Переработан README.md с подробными инструкциями
- Улучшена структура проекта

### Removed
- Удалены компоненты, специфичные для bronivik_jr (API сервер, Google Sheets синхронизация, event bus и т.д.)
