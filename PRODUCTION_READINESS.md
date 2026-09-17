# Suivi — Remédiation audit production readiness

Suivi des actions issues de l'audit du 2026-09-17. Statuts : `⬜ à faire` / `🔄 en cours` / `✅ fait` / `⏭️ reporté`.

## Quick wins

| # | Action | Fichier(s) | Statut | Notes |
|---|---|---|---|---|
| 1 | `/api/health` renvoie 503 si DB déconnectée | `cmd/server/main.go` | ✅ | Ping réel du pool à chaque appel (pas juste l'état de connexion au démarrage). Testé en conteneur : DB down → 503/`unhealthy`, DB up → 200/`healthy`. |
| 2 | `HEALTHCHECK` Dockerfile + `healthcheck:` compose pour le service `api` | `Dockerfile`, `docker-compose.yml` | ✅ | `wget --spider` (busybox, déjà dans l'image alpine). Vérifié : Docker marque le conteneur `unhealthy` quand la DB est down, `healthy` sinon. |
| 3 | Driver de log `json-file` avec `max-size`/`max-file` | `docker-compose.yml` | ✅ | 10m / 3 fichiers sur `postgres` et `api`. |
| 4 | `recover()` sur les goroutines de fond | `internal/services/sync_jobs.go`, `internal/services/sync_service.go`, `cmd/server/main.go` | ✅ | Ajout d'un `recoverPanic()` partagé (`sync_jobs.go`) réutilisé dans `StartSync`, la goroutine de notification, et le worker planifié (isolé par véhicule via `runScheduledSyncSafe` pour qu'un panic sur un véhicule n'arrête pas les autres). |
| 5 | Logger les erreurs avalées silencieusement | `internal/handlers/tire_handler.go`, `internal/services/sync_service.go` | ✅ | `GetHistory` loggue désormais les 3 erreurs ignorées ; l'erreur de `CheckAndNotify` (sync_service.go) est aussi loggée au lieu d'être jetée. |
| 6 | Pin de l'image sur un tag précis plutôt que `:latest` | `docker-compose.yml`, `.env.example`, `README.md` | ✅ | Nouvelle variable `TESLACOST_VERSION` (défaut `latest` pour le quick-start, à pinner en prod). |
| 7 | Ne plus exposer le port Postgres sur toutes les interfaces par défaut | `docker-compose.yml`, `.env.example`, `README.md` | ✅ | Bind `127.0.0.1:5432` par défaut via `DB_PORT_BIND`. Vérifié avec `docker port`. |
| 8 | Scan de dépendances en CI | `.github/workflows/ci.yml` | ✅ | `govulncheck@v1.8.0` (Go, pinné) + `npm audit --audit-level=high` (frontend). Les deux passent à blanc sur l'état actuel du repo. |

Build Go, `go vet`, `go test ./...`, build Docker multi-stage et un run complet `docker compose` (DB down puis DB up) ont été exécutés pour valider ce lot — tout est vert.

## Top 5 actions bloquantes

| # | Action | Fichier(s) | Statut | Commit |
|---|---|---|---|---|
| B1 | Garde-fou démarrage : refuser de lancer en prod avec les secrets par défaut | `internal/config/config.go`, `cmd/server/main.go` | ⬜ | |
| B2 | Sauvegarde automatisée DB + documents avec procédure de restauration documentée | `docker-compose.yml`, `README.md`, script backup | ⬜ | |
| B3 | Healthcheck Docker fiable sur `api` | *(couvert par quick win #1 et #2)* | ⬜ | |
| B4 | Goroutines sans `recover()` | *(couvert par quick win #4)* | ⬜ | |
| B5 | Alerting sur erreurs critiques (sync down, circuit breaker OPEN) | `internal/services/notification_service.go`, `internal/services/sync_jobs.go` | ⬜ | |

## Autres constats (⚠️ à améliorer)

| # | Action | Fichier(s) | Statut | Commit |
|---|---|---|---|---|
| A1 | Logging structuré (`log/slog`) avec niveaux | tout le backend | ⬜ | |
| A2 | Propagation du request ID dans les logs métier | services + handlers | ⬜ | |
| A3 | Rate limiting sur `/auth/login` et `/auth/register` | `internal/handlers/auth_handler.go` | ⬜ | |
| A4 | Access token en cookie httpOnly plutôt que `localStorage` | `web/src/services/api.ts` | ⏭️ reporté (refonte du flux auth, à planifier séparément) | |
| A5 | Documenter headers de sécurité recommandés côté reverse proxy | `README.md` | ⬜ | |
| A6 | Section "Exploitation" dans le README (backup, logs, diagnostic incident) | `README.md` | ⬜ | |

---
*Ce fichier est un artefact de suivi temporaire pour la remédiation en cours ; il pourra être supprimé une fois toutes les actions traitées.*
