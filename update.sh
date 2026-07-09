#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# CommitLens On-Premise Updater
# Usage:
#   ./update.sh               → upgrades to whatever COMMITLENS_VERSION is in .env
#   ./update.sh v1.3.0        → pins to v1.3.0 and upgrades
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

COMPOSE_FILE="$(dirname "$0")/docker-compose.yml"
ENV_FILE="$(dirname "$0")/.env"

# ── Colour helpers ────────────────────────────────────────────────────────────
GREEN='\033[0;32m'; YELLOW='\033[1;33m'; RED='\033[0;31m'; NC='\033[0m'
info()    { echo -e "${GREEN}[commitlens]${NC} $*"; }
warn()    { echo -e "${YELLOW}[commitlens]${NC} $*"; }
error()   { echo -e "${RED}[commitlens]${NC} $*"; exit 1; }

# ── Pin to a specific version if provided ────────────────────────────────────
if [[ -n "${1:-}" ]]; then
    NEW_VERSION="$1"
    info "Pinning COMMITLENS_VERSION to $NEW_VERSION in .env"
    if grep -q "^COMMITLENS_VERSION=" "$ENV_FILE"; then
        sed -i "s/^COMMITLENS_VERSION=.*/COMMITLENS_VERSION=${NEW_VERSION}/" "$ENV_FILE"
    else
        echo "COMMITLENS_VERSION=${NEW_VERSION}" >> "$ENV_FILE"
    fi
fi

# ── Read current version ──────────────────────────────────────────────────────
CURRENT_VERSION=$(grep "^COMMITLENS_VERSION=" "$ENV_FILE" 2>/dev/null | cut -d= -f2 || echo "latest")
info "Target version: ${CURRENT_VERSION}"

# ── Backup database before upgrade ───────────────────────────────────────────
BACKUP_DIR="$(dirname "$0")/backups"
mkdir -p "$BACKUP_DIR"
BACKUP_FILE="${BACKUP_DIR}/pre-upgrade_$(date +%Y%m%dT%H%M%S).sql.gz"

warn "Creating database backup → $BACKUP_FILE"
DB_USER=$(grep "^DB_USER=" "$ENV_FILE" | cut -d= -f2)
DB_NAME=$(grep "^DB_NAME=" "$ENV_FILE" | cut -d= -f2)

docker compose -f "$COMPOSE_FILE" exec -T postgres \
    pg_dump -U "$DB_USER" "$DB_NAME" | gzip > "$BACKUP_FILE" \
    && info "Backup complete." \
    || warn "Backup failed — continuing anyway. Ensure postgres is running."

# ── Pull new image ────────────────────────────────────────────────────────────
info "Pulling new image..."
docker compose -f "$COMPOSE_FILE" pull app

# ── Restart the app service (zero-downtime: postgres + redis keep running) ────
info "Restarting app service..."
docker compose -f "$COMPOSE_FILE" up -d --no-deps app

# ── Wait for health check ─────────────────────────────────────────────────────
info "Waiting for health check..."
RETRIES=0
until docker inspect --format='{{.State.Health.Status}}' commitlens_app 2>/dev/null | grep -q "healthy"; do
    sleep 3
    RETRIES=$((RETRIES + 1))
    if [[ $RETRIES -ge 20 ]]; then
        error "App did not become healthy after 60s. Check logs: docker compose logs app"
    fi
done

info "Update complete! CommitLens is running version: ${CURRENT_VERSION}"
info "To verify: curl -k https://localhost/health"
