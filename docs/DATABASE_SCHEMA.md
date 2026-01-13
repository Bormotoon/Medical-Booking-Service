# Схема базы данных Medical Booking Service

## Общие принципы

- Обе системы используют **SQLite** в режиме WAL
- Все таблицы имеют поля `created_at` и `updated_at`
- Внешние ключи включены (`PRAGMA foreign_keys = ON`)
- Мягкое удаление через флаги (is_active, is_blacklisted)

---

## Бот 1: bronivik_jr (Аппараты)

### Таблица `users`

Пользователи системы.

```sql
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    telegram_id INTEGER UNIQUE NOT NULL,
    username TEXT,
    first_name TEXT NOT NULL,
    last_name TEXT,
    phone TEXT,
    is_manager BOOLEAN NOT NULL DEFAULT 0,
    is_blacklisted BOOLEAN NOT NULL DEFAULT 0,
    language_code TEXT,
    last_activity DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_users_telegram_id ON users(telegram_id);
CREATE INDEX idx_users_is_manager ON users(is_manager);
CREATE INDEX idx_users_is_blacklisted ON users(is_blacklisted);
```

### Таблица `user_settings`

Настройки пользователя (напоминания и др.).

```sql
CREATE TABLE user_settings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL UNIQUE,
    reminders_enabled BOOLEAN NOT NULL DEFAULT 1,
    reminder_hours_before INTEGER NOT NULL DEFAULT 24,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_user_settings_user_id ON user_settings(user_id);
```

### Таблица `items`

Медицинские аппараты/устройства.

```sql
CREATE TABLE items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    description TEXT,
    total_quantity INTEGER NOT NULL DEFAULT 1,
    sort_order INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT 1,
    permanent_reserved BOOLEAN NOT NULL DEFAULT 0,  -- для "вечной аренды"
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_items_sort ON items(sort_order, id);
CREATE INDEX idx_items_active ON items(is_active);
CREATE INDEX idx_items_permanent ON items(permanent_reserved);
```

### Таблица `bookings`

Бронирования аппаратов.

```sql
CREATE TABLE bookings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    user_name TEXT NOT NULL,
    user_nickname TEXT,
    phone TEXT NOT NULL,
    item_id INTEGER NOT NULL,
    item_name TEXT NOT NULL,
    date DATETIME NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    comment TEXT,
    reminder_sent BOOLEAN NOT NULL DEFAULT 0,
    external_booking_id TEXT,  -- ID из CRM бота
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    version INTEGER NOT NULL DEFAULT 1,
    FOREIGN KEY (item_id) REFERENCES items(id),
    FOREIGN KEY (user_id) REFERENCES users(telegram_id)
);

CREATE INDEX idx_bookings_date ON bookings(date);
CREATE INDEX idx_bookings_status ON bookings(status);
CREATE INDEX idx_bookings_item_id ON bookings(item_id);
CREATE INDEX idx_bookings_user_id ON bookings(user_id);
CREATE INDEX idx_bookings_item_date_status ON bookings(item_id, date, status);
CREATE INDEX idx_bookings_reminder ON bookings(reminder_sent, date);
CREATE INDEX idx_bookings_external ON bookings(external_booking_id);
```

### Таблица `sync_queue`

Очередь синхронизации с Google Sheets.

```sql
CREATE TABLE sync_queue (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_type TEXT NOT NULL,
    booking_id INTEGER NOT NULL,
    payload TEXT,
    status TEXT DEFAULT 'pending',
    retry_count INTEGER DEFAULT 0,
    last_error TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    processed_at DATETIME,
    next_retry_at DATETIME
);

CREATE INDEX idx_sync_queue_status ON sync_queue(status);
CREATE INDEX idx_sync_queue_next_retry ON sync_queue(next_retry_at);
```

---

## Бот 2: bronivik_crm (Кабинеты)

### Таблица `users`

```sql
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    telegram_id INTEGER UNIQUE NOT NULL,
    username TEXT,
    first_name TEXT,
    last_name TEXT,
    phone TEXT,
    is_manager BOOLEAN NOT NULL DEFAULT 0,
    is_blacklisted BOOLEAN NOT NULL DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_users_telegram_id ON users(telegram_id);
CREATE INDEX idx_users_is_manager ON users(is_manager);
```

### Таблица `user_settings`

```sql
CREATE TABLE user_settings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL UNIQUE,
    reminders_enabled BOOLEAN NOT NULL DEFAULT 1,
    reminder_hours_before INTEGER NOT NULL DEFAULT 24,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_user_settings_user_id ON user_settings(user_id);
```

### Таблица `cabinets`

Кабинеты для бронирования.

```sql
CREATE TABLE cabinets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    description TEXT,
    is_active BOOLEAN DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_cabinets_active ON cabinets(is_active);
```

### Таблица `cabinet_schedules`

Стандартное расписание кабинетов по дням недели.

