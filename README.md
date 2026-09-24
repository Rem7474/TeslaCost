# AutoLedger 🚗⚡

> Self-hosted, open-source app for tracking the full cost of ownership (TCO), maintenance, tires and carpooling of a car, with real-time [TeslaMate](https://github.com/teslamate-org/teslamate) synchronization or fully standalone operation.

[![CI / CD Pipeline](https://github.com/Rem7474/TeslaCost/actions/workflows/ci.yml/badge.svg)](https://github.com/Rem7474/TeslaCost/actions/workflows/ci.yml)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=Rem7474_TeslaCost&metric=alert_status)](https://sonarcloud.io/summary/new_code?id=Rem7474_TeslaCost)
[![Docker Image](https://img.shields.io/badge/docker-ghcr.io-blue?logo=docker)](https://github.com/Rem7474/TeslaCost/pkgs/container/teslacost)

> The project's repository, Go module, Docker image, database and volumes keep the historical `teslacost` name; only the product itself is now called AutoLedger. The UI and API messages are available in English and French.

---

## 🌟 Highlights

- **⚡ Resilient TeslaMate synchronization**: real-time odometer readback, resumable full import, sliding re-read of recent costs, and detection of deleted drives/charges.
- **📊 TCO calculator, down to the cent**: a single cost ledger (`cost_ledger`), financing support (cash, loan with amortization schedule, LOA/LLD lease-to-own), real depreciation and a residual-value provision.
- **🔐 Hybrid authentication (local JWT & OIDC SSO)**: native support for Authentik, Keycloak, Authelia and Kanidm via a secure Authorization Code Flow, JIT provisioning and a local fallback.
- **🔔 Maintenance reminders & homelab webhooks**: due-date and/or mileage alerts with outbound connectors to Telegram, Discord, Gotify and a generic JSON webhook.
- **📁 Document & invoice manager**: attachments (PDFs, photos) stored on a dedicated Docker volume with strict application-level access control.
- **🛞 Tire lifecycle management**: tread-depth tracking, transactional mount/dismount sessions, storage periods excluded automatically, and remaining-mileage projection.
- **👥 Fair carpooling module**: automatic sync of trip dates, batch recomputation of real costs, an electricity price weighted on recent charges, and a monthly consolidated per-km insurance share.
- **📱 PWA, quick add & offline mode**: a "+" button (charge, fill-up, toll/parking) in the bottom bar, a one-field prompt to complete the cost of a TeslaMate charge, a receipt photo straight from the device, and offline entry with an IndexedDB queue deduplicated by idempotency key.

---

## 🚀 Feature Details

### 1. Authentication & Security
- **Local authentication**: password sign-up/sign-in (bcrypt, 72-byte maximum), a 15-minute HS256 JWT access token and a 30-day rotating refresh token, the latter only in an `HttpOnly` cookie (never in a response body). An account locks out after 10 failed attempts in 15 minutes; an unknown email and a wrong password get the same response in the same amount of time.
- **OIDC / OAuth2 (SSO) support**: delegate authentication to your homelab IdP (Authentik, Keycloak, Authelia, Kanidm).
  - Standard *Authorization Code Flow* with PKCE (S256), anti-CSRF check (`state`) and anti-replay (SHA-256-hashed `nonce`).
  - Just-In-Time (JIT) provisioning: automatic account creation, or linking to a local account sharing the same email. The address must not be reported unverified by the provider (`email_verified: false` is refused), and only a local account with no SSO identity yet gets linked: an account already linked is never re-pointed to another identity.
  - Optional allowlist (`OIDC_ALLOWED_EMAILS`, case-insensitive) and the option to disable local authentication entirely (`OIDC_DISABLE_LOCAL_AUTH`).
- **Account & security** (the `Account` page): list of signed-in devices (browser, OS, address, last activity), signing out one device or all of them, and a password change that signs out the other devices. A signed-out device keeps access for up to 15 minutes, the access token's lifetime. Expired or revoked refresh tokens are purged daily.
- **Encryption at rest**: TeslaMate credentials and connection tokens are encrypted in PostgreSQL with symmetric AES-256-GCM.
- **Rate limiting**: `/api/auth/login` and `/api/auth/register` are limited to 10 attempts/minute per client address (see `TRUSTED_PROXIES` behind a reverse proxy); `/api/auth/refresh` and the SSO routes have wider thresholds.

### 2. TeslaMate API Synchronization
- **Real-time odometer**: the vehicle's odometer updates live as soon as TeslaMate reports it.
- **Background jobs**: non-blocking synchronization with progress tracking and per-vehicle mutual exclusion.
- **Resume after interruption**: importing drive (`/drives`) and charge (`/charges`) history automatically picks up where it left off.
- **Full re-read after an enrichment**: when a migration adds new data sourced from TeslaMate (battery levels, temperature), the import state is reset and the next synchronization re-reads the whole history. Manually entered costs are preserved.
- **Sliding re-read (30 days)**: automatically updates charge costs that TeslaMate completes after the fact.
- **Manual & costless charges**: charges with no fare are clearly flagged (never forced to €0), and charges outside TeslaMate's tracking can be entered by hand (e.g. a third party's home outlet).
- **Integrity checks**: odometer continuity checks (gaps, rollbacks, distance mismatches) and a safety threshold on bulk data deletion.

### 3. TCO Calculator & Car Financing
- **Unified cost ledger (`cost_ledger`)**: centralizes charges, tolls, maintenance expenses, insurance premiums, tire amortization and acquisition cost.
- **Supported acquisition modes**:
  - *Cash*: linear depreciation based on the estimated resale value or the actual sale amount when the vehicle is disposed of.
  - *Classic loan*: a month-by-month amortization schedule, principal/interest split, origination fees and borrower insurance.
  - *LOA & LLD (lease-to-own / long-term lease)*: down payment, monthly payments, security deposit, contractual mileage allowance and a monthly provision for excess mileage or refurbishment fees.
- **Advanced financial indicators**: usage cost per km (energy + tolls), full cost per km, net cost after carpooling revenue, and a TCO completeness score.
- **Energy efficiency** (electric vehicles): real consumption in kWh/100 km, energy cost per 100 km (monthly and 3-month average), charging efficiency (energy stored over energy drawn from the grid), and a breakdown of charges between home outlet, AC and DC based on their average power. Endpoint: `GET /api/vehicles/{id}/energy-stats`.
- **Battery and temperature**: start/end battery level of every drive and charge, average outside temperature (converted to °C whatever TeslaMate's unit is), estimated usable capacity from the energy added and the percentage gained, the cost of a 0-100% charge by charge type, and cold-weather overconsumption compared to mild weather. Battery health as computed by TeslaMate (`/battery-health`, recent TeslaMateApi versions) is read on every synchronization, once a day, to track how it evolves.

### 4. Maintenance Reminders & Homelab Notifications
- **Dual trigger condition**: combined monitoring of a due date and/or a mileage threshold computed from the real odometer.
- **Configurable lead time**: an advance warning before the critical threshold is crossed (e.g. warn 500 km or 15 days ahead).
- **Webhook notification connectors**:
  - **Discord**: rich embed messages (status colors, organized fields).
  - **Telegram**: Markdown-formatted messages via the bot HTTP API.
  - **Gotify**: self-hosted push notifications with priority handling.
  - **Generic JSON**: direct integration with Home Assistant, Node-RED or n8n.
- **Sync-failure alert**: the same vehicle webhook is also used to warn when TeslaMate synchronization fails repeatedly and is automatically suspended (circuit breaker), without waiting for you to open the app.

### 5. Document Archiving & Management
- **Filesystem storage**: attachments (maintenance invoices, receipts) live on a dedicated volume (`/data/documents`) rather than in PostgreSQL.
- **File security**: non-root isolation (`teslacost` user), no direct static HTTP access, strict JWT-based access control and path-traversal prevention.

### 6. Tire Lifecycle Management
- **Full tire records**: brand, model, dimensions, load/speed index, season (summer/winter/all-season), purchase price and DOT code.
- **Tread-depth tracking**: per-tire wear readings with automatic projection of remaining mileage (time spent in storage is automatically excluded from the calculation).
- **Mount/dismount sessions**: grouped mounts and dismounts across axles (`FL`, `FR`, `RL`, `RR`, `STORAGE`, `DISPOSED`) with a chronological history.

### 7. Carpooling Module
- **Split into legs**: linked to real TeslaMate drives or entered manually.
- **Date sync & batch recompute**: automatically realigns the carpool date to the actual drives and immediately recomputes each passenger's share.
- **Accurate energy & insurance**: electricity price computed from a weighted average of recent charges (a 5-day window), and an exact monthly insurance share prorated across the distance driven in that month (with stable reference rates for the ongoing month).

### 8. Trip Search & Navigation
- **Time filters**: filter by month, year, a custom range, or show everything.
- **Full-text search**: filter by start and end address.
- **Qualification queue**: automatic detection of highway drives needing toll qualification or review.

---

## 📁 Project Layout

```text
TeslaCost/
├── cmd/
│   └── server/
│       └── main.go                 # Application entry point & Chi routing
├── internal/
│   ├── auth/                       # bcrypt hashing, JWT tokens and the OIDC client
│   ├── config/                     # Environment variable loading and validation
│   ├── crypto/                     # AES-256-GCM symmetric encryption
│   ├── database/                   # pgx pool, SQL migrations and the repository layer
│   ├── handlers/                   # Chi REST HTTP controllers
│   ├── middleware/                 # JWT auth, CORS, logging
│   ├── models/                     # Strongly-typed Go data models
│   ├── services/                   # TCO engine, tire wear, carpooling, notifications
│   ├── storage/                    # Document storage service on a Docker volume
│   └── teslamate/                  # Client for the teslamateapi service
├── migrations/                     # Versioned PostgreSQL schema and migrations
├── web/                            # Vue 3 + Vite + Tailwind CSS SPA frontend, PWA
├── docker-compose.yml              # Containerized stack configuration
├── Dockerfile                      # Multi-stage build (Vue 3 + static Go binary)
└── .env.example                    # Configuration variable template
```

---

## 🛠️ Deployment & Installation

### Option 1: Docker Compose (recommended)

1. **Create a directory and download the configuration files:**
   ```bash
   mkdir autoledger && cd autoledger
   curl -O https://raw.githubusercontent.com/Rem7474/TeslaCost/main/docker-compose.yml
   curl -o .env https://raw.githubusercontent.com/Rem7474/TeslaCost/main/.env.example
   ```

2. **Generate your encryption key and configure `.env`:**
   ```bash
   # Generate a 32-byte AES-256 encryption key (64 hex characters):
   openssl rand -hex 32
   ```
   Fill in your keys in the `.env` file:
   - `APP_ENCRYPTION_KEY`: the generated encryption key
   - `DB_PASSWORD`: the database password
   - `JWT_SECRET`: the session-signing secret
   - *(Optional)* The OIDC / SSO section if you delegate authentication to your IdP

3. **Start the stack:**
   ```bash
   docker compose up -d
   ```

The application is available at **`http://localhost:8080`**.

> The repo's [`docker-compose.yml`](./docker-compose.yml) (the one downloaded in step 1) is authoritative; it includes the `postgres`, `api` and `backup` services (automatic backups, see "Operations" below) with their respective healthchecks.

---


### Option 2: Official Docker image (GHCR)

The multi-architecture Docker image (`linux/amd64`, `linux/arm64`) is published automatically to the GitHub Container Registry:

```bash
docker pull ghcr.io/rem7474/teslacost:latest
```

Example standalone run with an external PostgreSQL instance:

```bash
docker run -d \
  --name autoledger \
  -p 8080:8080 \
  -v teslacost_docs:/data/documents \
  -e ENVIRONMENT="production" \
  -e DATABASE_URL="postgres://user:password@postgres-host:5432/teslacost?sslmode=disable" \
  -e JWT_SECRET="your_strong_jwt_secret" \
  -e APP_ENCRYPTION_KEY="hex_key_of_exactly_64_characters" \
  -e APP_TIMEZONE="Europe/Paris" \
  ghcr.io/rem7474/teslacost:latest
```

---

### Exposing it on the Internet: reverse proxy & security

AutoLedger does not terminate TLS: put it behind a reverse proxy that does (Caddy or Traefik with automatic Let's Encrypt, or Nginx with an existing certificate). Everything else is handled by the application itself.

- **Secrets**: with `ENVIRONMENT=production` (the default in `docker-compose.yml`), the server refuses to start as long as `JWT_SECRET`, `APP_ENCRYPTION_KEY` or `DB_PASSWORD` still hold a value published in this repository. The error message names the variable to change. `ENVIRONMENT=development` keeps the example values for a local trial.
- **Client address**: `X-Forwarded-For` and `X-Forwarded-Proto` are only trusted when the connection comes from a trusted proxy (`TRUSTED_PROXIES`, defaulting to loopback and private ranges: Docker network, LAN). The chain is read right to left: entries added by the client are never used. The sign-in attempt limiter and sessions record this address. A proxy on a public address must be listed; `TRUSTED_PROXIES=none` trusts none.
- **Security headers** set by the application: `Content-Security-Policy` (same-origin resources, `blob:` previews), `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy`, `Permissions-Policy`, `Cross-Origin-Opener-Policy`, and `Strict-Transport-Security` on HTTPS requests only. `SECURITY_HEADERS=false` disables them if your proxy already sets them; `CONTENT_SECURITY_POLICY` overrides the policy (`off` removes only this one).
- **Cross-site requests**: a data-modifying request whose `Origin` header is neither `APP_BASE_URL`, an origin from `CORS_ALLOWED_ORIGINS`, nor the requested host is rejected (403). Requests carrying an `Authorization` header or no `Origin` at all (scripts, `curl`) are not affected.
- **Application port**: do not publish port 8080 to the Internet; only the proxy should reach it (`PORT` and the Docker network). A direct connection from the outside that appears to come from a private address (Docker NAT without source-IP preservation) would be treated as a trusted proxy.
- Sessions use a 15-minute access token (`JWT_ACCESS_EXPIRATION_MINUTES`) renewed by a 30-day rotating refresh token (`JWT_REFRESH_EXPIRATION_DAYS`).

Example with **Caddy** (`Caddyfile`):

```caddyfile
autoledger.homelab.local {
    reverse_proxy localhost:8080
}
```

Remember to adjust `APP_BASE_URL` and `CORS_ALLOWED_ORIGINS` to match the public domain name used, and to leave `COOKIE_SECURE` at its default (enabled automatically as soon as `APP_BASE_URL` starts with `https://` or `ENVIRONMENT=production`).

---

## 🛟 Operations: backups, restore & diagnostics

### Automatic backups

The `backup` service in `docker-compose.yml` runs continuously alongside `postgres` and `api`: every `BACKUP_INTERVAL_HOURS` hours (24h by default), it produces a compressed PostgreSQL dump and an archive of the documents volume into the `teslacost_backups` named volume, and deletes files older than `BACKUP_RETENTION_DAYS` days (14 by default).

```bash
# List available backups
docker compose exec backup ls -lh /backups

# Follow the backup service
docker compose logs -f backup
```

⚠️ A named Docker volume stays on the same disk as the rest of the stack: it does not protect against a disk or host failure. Regularly copy the contents of `teslacost_backups` elsewhere (a Proxmox backup job on the volume, `rsync` to another host, etc.).

### Restore

```bash
# 1. Copy a dump out of the container
docker compose cp backup:/backups/teslacost-db-<timestamp>.sql.gz .

# 2. Restore the database (overwrites the targeted database's existing data)
gunzip -c teslacost-db-<timestamp>.sql.gz | docker compose exec -T postgres psql -U "${DB_USER:-teslacost}" -d "${DB_NAME:-teslacost}"

# 3. Restore the documents into the application volume
docker compose cp backup:/backups/teslacost-documents-<timestamp>.tar.gz .
docker run --rm \
  -v teslacost_teslacost_documents:/data \
  -v "$(pwd)":/backup \
  alpine sh -c "cd /data && tar -xzf /backup/teslacost-documents-<timestamp>.tar.gz --strip-components=1"
```

### Incident diagnostics

- **Health status**: `curl http://localhost:8080/api/health` — returns `503`/`unhealthy` if the database is unreachable, `200`/`healthy` otherwise. This is also what the Docker `HEALTHCHECK` uses (`docker inspect --format='{{json .State.Health}}' teslacost-api`).
- **Application logs**: `docker compose logs -f api`. Lines prefixed `[auto-sync]`, `[sync]`, `[auth]`, `[notification]`, `[security]`, `[panic]` identify the subsystem involved.
- **TeslaMate synchronization state**: a prolonged TeslaMate API outage opens the per-vehicle circuit breaker (log `circuit breaker: OPEN`); attempts resume automatically after the cooldown (10 minutes by default) with no manual action needed.
- **Rollback**: redeploy with `TESLACOST_VERSION` pinned to the previous tag (`docker compose pull && docker compose up -d`), then, if a migration needs to be undone, manually apply the matching `.down.sql` from `migrations/` against the database.

---

## ⚙️ Environment Variables

| Variable | Description | Default |
|---|---|---|
| `PORT` | HTTP server listening port | `8080` |
| `ENVIRONMENT` | Runtime environment (`production`, `development`) — switches logs to JSON, defaults the log level to `INFO` (never `DEBUG`), enables `Secure` cookies and the insecure-defaults guard | `production` in `docker-compose.yml` |
| `LOG_LEVEL` | Forces the log level (`DEBUG`, `INFO`, `WARN`, `ERROR`), overriding the `ENVIRONMENT` default | *Optional* |
| `TESLACOST_VERSION` | Image tag to deploy (`ghcr.io/rem7474/teslacost:<tag>`); pin it in production | `latest` |
| `DATABASE_URL` | PostgreSQL connection string (`postgres://...`); an alternative to the `DB_*` variables | *Optional* |
| `JWT_SECRET` | Secret used to sign JWT tokens — **must be changed**, the default value is publicly known | *Required* |
| `JWT_ACCESS_EXPIRATION_MINUTES` | Access token lifetime | `15` |
| `JWT_REFRESH_EXPIRATION_DAYS` | Refresh token lifetime | `30` |
| `TRUSTED_PROXIES` | Addresses or CIDR ranges of the trusted reverse proxy allowed to supply `X-Forwarded-*` (`none`: trust none) | loopback + private ranges |
| `SECURITY_HEADERS` | Whether the application sends security headers | `true` |
| `CONTENT_SECURITY_POLICY` | Overrides the CSP policy (`off`: none) | built-in policy |
| `APP_ENCRYPTION_KEY` | AES-256 key encrypting TeslaMate credentials — **must be changed**, the default value is publicly known | *Required* |
| `APP_TIMEZONE` | Timezone used for computations and reporting | `Europe/Paris` |
| `STORAGE_DIR` | Directory where documents are stored on the volume | `/data/documents` |
| `DISABLE_REGISTRATION` | Disable open local account creation | `false` |
| `INITIAL_ADMIN_EMAIL` | Email of the pre-provisioned admin account | *Optional* |
| `INITIAL_ADMIN_PASSWORD` | Password of the pre-provisioned admin account | *Optional* |
| `DB_PORT_BIND` | Bind address:port of the Postgres container on the host | `127.0.0.1:5432` |
| `BACKUP_INTERVAL_HOURS` | Interval between two automatic backup cycles | `24` |
| `BACKUP_RETENTION_DAYS` | Retention period for backups before they are purged | `14` |
| `CORS_ALLOWED_ORIGINS` | Allowed CORS origins (comma-separated), only useful for a frontend served from another origin | *Optional* (the origin of `APP_BASE_URL`, plus `localhost:3000`/`5173` outside production) |

### OIDC / SSO Configuration (Optional)

| Variable | Description | Example |
|---|---|---|
| `OIDC_ISSUER_URL` | IdP issuer URL (OpenID Discovery) | `https://auth.homelab.local/application/o/autoledger/` |
| `OIDC_CLIENT_ID` | OAuth2 client identifier | `autoledger` |
| `OIDC_CLIENT_SECRET` | OAuth2 client secret | `secret_provided_by_your_idp` |
| `OIDC_REDIRECT_URL` | Registered callback redirect URL | `https://autoledger.homelab.local/api/auth/oidc/callback` |
| `OIDC_PROVIDER_NAME` | Provider name shown on the sign-in screen | `Authentik` / `Keycloak` |
| `OIDC_SCOPES` | Requested OIDC scopes (space-separated) | `openid email profile` |
| `OIDC_ALLOWED_EMAILS` | Allowlist of authorized addresses (comma-separated) | `admin@domain.com,me@domain.com` |
| `OIDC_DISABLE_LOCAL_AUTH` | Disable the local sign-in/sign-up form | `false` |

---

## 🧪 Development & Testing

```bash
# Run the backend unit tests
go test -v ./...

# Run the integration tests against a temporary database
docker run -d --name teslacost-test-pg -e POSTGRES_USER=teslacost -e POSTGRES_PASSWORD=test -e POSTGRES_DB=teslacost_test -p 55432:5432 postgres:14-alpine
TEST_DATABASE_URL="postgres://teslacost:test@localhost:55432/teslacost_test?sslmode=disable" go test -v ./internal/services/

# Build the Vue 3 frontend
cd web && npm install && npm run build
```

---

## 📄 License

This project is distributed under the [MIT](LICENSE) license.

Toll data used for detection and cost estimation (`internal/tolldata`) comes from [OpenTollData](https://github.com/louis2038/OpenTollData), licensed under [ODbL-1.0](https://opendatacommons.org/licenses/odbl/1-0/).
