#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE="$SCRIPT_DIR/docker-compose.yaml"
ENV_FILE="$SCRIPT_DIR/.env"

usage() {
    cat <<EOF
Usage: $(basename "$0") <command>

Commands:
  start       Start PostgreSQL container
  stop        Stop PostgreSQL container
  restart     Restart PostgreSQL container
  status      Check container health
  logs        Show container logs
  migrate     Run database migrations
  seed        Seed initial data (templates, images)
  backup      Backup database to ./backups/
  shell       Open psql shell
  connection   Print connection string

Examples:
  $(basename "$0") start
  $(basename "$0") migrate
  POSTGRES_PASSWORD=secret $(basename "$0") start
EOF
    exit 1
}

load_env() {
    if [[ -f "$ENV_FILE" ]]; then
        set -a; source "$ENV_FILE"; set +a
    fi
    POSTGRES_USER="${POSTGRES_USER:-openclaw}"
    POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-openclaw}"
    POSTGRES_DB="${POSTGRES_DB:-openclaw_autodeploy}"
    POSTGRES_PORT="${POSTGRES_PORT:-5432}"
}

conn_str() {
    echo "postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@localhost:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable"
}

cmd_start() {
    load_env
    docker compose -f "$COMPOSE_FILE" up -d
    echo "Waiting for PostgreSQL to be ready..."
    local retries=30
    while [[ $retries -gt 0 ]]; do
        if docker compose -f "$COMPOSE_FILE" exec -T postgres pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB" > /dev/null 2>&1; then
            echo "PostgreSQL is ready."
            return 0
        fi
        retries=$((retries - 1))
        sleep 1
    done
    echo "ERROR: PostgreSQL failed to start."
    return 1
}

cmd_stop() {
    load_env
    docker compose -f "$COMPOSE_FILE" down
}

cmd_restart() {
    cmd_stop
    cmd_start
}

cmd_status() {
    load_env
    docker compose -f "$COMPOSE_FILE" ps
}

cmd_logs() {
    load_env
    docker compose -f "$COMPOSE_FILE" logs -f
}

cmd_migrate() {
    load_env
    local migrations_dir="${1:-}"
    if [[ -z "$migrations_dir" ]]; then
        if [[ -d "$SCRIPT_DIR/../../migrations" ]]; then
            migrations_dir="$SCRIPT_DIR/../../migrations"
        else
            echo "ERROR: Provide migrations directory path"
            exit 1
        fi
    fi

    echo "Running migrations from: $migrations_dir"
    local conn="$(conn_str)"
    for f in $(ls "$migrations_dir"/*.up.sql 2>/dev/null | sort); do
        echo "  $(basename "$f")"
        docker compose -f "$COMPOSE_FILE" exec -T postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" < "$f"
    done
    echo "Migrations complete."
}

cmd_seed() {
    load_env
    local seed_file="${1:-}"
    if [[ -z "$seed_file" ]]; then
        seed_file="$SCRIPT_DIR/../../scripts/seed.sql"
    fi

    if [[ ! -f "$seed_file" ]]; then
        echo "No seed file found at: $seed_file"
        return 1
    fi

    echo "Seeding from: $seed_file"
    docker compose -f "$COMPOSE_FILE" exec -T postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" < "$seed_file"
    echo "Seed complete."
}

cmd_shell() {
    load_env
    docker compose -f "$COMPOSE_FILE" exec -T postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB"
}

cmd_connection() {
    load_env
    echo "$(conn_str)"
}

cmd_backup() {
    load_env
    local backup_dir="$SCRIPT_DIR/backups"
    mkdir -p "$backup_dir"
    local ts="$(date +%Y%m%d_%H%M%S)"
    local dump_file="$backup_dir/openclaw_${ts}.sql.gz"
    echo "Backing up to: $dump_file"
    docker compose -f "$COMPOSE_FILE" exec -T postgres pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB" | gzip > "$dump_file"
    echo "Backup complete: $dump_file"
    echo "Size: $(du -sh "$dump_file" | cut -f1)"
}

COMMAND="${1:-}"
shift || true

case "$COMMAND" in
    start)    cmd_start "$@" ;;
    stop)     cmd_stop ;;
    restart)  cmd_restart ;;
    status)   cmd_status ;;
    logs)     cmd_logs ;;
    migrate)  cmd_migrate "$@" ;;
    seed)     cmd_seed "$@" ;;
    backup)   cmd_backup ;;
    shell)    cmd_shell ;;
    connection) cmd_connection ;;
    *)        usage ;;
esac