```sql
CREATE TABLE cabinet_schedules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    cabinet_id INTEGER NOT NULL,
    day_of_week INTEGER NOT NULL,          -- 1=Пн, 7=Вс
    start_time TEXT NOT NULL,              -- "10:00"
    end_time TEXT NOT NULL,                -- "22:00"
    lunch_start TEXT,                      -- "13:00" (опционально)
    lunch_end TEXT,                        -- "14:00" (опционально)
    slot_duration INTEGER DEFAULT 30,      -- минуты
    is_active BOOLEAN DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (cabinet_id) REFERENCES cabinets(id)
);

CREATE INDEX idx_schedules_cabinet ON cabinet_schedules(cabinet_id, day_of_week);
```

### Таблица `cabinet_schedule_overrides`

Переопределение расписания на конкретные даты.

```sql
CREATE TABLE cabinet_schedule_overrides (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    cabinet_id INTEGER NOT NULL,
    date DATE NOT NULL,
    is_closed BOOLEAN DEFAULT 0,           -- полный выходной
    start_time TEXT,                       -- переопределенное время начала
    end_time TEXT,                         -- переопределенное время окончания
    lunch_start TEXT,
    lunch_end TEXT,
    reason TEXT,                           -- причина изменения
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (cabinet_id) REFERENCES cabinets(id),
    UNIQUE(cabinet_id, date)
);

CREATE INDEX idx_overrides_cabinet_date ON cabinet_schedule_overrides(cabinet_id, date);
```

### Таблица `hourly_bookings`

Почасовые бронирования кабинетов.

```sql
CREATE TABLE hourly_bookings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    cabinet_id INTEGER NOT NULL,
    item_id INTEGER,                       -- ID аппарата из bronivik_jr
    item_name TEXT,                        -- название аппарата
    client_name TEXT NOT NULL,             -- ФИО клиента
    client_phone TEXT,
    start_time DATETIME NOT NULL,
    end_time DATETIME NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    comment TEXT,
    manager_comment TEXT,                  -- комментарий менеджера
    reminder_sent BOOLEAN NOT NULL DEFAULT 0,
    external_device_booking_id INTEGER,    -- ID брони аппарата в bronivik_jr
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (cabinet_id) REFERENCES cabinets(id),
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE INDEX idx_hourly_bookings_times ON hourly_bookings(cabinet_id, start_time, end_time);
CREATE INDEX idx_hourly_bookings_status ON hourly_bookings(status);
CREATE INDEX idx_hourly_bookings_user ON hourly_bookings(user_id);
CREATE INDEX idx_hourly_bookings_reminder ON hourly_bookings(reminder_sent, start_time);
CREATE INDEX idx_hourly_bookings_date ON hourly_bookings(date(start_time));
```

---

## Статусы заявок

| Статус | Описание | Бот 1 | Бот 2 |
|--------|----------|-------|-------|
| `pending` | Ожидает подтверждения | ✓ | ✓ |
| `confirmed` / `approved` | Подтверждена | ✓ | ✓ |
| `rejected` | Отклонена | ✓ | ✓ |
| `canceled` | Отменена пользователем | ✓ | ✓ |
| `needs_revision` | На доработке | — | ✓ |
| `completed` | Завершена | ✓ | ✓ |
| `changed` | Изменена | ✓ | — |

---

## Миграции

### Добавление user_settings (если таблица не существует)

```sql
-- Для обоих ботов
CREATE TABLE IF NOT EXISTS user_settings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL UNIQUE,
    reminders_enabled BOOLEAN NOT NULL DEFAULT 1,
    reminder_hours_before INTEGER NOT NULL DEFAULT 24,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
```

### Добавление reminder_sent в bookings

```sql
-- bronivik_jr
ALTER TABLE bookings ADD COLUMN reminder_sent BOOLEAN NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_bookings_reminder ON bookings(reminder_sent, date);

-- bronivik_crm
ALTER TABLE hourly_bookings ADD COLUMN reminder_sent BOOLEAN NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_hourly_bookings_reminder ON hourly_bookings(reminder_sent, start_time);
```

### Добавление permanent_reserved в items

```sql
-- bronivik_jr
ALTER TABLE items ADD COLUMN permanent_reserved BOOLEAN NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_items_permanent ON items(permanent_reserved);
```

### Добавление полей обеда в расписание

```sql
-- bronivik_crm
ALTER TABLE cabinet_schedules ADD COLUMN lunch_start TEXT;
ALTER TABLE cabinet_schedules ADD COLUMN lunch_end TEXT;
ALTER TABLE cabinet_schedule_overrides ADD COLUMN lunch_start TEXT;
ALTER TABLE cabinet_schedule_overrides ADD COLUMN lunch_end TEXT;
```

---

## Политика хранения данных

- Заявки старше **31 дня** автоматически удаляются
- Перед удалением данные экспортируются в XLS

```sql
-- Удаление старых заявок (bronivik_jr)
DELETE FROM bookings WHERE created_at < datetime('now', '-31 days');

-- Удаление старых заявок (bronivik_crm)
DELETE FROM hourly_bookings WHERE created_at < datetime('now', '-31 days');
```

---

*Документ создан: 13 января 2026*
