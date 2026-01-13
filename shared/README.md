# Shared Modules

Общие модули для обоих ботов Medical Booking Service.

## Структура

```text
shared/
├── access/       # Управление доступом (blocklist, managers)
├── audit/        # Аудит и экспорт данных
├── reminders/    # Система напоминаний
└── utils/        # Общие утилиты
```

## Использование

Каждый модуль может быть импортирован в боты как Go-пакет или скопирован как шаблон.

Подробности см. в [docs/ARCHITECTURE.md](../docs/ARCHITECTURE.md).
