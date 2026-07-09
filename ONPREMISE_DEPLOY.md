# CommitLens — On-Premise Deployment Guide

This guide walks a team through self-hosting CommitLens on their own server (Linux, bare-metal or VM).

---

## Architecture Overview

```
Internet / Internal network
        │
   ┌────▼─────┐
   │  Nginx   │  :80 (→ redirect) / :443 (TLS)
   └────┬─────┘
        │ proxy_pass
   ┌────▼──────────┐
   │ CommitLens API │  :8000  (Go / Echo)
   └────┬──────────┘
        │
   ┌────┴──────────────────┐
   │                       │
┌──▼──┐              ┌─────▼────┐
│ PG  │ (pgvector)   │  Redis   │ (asynq queue)
└─────┘              └──────────┘
```

External SaaS dependencies (the team must supply their own credentials):
| Service | Purpose | Can it be replaced? |
|---|---|---|
| Azure OpenAI | Commit analysis + vector embeddings | Yes — see "Replacing Azure OpenAI" below |
| GitHub App | Webhook delivery + API access | No — needs a GitHub App registration |
| GitHub OAuth | Login with GitHub | No — needs an OAuth App registration |
| Google OAuth | Login with Google | Optional — disable if not needed |
| SMTP | Transactional email | Any SMTP server works |

---

## Prerequisites

| Requirement | Minimum |
|---|---|
| OS | Ubuntu 22.04 / Debian 12 / RHEL 9 or any modern Linux |
| Docker | 24+ |
| Docker Compose | v2.20+ (`docker compose` plugin) |
| RAM | 2 GB (4 GB recommended) |
| Disk | 20 GB+ |
| Ports open | 80, 443 (inbound); 443 outbound to GitHub API + Azure OpenAI |
| Domain / IP | A resolvable hostname or internal IP for TLS |

---

## Step 1 — Install Docker

```bash
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER   # log out and back in after this
docker compose version           # should print v2.x.x
```

---

## Step 2 — Clone / Transfer the Code

If the team has internet access to your repo:
```bash
git clone https://github.com/your-org/commitlens.git
cd commitlens
```

If air-gapped, transfer the directory via `scp` or a USB drive.

---

## Step 3 — TLS Certificate

Place two files inside `deploy/certs/`:
- `fullchain.pem` — your certificate chain
- `privkey.pem`   — your private key

**Option A — Let's Encrypt (public domain):**
```bash
sudo apt install certbot
sudo certbot certonly --standalone -d api.yourcompany.com
sudo cp /etc/letsencrypt/live/api.yourcompany.com/fullchain.pem deploy/certs/
sudo cp /etc/letsencrypt/live/api.yourcompany.com/privkey.pem   deploy/certs/
sudo chmod 644 deploy/certs/*.pem
```

**Option B — Self-signed (internal / air-gapped networks):**
```bash
mkdir -p deploy/certs
openssl req -x509 -newkey rsa:4096 -keyout deploy/certs/privkey.pem \
  -out deploy/certs/fullchain.pem -sha256 -days 3650 -nodes \
  -subj "/CN=api.yourcompany.com"
```
Distribute `fullchain.pem` to clients / browsers as a trusted CA.

**Option C — No TLS (internal HTTP only):**
Remove the `nginx` service from `docker-compose.yml` and expose port 8000 directly from the `app` service. Not recommended for production.

---

## Step 4 — Configure Environment

```bash
cp .env.example .env
nano .env          # fill in EVERY value marked CHANGE_ME
```

Generate strong secrets with:
```bash
openssl rand -hex 32    # for DB_PASSWORD, REDIS_PASSWORD, JWT_SECRET, etc.
```

Update `deploy/nginx.conf` — replace `api.yourcompany.com` with your actual domain or IP.

---

## Step 5 — Register a GitHub App

1. Go to **https://github.com/settings/apps** → **New GitHub App**
2. Set:
   - **Homepage URL**: `https://commitlens.yourcompany.com`
   - **Webhook URL**: `https://api.yourcompany.com/v1/github/webhook`
   - **Webhook secret**: the value you put in `githubwebhooksecret`
   - **Permissions**: Repository → Contents (read), Metadata (read), Webhooks (read)
   - **Subscribe to events**: Push, Pull request
3. Generate a **Private Key** (.pem) and paste its contents (with `\n` line breaks) into `GITHUB_PRIVATE_KEY` in `.env`.
4. Copy the **App ID** into `GITHUB_APP_ID`.
5. Install the app on the organisation/repositories you want to index.

---

## Step 6 — Register OAuth Apps (for login)

