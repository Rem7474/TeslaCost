# TeslaCost

Self-hosted total-cost-of-ownership tracker for cars: energy/fuel, maintenance, tires, documents, reminders, carpooling, financing. Vehicles can be fed automatically by a [TeslaMate](https://github.com/teslamate-org/teslamate) instance through `teslamateapi`, or tracked entirely by hand.

- Backend: Go (`go.mod` pins the toolchain), chi router, pgx/pgxpool, PostgreSQL. Module path `github.com/teslacost/teslacost`.
- Frontend: Vue 3 `<script setup>` + TypeScript, Pinia, vue-router, Tailwind 4, Chart.js, Vite. Built into `web/dist` and embedded in the Go binary (`web/embed.go`).
- One Docker image serves API + SPA. `docker-compose.yml` runs `postgres`, `api` and `backup`.
- The user-facing docs (`README.md`) and UI strings are in French for now.

## Commands

```bash
# Backend
go build ./... && go vet ./...
gofmt -l .                       # must print nothing (run it on files you touch; some drift)
go test ./...                    # unit tests; integration tests are skipped without TEST_DATABASE_URL
make dev                         # go run ./cmd/server/main.go (needs DATABASE_URL + secrets, see .env.example)

# Frontend (from web/)
npm ci --ignore-scripts
npm run typecheck                # vue-tsc --noEmit
npm test                         # vitest, node environment: utils/composables only, no DOM
npm run build                    # typecheck + vite build; the Go build embeds web/dist
```

Integration tests (services, handlers, database) need PostgreSQL. Use your own container name, other projects run containers on the same host:

```bash
docker run -d --rm --name tc-pg -e POSTGRES_USER=teslacost -e POSTGRES_PASSWORD=test \
  -e POSTGRES_DB=teslacost_test -p 55433:5432 postgres:16-alpine
# wait until a TCP connection works, pg_isready alone is not enough (psql is not installed on the host):
until docker exec -e PGPASSWORD=test tc-pg psql -h 127.0.0.1 -U teslacost -d teslacost_test -c 'select 1' >/dev/null 2>&1; do sleep 1; done
TEST_DATABASE_URL='postgres://teslacost:test@localhost:55433/teslacost_test?sslmode=disable' go test -timeout 20m ./...
docker rm -f tc-pg
```

The full run takes several minutes (`internal/services` ~5 min). A `go test` that hangs on pgxpool acquire means the database was not really up: recreate the container. CI runs `go test -race` on postgres:14, `govulncheck`, `npm audit --audit-level=high`, and SonarCloud.

## Layout

```
cmd/server/main.go          wiring, chi routes, background workers
internal/config             env-var loading and validation (Config)
internal/auth               JWT, bcrypt, OIDC client, login throttle
internal/middleware         auth, client IP / trusted proxies, security headers, Origin check
internal/crypto             AES-256-GCM for stored credentials
internal/models             plain structs and request types, grouped by domain (models_vehicle.go, ...)
internal/database           pgx pool, embedded migrations runner, one repository_*.go per domain
internal/handlers           HTTP handlers, one file per resource
internal/services           TCO engine (tco_*.go), sync (sync_*.go), energy stats, carpool, comparison, reminders/notifications, toll detection
internal/teslamate          HTTP client for teslamateapi
internal/storage            document files on the volume
internal/money, tolldata    integer-cent helpers, toll reference data
migrations/                 NNNNNN_name.{up,down}.sql, embedded via migrations/embed.go
web/src/views               one orchestration view per route (kept thin)
web/src/components/<area>   modals, panels and cards extracted from the views
web/src/utils               pure logic + colocated *.test.ts (this is what vitest covers)
web/src/composables         shared reactive logic (document preview/attach, selection, confirm)
web/src/stores              Pinia: auth, vehicle, offline queue, quick-add
web/src/services            api.ts (fetch wrapper, refresh handling), offlineQueue.ts (IndexedDB)
backup/                     sidecar image: pg_dump + documents archive on a schedule
```

## Domain notes

- Money is stored and computed in integer cents (`internal/money`); convert at the edges only.
- A vehicle has a `Powertrain` (`EV` or `ICE`). ICE vehicles are tracked through fuel logs and have no TeslaMate link. EV vehicles may or may not have a TeslaMate connection (`teslamate_api_url`, `teslamate_car_id`, credentials stored encrypted). TeslaMate-fed features (drives, energy/battery/temperature panels, sync button, Tesla tire presets) are shown only when `hasTeslaMate` is true (`web/src/utils/vehicles.ts`, exposed by the vehicle store): electric and a teslamateapi URL set. New UI that depends on TeslaMate data must use it.
- `cost_ledger` is the single view the TCO engine reads; charges, fuel, tolls, maintenance, insurance, tire amortization and acquisition cost all flow into it.
- TeslaMate sync (`internal/services/sync_*.go`): per-vehicle background jobs, resumable full import, a 30-day sliding re-read, reconciliation of deleted drives/charges, and a per-vehicle circuit breaker. Changing what is read from TeslaMate means resetting `sync_state.full_import_completed_at` in a migration so the next sync re-reads the history.
- teslamateapi facts (verified in its source, its README has no field docs): drives/charges carry `battery_details{start_battery_level,end_battery_level}` and `outside_temp_avg` in `units.unit_of_temperature` (F is possible); `charge_energy_used` is `GREATEST(used, added)`, so used == added means "not measured" (DC); missing levels arrive as 0; `/battery-health` reports current capacity over the best capacity ever observed.
- Manual charges require a cost. `UpdateCharge` overwrites notes/`document_id` with what it receives: send the existing values back when only completing a cost. Foreign currency (`fx_rate`) exists only on manual entries.
- Sessions: 15-minute JWT access token, 30-day rotating refresh token in an `HttpOnly` cookie with reuse detection. With `ENVIRONMENT=production` the server refuses to start on published default secrets.

## Conventions

Backend
- Handlers are thin: decode, validate (`validation.go`), check ownership/role (`ownership.go`), call a repository or service, write with `response.go`. Errors returned to clients are generic; details go to logs with a subsystem prefix (`[sync]`, `[auth]`, ...).
- Every repository query that touches user data is scoped by ownership; new endpoints need an ownership test.
- Migrations: next number after the last file, always with a `.down.sql`, each applied in its own transaction. Do not edit an applied migration. `sonar.cpd.exclusions` already excludes `migrations/`.
- `internal/handlers/auth_handler.go` uses CRLF line endings: edit it with a tool that preserves them (Python `newline=''`), or the whole file shows as changed.
- Keep files focused (the large ones were split by responsibility; do not regrow them). New behavior gets a test next to it; integration tests use `TEST_DATABASE_URL` and skip otherwise.

Frontend
- Views orchestrate; anything with its own form or API call is a component under `components/<area>/`. Modals use `defineModel('open')`, seed their form in `watch(open)` and emit `saved`.
- Put logic in `utils/*.ts` with a test, not in the component. Shared date helpers live in `utils/dates.ts`.
- `<table class="sr-only">` does not clip: wrap it in `<div class="sr-only">`. After changing a page, check that `documentElement.scrollWidth` equals the viewport at 320/360/375/390/414 px.
- Do not pipe `vite build` through `tail` and trust the result: a failed build leaves the previous `dist`. Grep for `built|error`.

Refactors
- Splitting a file is a pure move unless stated otherwise: verify declaration hashes and the multiset of non-blank lines before and after, then `gofmt`, build, vet, tests. For Vue views, compare the rendered DOM and recorded API writes before and after with a Playwright characterization run (mock `/api/**`, fixed clock).
- SonarCloud counts moved code as new code: duplication that already existed inside a big file fails `new_duplicated_lines_density` (3 %) once the file is split. Find the blocks with `api/duplications/show?key=Rem7474_TeslaCost:<path>&pullRequest=<n>` and extract a small helper in a separate commit.

## Git and PRs

- Check `gh pr list --state all` before touching a branch that had a PR: PRs are merged quickly and a merged branch must not be reused. Follow-up work goes on a fresh branch from `origin/main`, one PR per topic, independent PRs rather than stacks unless the work truly depends on the previous one.
- Commit messages and PR titles/bodies are in English. PR body: `## Summary` and `## Notes` sections. `gh pr edit` fails on the deprecated Projects-classic GraphQL field; update a body with `gh api -X PATCH repos/Rem7474/TeslaCost/pulls/<n> -F body=@file`.
- Do not push local `feat/*` branches left over from merged PRs.
- Documentation states the current behavior; it does not narrate history ("now", "again", "re-introduced") or cite PR numbers. That belongs in commit messages.

## Browser checks

Playwright and Chromium are available on the dev host (`PLAYWRIGHT_BROWSERS_PATH=/opt/pw-browsers` in the remote environment). Run `vite preview` bound to 127.0.0.1, mock `/api/**` with `context.route`, and never `pkill -f` a pattern that also appears in your own command line.
