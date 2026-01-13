#!/bin/bash
# Migration script for Medical Booking Service
# Usage: ./scripts/migrate.sh [command] [service] [args]

set -e

# Configuration
BRONIVIK_JR_DB="${BRONIVIK_JR_DB:-/app/data/bronivik_jr.db}"
BRONIVIK_CRM_DB="${BRONIVIK_CRM_DB:-/app/data/bronivik_crm.db}"
BACKUP_DIR="${BACKUP_DIR:-/app/backups}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if migrate tool is installed
check_migrate() {
    if ! command -v migrate &> /dev/null; then
        log_error "migrate tool is not installed"
        log_info "Install with: go install -tags 'sqlite3' github.com/golang-migrate/migrate/v4/cmd/migrate@latest"
        exit 1
    fi
}

# Get database path for service
get_db_path() {
    case "$1" in
        jr|bronivik-jr)
            echo "$BRONIVIK_JR_DB"
            ;;
        crm|bronivik-crm)
            echo "$BRONIVIK_CRM_DB"
            ;;
        *)
            log_error "Unknown service: $1. Use 'jr' or 'crm'"
            exit 1
            ;;
    esac
}

# Get migrations path for service
get_migrations_path() {
    case "$1" in
        jr|bronivik-jr)
            echo "./bronivik_jr/migrations"
            ;;
        crm|bronivik-crm)
            echo "./bronivik_crm/migrations"
            ;;
        *)
            log_error "Unknown service: $1. Use 'jr' or 'crm'"
            exit 1
            ;;
    esac
}

# Create backup
backup() {
    local service="$1"
    local db_path=$(get_db_path "$service")
    local timestamp=$(date +%Y%m%d_%H%M%S)
    local backup_file="${BACKUP_DIR}/${service}_${timestamp}.db"
    
    mkdir -p "$BACKUP_DIR"
    
    if [ -f "$db_path" ]; then
        log_info "Creating backup: $backup_file"
        cp "$db_path" "$backup_file"
        log_info "Backup created successfully"
    else
        log_warn "Database file not found: $db_path"
    fi
}

# Apply migrations
migrate_up() {
    local service="$1"
    local steps="${2:-}"
    local db_path=$(get_db_path "$service")
    local migrations_path=$(get_migrations_path "$service")
    
    log_info "Applying migrations for $service"
    
    if [ -n "$steps" ]; then
        migrate -path "$migrations_path" -database "sqlite3://$db_path" up "$steps"
    else
        migrate -path "$migrations_path" -database "sqlite3://$db_path" up
    fi
    
    log_info "Migrations applied successfully"
}

# Rollback migrations
migrate_down() {
    local service="$1"
    local steps="${2:-1}"
    local db_path=$(get_db_path "$service")
    local migrations_path=$(get_migrations_path "$service")
    
    log_warn "Rolling back $steps migration(s) for $service"
    
    migrate -path "$migrations_path" -database "sqlite3://$db_path" down "$steps"
    
    log_info "Rollback completed"
}

# Show current version
migrate_version() {
    local service="$1"
    local db_path=$(get_db_path "$service")
    local migrations_path=$(get_migrations_path "$service")
    
    log_info "Current migration version for $service:"
    migrate -path "$migrations_path" -database "sqlite3://$db_path" version
}

# Force set version
migrate_force() {
    local service="$1"
    local version="$2"
    local db_path=$(get_db_path "$service")
    local migrations_path=$(get_migrations_path "$service")
    
    if [ -z "$version" ]; then
        log_error "Version number required for force command"
        exit 1
    fi
    
    log_warn "Forcing version to $version for $service"
    migrate -path "$migrations_path" -database "sqlite3://$db_path" force "$version"
    log_info "Version forced successfully"
}

# Show migration status
migrate_status() {
    local service="$1"
    local migrations_path=$(get_migrations_path "$service")
    
    log_info "Available migrations for $service:"
    ls -la "$migrations_path"/*.sql 2>/dev/null || log_warn "No migrations found"
}

# Apply migrations to all services
migrate_all() {
    local command="${1:-up}"
    
    log_info "Running $command on all services"
    
    for service in jr crm; do
        log_info "Processing $service..."
        case "$command" in
            up)
                backup "$service"
                migrate_up "$service"
                ;;
            version)
                migrate_version "$service"
                ;;
            status)
                migrate_status "$service"
                ;;
            *)
                log_error "Command '$command' not supported for all services"
                exit 1
                ;;
        esac
    done
}

# Show usage
usage() {
    cat << EOF
Usage: $0 <command> [service] [args]

Commands:
    up [service] [steps]     Apply migrations (all if no steps specified)
    down [service] [steps]   Rollback migrations (1 by default)
    version [service]        Show current migration version
    force [service] [ver]    Force set migration version
    status [service]         List available migrations
    backup [service]         Create database backup
    all [command]            Run command on all services (up, version, status)

Services:
    jr, bronivik-jr          Bronivik Jr (device booking)
    crm, bronivik-crm        Bronivik CRM (cabinet booking)

Examples:
    $0 up jr                 Apply all migrations for Bronivik Jr
    $0 down jr 1             Rollback 1 migration for Bronivik Jr
    $0 version crm           Show migration version for Bronivik CRM
    $0 backup jr             Create backup of Bronivik Jr database
    $0 all up                Apply migrations for all services

Environment Variables:
    BRONIVIK_JR_DB           Path to Bronivik Jr database (default: /app/data/bronivik_jr.db)
    BRONIVIK_CRM_DB          Path to Bronivik CRM database (default: /app/data/bronivik_crm.db)
    BACKUP_DIR               Backup directory (default: /app/backups)
EOF
    exit 1
}

# Main
main() {
    check_migrate
    
    local command="${1:-}"
    local service="${2:-}"
    local args="${3:-}"
    
    case "$command" in
        up)
            if [ "$service" = "all" ] || [ -z "$service" ]; then
                migrate_all up
            else
                backup "$service"
                migrate_up "$service" "$args"
            fi
            ;;
        down)
            if [ -z "$service" ]; then
                log_error "Service required for down command"
                exit 1
            fi
            backup "$service"
            migrate_down "$service" "$args"
            ;;
        version)
            if [ "$service" = "all" ] || [ -z "$service" ]; then
                migrate_all version
            else
                migrate_version "$service"
            fi
            ;;
        force)
            if [ -z "$service" ]; then
                log_error "Service required for force command"
                exit 1
            fi
            migrate_force "$service" "$args"
            ;;
        status)
            if [ "$service" = "all" ] || [ -z "$service" ]; then
                migrate_all status
            else
                migrate_status "$service"
            fi
            ;;
        backup)
            if [ -z "$service" ]; then
                log_error "Service required for backup command"
                exit 1
            fi
            backup "$service"
            ;;
        all)
            migrate_all "$service"
            ;;
        help|--help|-h)
            usage
            ;;
        *)
            log_error "Unknown command: $command"
            usage
            ;;
    esac
}

main "$@"