**GitHub OAuth:**
1. Go to **https://github.com/settings/developers** → **New OAuth App**
2. Callback URL: `https://api.yourcompany.com/v1/auth/github/callback`
3. Copy Client ID and Secret into `.env`.

**Google OAuth:**
1. Go to **https://console.cloud.google.com/apis/credentials** → **Create credentials** → OAuth client ID
2. Authorised redirect URI: `https://api.yourcompany.com/v1/auth/google/callback`
3. Copy Client ID and Secret into `.env`.

---

## Step 7 — Start the Stack

```bash
docker compose up -d --build
```

Watch logs:
```bash
docker compose logs -f app
docker compose logs -f postgres
```

Verify the API is healthy:
```bash
curl -k https://api.yourcompany.com/health
# or if using HTTP: curl http://localhost:8000/health
```

---

## Step 8 — Deploy the Frontend

The Next.js frontend is a separate service. The team can:

**A) Self-host with Docker:**
```bash
# inside the frontend repo
docker build -t commitlens-frontend .
docker run -d -p 3000:3000 \
  -e NEXT_PUBLIC_API_URL=https://api.yourcompany.com \
  commitlens-frontend
```

**B) Serve behind Nginx:**
Add a second `server {}` block in `deploy/nginx.conf` pointing at port 3000.

Set `FRONTEND_URL` in `.env` to wherever the frontend is reachable.

---

## Replacing Azure OpenAI (fully air-gapped AI)

If the team cannot use Azure OpenAI, deploy [Ollama](https://ollama.com) and use a compatible proxy:

```bash
# On the same server
curl -fsSL https://ollama.com/install.sh | sh
ollama pull llama3
ollama pull nomic-embed-text   # for embeddings
```

Then run [LiteLLM](https://github.com/BerriAI/litellm) as an OpenAI-compatible proxy:
```bash
docker run -d -p 8001:8000 \
  -e OPENAI_API_KEY=sk-fake \
  ghcr.io/berriai/litellm:main \
  --model ollama/llama3
```

Point CommitLens at it:
```
AZURE_OPENAI_ENDPOINT=http://litellm:8000/
AZURE_OPENAI_KEY=sk-fake
AZURE_OPENAI_MODEL=ollama/llama3
AZURE_EMBEDDING_ENDPOINT=http://litellm:8000/v1/embeddings
```

> Note: This requires changes to the CommitLens Go code that calls the Azure SDK — the Azure SDK client needs to be swapped for the standard OpenAI client or a compatible wrapper.

---

## Day-2 Operations

### Backups

```bash
# PostgreSQL daily backup
docker exec commitlens_postgres pg_dump -U commitlens commitlens | \
  gzip > /backups/commitlens_$(date +%F).sql.gz

# Restore
gunzip -c /backups/commitlens_2026-07-06.sql.gz | \
  docker exec -i commitlens_postgres psql -U commitlens commitlens
```

### Updates

```bash
git pull                        # or transfer new code
docker compose up -d --build    # rebuilds only changed layers
```

### Scale the API (multi-instance)

```bash
docker compose up -d --scale app=3
```
Update `nginx.conf` to use `upstream` with all three instances.

### Logs

```bash
docker compose logs -f              # all services
docker compose logs -f app          # API only
docker logs commitlens_postgres     # database
```

### Stop / restart

```bash
docker compose down          # stop (data is preserved in volumes)
docker compose down -v       # ⚠️  also deletes all data volumes
docker compose restart app   # restart only the API
```

---

## Security Checklist

- [ ] All `CHANGE_ME` values replaced with strong random secrets
- [ ] `.env` is not in git (add `.env` to `.gitignore`)
- [ ] Firewall allows only 80/443 inbound; 5432/6379 are NOT exposed externally
- [ ] TLS certificate is valid and auto-renews (or has a renewal reminder)
- [ ] PostgreSQL and Redis are not accessible from outside the Docker network
- [ ] Regular database backups are scheduled (cron or similar)
- [ ] GitHub App permissions scoped to minimum required
- [ ] `JWT_SECRET` is at least 32 random characters

---

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| `app` exits immediately | Missing env var or DB not ready | Check `docker compose logs app` |
| `extension "vector" does not exist` | pgvector not enabled | `deploy/init.sql` runs automatically on first start |
| Redis auth errors | `REDIS_PASSWORD` mismatch | Ensure same value in `.env` and redis command |
| GitHub webhooks not arriving | Webhook URL unreachable from GitHub | Make sure port 443 is open; check firewall |
| OAuth redirect mismatch | Redirect URL not registered | Add exact callback URL in GitHub/Google OAuth app settings |
