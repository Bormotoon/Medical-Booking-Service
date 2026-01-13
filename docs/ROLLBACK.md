# Rollback Plan for Medical Booking Service

## Overview

This document describes the rollback procedures for critical system changes. Always create a backup before applying migrations or making significant changes.

## Table of Contents

- [General Rollback Procedures](#general-rollback-procedures)
- [Database Migrations](#database-migrations)
- [Configuration Changes](#configuration-changes)
- [Container Deployments](#container-deployments)
- [Emergency Procedures](#emergency-procedures)

---

## General Rollback Procedures

### Pre-Deployment Checklist

1. **Create Database Backup**
   ```bash
   # Bronivik Jr
   cp /app/data/bronivik_jr.db /app/backups/bronivik_jr_$(date +%Y%m%d_%H%M%S).db
   
   # Bronivik CRM
   cp /app/data/bronivik_crm.db /app/backups/bronivik_crm_$(date +%Y%m%d_%H%M%S).db
   ```

2. **Record Current Version**
   ```bash
   # Docker image tags
   docker images --format "{{.Repository}}:{{.Tag}}" | grep bronivik > /app/backups/versions.txt
   
   # Git commit
   git rev-parse HEAD >> /app/backups/versions.txt
   ```

3. **Verify Backup Integrity**
   ```bash
   sqlite3 /app/backups/bronivik_jr_*.db "PRAGMA integrity_check;"
   ```

---

## Database Migrations

### Using golang-migrate

```bash
# Install migrate tool
go install -tags 'sqlite3' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Apply all migrations
migrate -path ./migrations -database "sqlite3:///app/data/bronivik_jr.db" up

# Rollback last migration
migrate -path ./migrations -database "sqlite3:///app/data/bronivik_jr.db" down 1

# Check current version
migrate -path ./migrations -database "sqlite3:///app/data/bronivik_jr.db" version

# Force set version (use with caution)
migrate -path ./migrations -database "sqlite3:///app/data/bronivik_jr.db" force VERSION
```

### Migration-Specific Rollbacks

#### 001_add_end_time (bronivik_jr)

**What it does:**
- Adds `end_time` column to `bookings` table for range booking support

**Rollback command:**
```bash
migrate -path ./bronivik_jr/migrations -database "sqlite3:///app/data/bronivik_jr.db" down 1
```

**Data impact:**
- Existing bookings will lose `end_time` data
- Range bookings will be treated as single-day bookings

**Recovery steps:**
1. Stop the bot service
2. Run rollback migration
3. Restart bot service
4. Verify bookings display correctly

#### 002_create_reminders (bronivik_jr)

**What it does:**
- Creates `reminders` table for scheduling notifications

**Rollback command:**
```bash
migrate -path ./bronivik_jr/migrations -database "sqlite3:///app/data/bronivik_jr.db" down 1
```

**Data impact:**
- All scheduled reminders will be deleted
- Users will not receive reminder notifications

**Recovery steps:**
1. Stop the reminder worker
2. Run rollback migration
3. Disable reminder feature in config
4. Restart services

#### 001_create_reminders (bronivik_crm)

**What it does:**
- Creates `reminders` table for cabinet booking notifications

**Rollback command:**
```bash
migrate -path ./bronivik_crm/migrations -database "sqlite3:///app/data/bronivik_crm.db" down 1
```

---

## Configuration Changes

### Cabinets Configuration (cabinets.yaml)

**Rollback procedure:**
1. Restore previous `cabinets.yaml` from backup
2. Restart bronivik-crm-bot service
3. Verify cabinet list in bot

**Validation:**
```bash
# Test configuration loading
go run ./cmd/bot validate-config --config=configs/cabinets.yaml
```

### Environment Variables

**Critical variables:**
- `REMINDER_ENABLED` - Disable reminders without code changes
- `RATE_LIMIT_PER_SEC` - Adjust rate limiting
- `API_AUTH_ENABLED` - Toggle API authentication

**Quick disable reminders:**
```bash
docker-compose stop bronivik-jr-worker
# or
REMINDER_ENABLED=false docker-compose up -d bronivik-jr-worker
```

---

## Container Deployments

### Rollback to Previous Docker Image

```bash
# List available images
docker images bronivik-jr --format "table {{.Tag}}\t{{.CreatedAt}}"

# Stop current containers
docker-compose down

# Update docker-compose.yml to use previous version
# VERSION=previous-tag docker-compose up -d
VERSION=v1.0.0 docker-compose up -d
```

### Full Stack Rollback

```bash
# 1. Stop all services
docker-compose down

# 2. Restore databases from backup
cp /app/backups/bronivik_jr_YYYYMMDD_HHMMSS.db /app/data/bronivik_jr.db
cp /app/backups/bronivik_crm_YYYYMMDD_HHMMSS.db /app/data/bronivik_crm.db

# 3. Checkout previous code version
git checkout <previous-commit>

# 4. Rebuild and start
docker-compose build
docker-compose up -d

# 5. Verify health
docker-compose ps
curl http://localhost:8080/healthz
```

### Worker-Only Rollback

```bash
# Stop only the worker
docker-compose stop bronivik-jr-worker

# Start bot without worker (reminders disabled)
REMINDER_ENABLED=false docker-compose up -d bronivik-jr-bot
```

---

## Emergency Procedures

### Service Completely Down

1. **Check container status:**
   ```bash
   docker-compose ps
   docker-compose logs --tail=100 bronivik-jr-bot
   ```

2. **Check resource usage:**
   ```bash
   docker stats --no-stream
   ```

3. **Restart services:**
   ```bash
   docker-compose restart bronivik-jr-bot bronivik-jr-api
   ```

4. **If restart fails, rebuild:**
   ```bash
   docker-compose down
   docker-compose build --no-cache
   docker-compose up -d
   ```

### Database Corruption

1. **Stop all services:**
   ```bash
   docker-compose down
   ```

2. **Check database integrity:**
   ```bash
   sqlite3 /app/data/bronivik_jr.db "PRAGMA integrity_check;"
   ```

3. **If corrupted, restore from backup:**
   ```bash
   cp /app/backups/bronivik_jr_latest.db /app/data/bronivik_jr.db
   ```

4. **Restart services:**
   ```bash
   docker-compose up -d
   ```

### Telegram Rate Limited

1. **Check rate limit errors in logs:**
   ```bash
   docker-compose logs --tail=200 bronivik-jr-bot | grep -i "rate"
   ```

2. **Reduce rate limit:**
   ```bash
   RATE_LIMIT_PER_SEC=10 docker-compose up -d bronivik-jr-worker
   ```

3. **Wait for rate limit to expire (typically 1-60 minutes)**

4. **Gradually increase rate limit back to normal**

---

## Contact Information

- **On-Call Engineer:** Check PagerDuty/OpsGenie rotation
- **Slack Channel:** #bronivik-incidents
- **Runbook Location:** https://wiki.example.com/bronivik/runbooks

---

## Change Log

| Date | Version | Change | Author |
|------|---------|--------|--------|
| 2026-01-13 | 1.0.0 | Initial rollback documentation | System |
