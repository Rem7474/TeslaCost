# Suivi — Remédiation audit production readiness

Suivi des actions issues de l'audit du 2026-09-17. Statuts : `⬜ à faire` / `🔄 en cours` / `✅ fait` / `⏭️ reporté`.

PR de suivi : [#42 — fix(ops): production-readiness quick wins + top-5 blocking actions](https://github.com/Rem7474/TeslaCost/pull/42) (mergée) et [#45 — security: access token httpOnly cookie](https://github.com/Rem7474/TeslaCost/pull/45). ✅ Tous les checks CI passent sur les deux.

Seul point encore ouvert de l'audit initial : pas de métriques `/metrics` Prometheus / dashboard Grafana (catégorie Observabilité de l'audit, jamais traitée dans un lot — à arbitrer si utile pour votre setup).

## Trouvés en corrigeant la CI (pas dans l'audit initial)

| # | Action | Fichier(s) | Statut | Notes |
|---|---|---|---|---|
| C1 | `go.mod` pinnait `go 1.26.0` (version exacte, non patchée) — le toolchain auto-résolu par CI reprenait exactement cette version avec 9 CVE stdlib connus (déjà corrigés en 1.26.4–1.26.6) | `go.mod`, `.github/workflows/ci.yml` | ✅ | Bump vers `go 1.26.8`. CI passée en `go-version-file: go.mod` pour ne plus pouvoir dériver silencieusement (elle pointait vers `1.25`, un cran en dessous de ce que `go.mod` exigeait déjà). `govulncheck` confirmé clean après coup. |
| C2 | Le conteneur `backup` tournait en `root` (défaut de l'image `postgres:16-alpine`) | `backup/Dockerfile` | ✅ | Bascule sur l'utilisateur `postgres` (uid 70) déjà créé par l'image de base, `chown` du répertoire `/backups`. Revérifié en conditions réelles : dump complet (25 tables) toujours produit correctement en non-root. Signalé par le Quality Gate SonarCloud (« B Security Rating on New Code »). |

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

| # | Action | Fichier(s) | Statut | Notes |
|---|---|---|---|---|
| B1 | Garde-fou démarrage : détecter les secrets par défaut en prod | `internal/config/config.go`, `cmd/server/main.go` | ✅ | Décision produit : **warning loggué**, ne bloque pas le démarrage (pour ne pas casser un déploiement existant). `Config.InsecureDefaults()` détecte JWT_SECRET, APP_ENCRYPTION_KEY (3 valeurs placeholder connues : config.go/docker-compose.yml/.env.example) et le mot de passe DB par défaut ; loggué uniquement si `ENVIRONMENT=production`. Testé unitairement. |
| B2 | Sauvegarde automatisée DB + documents avec procédure de restauration documentée | `backup/` (nouveau service), `docker-compose.yml`, `.env.example`, `README.md` | ✅ | Décision produit : **service cron dans docker-compose** (pas de dépendance à l'hôte Proxmox). Sidecar basé sur `postgres:16-alpine` (pg_dump de la bonne version), boucle simple (pas de crond), dump + archive documents + purge par rétention. Bug de course découvert et corrigé en testant : le service attendait seulement `postgres` healthy, pas `api` healthy, donc le tout premier backup pouvait précéder les migrations (1 table au lieu de 25) — corrigé via `depends_on: api: condition: service_healthy`. Cycle complet (backup + restauration DB + restauration documents) vérifié en conditions réelles avec docker compose. |
| B3 | Healthcheck Docker fiable sur `api` | *(couvert par quick win #1 et #2)* | ✅ | |
| B4 | Goroutines sans `recover()` | *(couvert par quick win #4)* | ✅ | |
| B5 | Alerting sur erreurs critiques (sync down, circuit breaker OPEN) | `internal/services/notification_service.go`, `internal/services/sync_jobs.go` | ✅ | Décision produit : **réutilisation du webhook véhicule existant**. Nouvelle alerte `NotifySyncCircuitOpen` envoyée une seule fois exactement au moment où le circuit breaker d'un véhicule bascule OPEN (pas à chaque échec suivant). L'alerte globale sur panne DB elle-même n'est **pas** couverte par ce mécanisme : si la DB est down, l'app ne peut pas lire la config webhook en base pour alerter — ce cas relève du monitoring externe sur `/api/health` (documenté dans la nouvelle section README « Exploitation »), pas d'un webhook applicatif. Testé en intégration avec une vraie base Postgres (`TestRecordSyncFailureAlertsOnlyOnTransitionToOpen`) : exactement 1 appel webhook sur 3 échecs consécutifs. |

## Autres constats (⚠️ à améliorer)

| # | Action | Fichier(s) | Statut | Commit |
|---|---|---|---|---|
| A1 | Logging structuré (`log/slog`) avec niveaux | tout le backend (9 fichiers) | ✅ | JSON en prod, texte lisible sinon ; niveau par défaut `INFO` en prod / `DEBUG` en dev, override via `LOG_LEVEL`. **Bug découvert en testant** : `docker-compose.yml` ne positionnait jamais `ENVIRONMENT`, et `.env.example` le mettait explicitement à `development` — donc ni les logs JSON, ni le garde-fou B1, ni les cookies `Secure` ne s'activaient jamais en suivant le README. Corrigé : `ENVIRONMENT: ${ENVIRONMENT:-production}` dans `docker-compose.yml`, override `development` dans `docker-compose.dev.yml`, `.env.example` mis à `production`. **Bug de doc découvert en même temps** : le tableau README référençait `APP_ENV`/`ENCRYPTION_KEY`, des noms qui n'existent pas dans le code (les vraies variables sont `ENVIRONMENT`/`APP_ENCRYPTION_KEY`) — corrigé. Vérifié en conditions réelles : `docker compose up` avec `.env.example` non modifié produit désormais des logs JSON et affiche les 3 avertissements de sécurité dès le démarrage. Portée : les logs applicatifs (`log.*` → `slog.*`) ; le logger d'accès HTTP de chi (`chiMiddleware.Logger`) n'a pas été touché (déjà structuré ligne par ligne, non-JSON). |
| A2 | Propagation du request ID dans les logs métier | `cmd/server/main.go`, `internal/handlers/validation.go` (+ 10 handlers), `idempotency.go`, `tire_handler.go` | ✅ | `requestIDHandler` (wrapper `slog.Handler`) attache automatiquement le `request_id` chi à tout appel `slog.*Context(r.Context(), ...)`. `writeRepoError` (80 points d'appel, remplacement mécanique) et les logs handler-level de `idempotency.go`/`tire_handler.go` utilisent désormais `ErrorContext`. Portée volontairement limitée aux logs déclenchés depuis une requête HTTP — les jobs de fond (sync, notifications) n'ont pas de requête à corréler. Testé unitairement (`cmd/server/main_test.go`). |
| A3 | Rate limiting sur `/auth/login` et `/auth/register` | `cmd/server/main.go`, `go.mod` | ✅ | `github.com/go-chi/httprate`, 10 requêtes/minute par IP, appliqué indépendamment à chaque route (compteurs séparés). Vérifié en conditions réelles : 11 tentatives de login consécutives → `429` à la 11ᵉ, `/register` non affecté par les tentatives sur `/login`. `govulncheck` clean sur la nouvelle dépendance. |
| A4 | Access token en cookie httpOnly plutôt que `localStorage` | `internal/handlers/auth_handler.go`, `internal/middleware/auth_middleware.go`, `web/src/services/api.ts`, `web/src/stores/auth.ts`, `web/src/router/index.ts` | ✅ | PR [#45](https://github.com/Rem7474/TeslaCost/pull/45). Nouveau cookie httpOnly `teslacost_access_token` (même schéma que le refresh token). **Bug préexistant trouvé au passage** : `cfg.CookieSecure` était calculé mais jamais lu — tous les cookies avaient `Secure: true` en dur, cassant les sessions en HTTP simple sur tout hôte hors `localhost` littéral. Corrigé sur les 7 points de pose de cookie du fichier (refresh, access, OIDC state/nonce). Le SPA ne pouvant plus lire son état de session de façon synchrone (plus de `localStorage`), le garde de route attend désormais une vérification `/auth/me` avant sa première décision par chargement de page. Vérifié sur la stack réelle : cookies posés à l'inscription, endpoint protégé accessible cookie seul (aucun header `Authorization`), rotation au refresh, effacement au logout, et `CookieSecure=false` produit bien des cookies non-`Secure` en dev. |
| A5 | Documenter headers de sécurité recommandés côté reverse proxy | `README.md` | ✅ | Nouvelle section « Exposition sur Internet » avec exemples Caddy et Traefik (HSTS, X-Content-Type-Options, X-Frame-Options, Referrer-Policy). **Bug de doc trouvé au passage** : l'exemple `docker run` de l'Option 2 utilisait aussi `ENCRYPTION_KEY` au lieu de `APP_ENCRYPTION_KEY` (même bug que le tableau des variables, corrigé dans le lot précédent) — corrigé ici aussi. |
| A6 | Section "Exploitation" dans le README (backup, logs, diagnostic incident) | `README.md` | ✅ | Ajoutée en même temps que B2 (sauvegardes, restauration, diagnostic, rollback). |

---
*Ce fichier est un artefact de suivi temporaire pour la remédiation en cours ; il pourra être supprimé une fois toutes les actions traitées.*
