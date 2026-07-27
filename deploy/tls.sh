#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# CommitLens TLS helper
# Usage:
#   ./deploy/tls.sh bootstrap   → one-time: create a throwaway self-signed cert
#                                 so nginx can start at all (chicken/egg fix)
#   ./deploy/tls.sh issue       → request/renew the real Let's Encrypt cert via
#                                 the ACME webroot challenge, then reload nginx
#                                 (safe to re-run; put this on a cron job)
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
ENV_FILE="$ROOT_DIR/.env"
CERTS_DIR="$ROOT_DIR/deploy/certs"
CERTBOT_STATE_DIR="$ROOT_DIR/deploy/certbot-state"
CERTBOT_WEBROOT="$ROOT_DIR/deploy/certbot-webroot"

GREEN='\033[0;32m'; YELLOW='\033[1;33m'; RED='\033[0;31m'; NC='\033[0m'
info()  { echo -e "${GREEN}[tls]${NC} $*"; }
warn()  { echo -e "${YELLOW}[tls]${NC} $*"; }
error() { echo -e "${RED}[tls]${NC} $*"; exit 1; }

DOMAIN=$(grep "^DOMAIN=" "$ENV_FILE" 2>/dev/null | cut -d= -f2 || true)
LETSENCRYPT_EMAIL=$(grep "^LETSENCRYPT_EMAIL=" "$ENV_FILE" 2>/dev/null | cut -d= -f2 || true)
[[ -n "$DOMAIN" ]] || error "Set DOMAIN=commitlens.tech in .env first"

mkdir -p "$CERTS_DIR" "$CERTBOT_STATE_DIR" "$CERTBOT_WEBROOT"

case "${1:-}" in
  bootstrap)
    if [[ -f "$CERTS_DIR/fullchain.pem" ]]; then
      warn "$CERTS_DIR/fullchain.pem already exists — skipping. Delete it first if you want a fresh throwaway cert."
      exit 0
    fi
    info "Generating throwaway self-signed cert for $DOMAIN (nginx needs *a* cert to start)…"
    openssl req -x509 -newkey rsa:4096 -keyout "$CERTS_DIR/privkey.pem" \
      -out "$CERTS_DIR/fullchain.pem" -sha256 -days 1 -nodes \
      -subj "/CN=$DOMAIN"
    info "Done. Now run: docker compose up -d"
    info "Once nginx is up and port 80/443 are reachable from the internet, run: ./deploy/tls.sh issue"
    ;;

  issue)
    [[ -n "$LETSENCRYPT_EMAIL" ]] || error "Set LETSENCRYPT_EMAIL=you@example.com in .env first"
    info "Requesting/renewing certificate for $DOMAIN via webroot challenge…"
    docker run --rm \
      -v "$CERTBOT_STATE_DIR:/etc/letsencrypt" \
      -v "$CERTBOT_WEBROOT:/var/www/certbot" \
      certbot/certbot certonly --webroot -w /var/www/certbot \
      -d "$DOMAIN" --agree-tos -m "$LETSENCRYPT_EMAIL" --no-eff-email --non-interactive --keep-until-expiring

    info "Copying issued cert into $CERTS_DIR and reloading nginx…"
    # certbot writes as root inside certbot-state/ — need sudo to read it back out.
    sudo cp "$CERTBOT_STATE_DIR/live/$DOMAIN/fullchain.pem" "$CERTS_DIR/fullchain.pem"
    sudo cp "$CERTBOT_STATE_DIR/live/$DOMAIN/privkey.pem"   "$CERTS_DIR/privkey.pem"
    sudo chown "$(id -u):$(id -g)" "$CERTS_DIR/fullchain.pem" "$CERTS_DIR/privkey.pem"
    chmod 644 "$CERTS_DIR/fullchain.pem"
    chmod 600 "$CERTS_DIR/privkey.pem"
    docker compose -f "$ROOT_DIR/docker-compose.yml" exec nginx nginx -s reload
    info "Certificate for $DOMAIN is live."
    ;;

  *)
    echo "Usage: $0 {bootstrap|issue}"
    exit 1
    ;;
esac
