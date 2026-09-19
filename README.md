# TeslaCost 🚗⚡

> Application auto-hébergée open-source de suivi complet du coût de possession (TCO), de l'entretien, des pneus et du covoiturage automobile, avec synchronisation [TeslaMate](https://github.com/teslamate-org/teslamate) en temps réel ou fonctionnement 100% autonome.

[![CI / CD Pipeline](https://github.com/Rem7474/TeslaCost/actions/workflows/ci.yml/badge.svg)](https://github.com/Rem7474/TeslaCost/actions/workflows/ci.yml)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=Rem7474_TeslaCost&metric=alert_status)](https://sonarcloud.io/summary/new_code?id=Rem7474_TeslaCost)
[![Docker Image](https://img.shields.io/badge/docker-ghcr.io-blue?logo=docker)](https://github.com/Rem7474/TeslaCost/pkgs/container/teslacost)

---

## 🌟 Points forts

- **⚡ Synchronisation TeslaMate résiliente** : récupération de l'odomètre en temps réel, import complet avec reprise sur interruption, relecture des coûts récents et détection des trajets/charges supprimés.
- **📊 Calculateur TCO au centime près** : registre unique des coûts (`cost_ledger`), gestion du financement (comptant, crédit avec tableau d'amortissement, LOA, LLD), décote réelle et provision de restitution.
- **🔐 Authentification hybride (JWT local & OIDC SSO)** : support natif d'Authentik, Keycloak, Authelia et Kanidm via Authorization Code Flow sécurisé, JIT provisioning et fallback local.
- **🔔 Moteur de rappels d'entretien & Webhooks** : alertes d'échéance par date et/ou kilométrage avec connecteurs sortants vers Telegram, Discord, Gotify et Webhook JSON générique.
- **📁 Gestionnaire de documents & factures** : stockage sécurisé des pièces jointes (PDF, photos) sur volume Docker dédié avec contrôle d'accès applicatif strict.
- **🛞 Gestion du cycle de vie des pneus** : suivi des sculptures, sessions de permutation transactionnelles, exclusion des périodes de stockage et projection kilométrique restante.
- **👥 Module de covoiturage équitable** : synchronisation automatique des dates de trajet, recalcul des coûts réels en lot, prix d'électricité pondéré sur les charges récentes et quote-part d'assurance au jour/km.
- **📱 PWA, saisie rapide & mode hors-connexion** : bouton « + » (recharge, plein, péage/parking) dans la barre du bas, coût des recharges TeslaMate à compléter en un champ, photo du ticket depuis l'appareil, et saisie sans réseau avec file d'attente IndexedDB et déduplication par clé d'idempotence.

---

## 🚀 Fonctionnalités détaillées

### 1. Authentification & Sécurité
- **Authentification locale** : inscription/connexion par mot de passe (bcrypt, 72 octets maximum), jeton d'accès JWT HS256 de 15 minutes et jeton de rafraîchissement rotatif de 30 jours, ce dernier uniquement dans un cookie `HttpOnly` (jamais dans le corps des réponses). Un compte est fermé aux tentatives après 10 échecs en 15 minutes ; un e-mail inconnu et un mot de passe erroné reçoivent la même réponse dans le même délai.
- **Support OIDC / OAuth2 (SSO)** : délégation d'authentification à votre IdP homelab (Authentik, Keycloak, Authelia, Kanidm).
  - Flux standard *Authorization Code Flow* avec PKCE (S256), vérification anti-CSRF (`state`) et anti-rejeu (`nonce` chiffré SHA-256).
  - Just-In-Time (JIT) provisioning : création automatique du compte ou liaison avec un compte local partageant le même e-mail. L'adresse ne doit pas être signalée non vérifiée par le fournisseur (`email_verified: false` est refusé), et seul un compte local qui n'a pas encore d'identité SSO est lié : un compte déjà lié n'est jamais rattaché à une autre identité.
  - Whitelist optionnelle (`OIDC_ALLOWED_EMAILS`, sans distinction de casse) et désactivation possible de l'authentification locale (`OIDC_DISABLE_LOCAL_AUTH`).
- **Chiffrement au repos** : chiffrement symétrique AES-256-GCM des identifiants et tokens de connexion TeslaMate dans PostgreSQL.
- **Rate limiting** : `/api/auth/login` et `/api/auth/register` sont limités à 10 tentatives/minute par adresse client (voir `TRUSTED_PROXIES` derrière un reverse proxy), `/api/auth/refresh` et les routes SSO à des seuils plus larges.

### 2. Synchronisation TeslaMate API
- **Odomètre temps réel** : actualisation en direct de l'odomètre du véhicule dès que TeslaMate le remonte.
- **Tâches en arrière-plan** : synchronisation non bloquante avec suivi de progression et exclusion mutuelle par véhicule.
- **Reprise après coupure** : importation de l'historique des trajets (`/drives`) et recharges (`/charges`) reprenant automatiquement là où elle s'est arrêtée.
- **Relecture complète après enrichissement** : quand une migration ajoute des données issues de TeslaMate (niveaux de batterie, température), l'état d'import est réinitialisé et la synchronisation suivante relit tout l'historique. Les coûts saisis à la main sont conservés.
- **Relecture glissante (30 jours)** : mise à jour automatique des coûts de recharge complétés a posteriori dans TeslaMate.
- **Recharges manuelles & sans coût** : signalement clair des recharges sans tarif (jamais 0 € imposé) et saisie possible des recharges hors suivi (ex: prise domestique chez un tiers).
- **Contrôle d'intégrité** : vérification de continuité de l'odomètre (trous, reculs, écarts de distance) et seuil de sécurité sur la suppression massive de données.

### 3. Calculateur de TCO & Financement Automobile
- **Registre des coûts unifié (`cost_ledger`)** : centralisation des recharges, péages, dépenses d'entretien, primes d'assurance, amortissement des pneus et coût d'acquisition.
- **Modes d'acquisition supportés** :
  - *Comptant* : décote linéaire basée sur l'estimation de revente ou le montant de vente réel à la cession du véhicule.
  - *Crédit classique* : tableau d'amortissement mois par mois, dissociation capital / intérêts, frais de dossier et assurance emprunteur.
  - *LOA & LLD* : prise en compte de l'apport initial, des loyers mensuels, du dépôt de garantie, du forfait kilométrique contractuel et provision mensuelle pour dépassement ou frais de remise en état.
- **Indicateurs financiers avancés** : coût d'usage au km (énergie + péages), coût complet au km, coût net des recettes de covoiturage et score de complétude du TCO.
- **Efficacité énergétique** (véhicules électriques) : consommation réelle en kWh/100 km, coût de l'énergie aux 100 km (mensuel et moyenne sur 3 mois), rendement de charge (énergie stockée sur énergie tirée du réseau) et répartition des recharges entre prise domestique, AC et DC d'après leur puissance moyenne. Endpoint `GET /api/vehicles/{id}/energy-stats`.
- **Batterie et température** : niveau de batterie de début et de fin de chaque trajet et recharge, température extérieure moyenne (convertie en °C quelle que soit l'unité de TeslaMate), capacité utile estimée d'après l'énergie ajoutée et le pourcentage gagné, coût d'une charge de 0 à 100 % par type de recharge, et surconsommation par temps froid comparée au temps doux. La santé de la batterie calculée par TeslaMate (`/battery-health`, versions récentes de TeslaMateApi) est relevée à chaque synchronisation, une fois par jour, pour tracer son évolution.

### 4. Rappels d'entretien & Notifications Homelab
- **Double condition de déclenchement** : surveillance combinée de la date d'échéance et/ou du seuil kilométrique calculé sur l'odomètre réel.
- **Seuils d'anticipation configurables** : notification préalable avant le dépassement critique (ex: avertir 500 km ou 15 jours avant).
- **Connecteurs de notification webhook** :
  - **Discord** : envoi de messages avec embed enrichi (couleurs de statut, champs organisés).
  - **Telegram** : messages formatés Markdown via bot HTTP.
  - **Gotify** : notifications push auto-hébergées avec gestion des priorités.
  - **JSON Générique** : intégration directe avec Home Assistant, Node-RED ou n8n.
- **Alerte de synchronisation en échec** : le même webhook véhicule est aussi utilisé pour prévenir quand la synchronisation TeslaMate échoue de façon répétée et est automatiquement suspendue (circuit breaker), sans attendre d'ouvrir l'application.

### 5. Archivage & Gestion des Documents
- **Stockage sur volume filesystem** : migration des pièces jointes (factures d'entretien, justificatifs) hors de PostgreSQL vers un volume dédié (`/data/documents`).
- **Sécurité des fichiers** : isolation non-root (`teslacost`), aucun accès HTTP statique direct, contrôle strict par token JWT et prévention contre les attaques de type path-traversal.

### 6. Gestion du Cycle de Vie des Pneus
- **Fiches complètes** : marque, modèle, dimensions, indice de charge/vitesse, saison (été/hiver/4 saisons), prix d'achat et code DOT.
- **Suivi des sculptures** : relevés d'usure millimétriques par pneu avec projection automatique de l'usure kilométrique restante (les périodes passées en stockage sont automatiquement exclues du calcul).
- **Sessions de permutation** : montages et démontages groupés sur les essieux (`FL`, `FR`, `RL`, `RR`, `STORAGE`, `DISPOSED`) avec historique chronologique.

### 7. Module de Covoiturage (BlaBlaCar & Directs)
- **Découpage en étapes** : association avec les trajets réels TeslaMate ou saisie manuelle.
- **Synchronisation des dates & recalcul en lot** : réajustement automatique de la date du covoiturage sur les trajets réels et recalcul immédiat des parts de chaque passager.
- **Énergie & assurance au plus juste** : calcul du tarif de l'électricité basé sur la moyenne pondérée des charges récentes (fenêtre de 5 jours) et prorata exact de l'assurance au kilomètre parcouru dans la journée.

### 8. Recherche & Navigation des Trajets
- **Filtres temporels** : sélection par mois, année, période personnalisée ou affichage complet.
- **Recherche plein texte** : filtrage par adresses de départ et d'arrivée.
- **File de qualification** : détection automatique des trajets autoroutiers nécessitant la qualification ou la vérification d'un péage.

---

## 📁 Arborescence du Projet

```text
TeslaCost/
├── cmd/
│   └── server/
│       └── main.go                 # Point d'entrée de l'application & routage Chi
├── internal/
│   ├── auth/                       # Hachage bcrypt, tokens JWT et service client OIDC
│   ├── config/                     # Chargement et validation des variables d'environnement
│   ├── crypto/                     # Chiffrement symétrique AES-256-GCM
│   ├── database/                   # Pool pgx, migrations SQL et couche repository
│   ├── handlers/                   # Contrôleurs HTTP REST Chi
│   ├── middleware/                 # Authentification JWT, CORS, logger
│   ├── models/                     # Modèles de données Go fortement typés
│   ├── services/                   # Moteur TCO, usure pneus, covoiturage, notifications
│   ├── storage/                    # Service de stockage des documents sur volume Docker
│   └── teslamate/                  # Client de communication avec teslamateapi
├── migrations/                     # Schémas et migrations SQL PostgreSQL versionnés
├── web/                            # Frontend SPA Vue 3 + Vite + Tailwind CSS + PWA
├── docker-compose.yml              # Configuration de la stack conteneurisée
├── Dockerfile                      # Build multi-stage (Vue 3 + binaire statique Go)
└── .env.example                    # Modèle des variables de configuration
```

---

## 🛠️ Déploiement & Installation

### Option 1 : Docker Compose (Recommandé)

1. **Créez un répertoire et téléchargez les fichiers de configuration :**
   ```bash
   mkdir teslacost && cd teslacost
   curl -O https://raw.githubusercontent.com/Rem7474/TeslaCost/main/docker-compose.yml
   curl -o .env https://raw.githubusercontent.com/Rem7474/TeslaCost/main/.env.example
   ```

2. **Générez votre clé de chiffrement et configurez `.env` :**
   ```bash
   # Générer une clé de chiffrement AES-256 de 32 octets (64 caractères hexadécimaux) :
   openssl rand -hex 32
   ```
   Renseignez vos clés dans le fichier `.env` :
   - `APP_ENCRYPTION_KEY` : la clé de chiffrement générée
   - `DB_PASSWORD` : mot de passe de la base de données
   - `JWT_SECRET` : secret de signature des sessions
   - *(Optionnel)* La section OIDC / SSO si vous déléguez l'authentification à votre IdP

3. **Lancez la stack :**
   ```bash
   docker compose up -d
   ```

L'application est disponible sur **`http://localhost:8080`**.

> Le fichier [`docker-compose.yml`](./docker-compose.yml) du repo (celui téléchargé à l'étape 1) fait foi ; il inclut les services `postgres`, `api` et `backup` (sauvegardes automatiques, voir la section « Exploitation » plus bas) avec leurs healthchecks respectifs.

---


### Option 2 : Image Docker officielle (GHCR)

L'image Docker multi-architecture (`linux/amd64`, `linux/arm64`) est publiée automatiquement sur GitHub Container Registry :

```bash
docker pull ghcr.io/rem7474/teslacost:latest
```

Exemple d'exécution autonome avec un PostgreSQL externe :

```bash
docker run -d \
  --name teslacost \
  -p 8080:8080 \
  -v teslacost_docs:/data/documents \
  -e ENVIRONMENT="production" \
  -e DATABASE_URL="postgres://user:password@postgres-host:5432/teslacost?sslmode=disable" \
  -e JWT_SECRET="votre_clef_secrete_jwt_robuste" \
  -e APP_ENCRYPTION_KEY="clef_hexadecimale_de_64_caracteres_exactement" \
  -e APP_TIMEZONE="Europe/Paris" \
  ghcr.io/rem7474/teslacost:latest
```

---

### Exposition sur Internet : reverse proxy & sécurité

TeslaCost ne termine pas le TLS : placez-le derrière un reverse proxy qui le gère (Caddy ou Traefik avec Let's Encrypt automatique, ou Nginx avec un certificat existant). Le reste est porté par l'application.

- **Secrets** : avec `ENVIRONMENT=production` (valeur par défaut de `docker-compose.yml`), le serveur refuse de démarrer tant que `JWT_SECRET`, `APP_ENCRYPTION_KEY` ou `DB_PASSWORD` ont une valeur publiée dans le dépôt. Le message d'erreur nomme la variable à changer. `ENVIRONMENT=development` conserve les valeurs d'exemple pour un essai local.
- **Adresse du client** : `X-Forwarded-For` et `X-Forwarded-Proto` ne sont crus que si la connexion vient d'un proxy de confiance (`TRUSTED_PROXIES`, par défaut le loopback et les plages privées : réseau Docker, LAN). La chaîne est lue de droite à gauche : les entrées ajoutées par le client ne sont jamais utilisées. Le limiteur de tentatives de connexion et les sessions enregistrent cette adresse. Un proxy sur une adresse publique doit être listé ; `TRUSTED_PROXIES=none` n'en approuve aucun.
- **En-têtes de sécurité** posés par l'application : `Content-Security-Policy` (ressources du même domaine, aperçus par `blob:`), `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy`, `Permissions-Policy`, `Cross-Origin-Opener-Policy`, et `Strict-Transport-Security` uniquement sur les requêtes HTTPS. `SECURITY_HEADERS=false` les désactive si votre proxy les pose déjà ; `CONTENT_SECURITY_POLICY` remplace la politique (`off` supprime seulement celle-ci).
- **Requêtes entre sites** : une requête qui modifie des données et dont l'en-tête `Origin` n'est ni `APP_BASE_URL`, ni une origine de `CORS_ALLOWED_ORIGINS`, ni l'hôte demandé est refusée (403). Les requêtes portant un en-tête `Authorization` ou sans `Origin` (scripts, `curl`) ne sont pas concernées.
- **Port de l'application** : ne publiez pas le port 8080 sur Internet ; seul le proxy doit l'atteindre (`PORT` et le réseau Docker). Une connexion directe de l'extérieur qui apparaîtrait depuis une adresse privée (NAT Docker sans conservation de l'IP source) serait traitée comme un proxy de confiance.
- Les sessions utilisent un jeton d'accès de 15 minutes (`JWT_ACCESS_EXPIRATION_MINUTES`) renouvelé par un jeton de rafraîchissement rotatif de 30 jours (`JWT_REFRESH_EXPIRATION_DAYS`).

Exemple avec **Caddy** (`Caddyfile`) :

```caddyfile
teslacost.homelab.local {
    reverse_proxy localhost:8080
}
```

Pensez à ajuster `APP_BASE_URL` et `CORS_ALLOWED_ORIGINS` pour qu'ils reflètent le nom de domaine public utilisé, et à laisser `COOKIE_SECURE` sur sa valeur par défaut (activée automatiquement dès que `APP_BASE_URL` commence par `https://` ou que `ENVIRONMENT=production`).

---

## 🛟 Exploitation : sauvegardes, restauration & diagnostic

### Sauvegardes automatiques

Le service `backup` de `docker-compose.yml` tourne en continu à côté de `postgres` et `api` : toutes les `BACKUP_INTERVAL_HOURS` heures (24h par défaut), il produit un dump PostgreSQL compressé et une archive du volume de documents dans le volume nommé `teslacost_backups`, et supprime les fichiers plus vieux que `BACKUP_RETENTION_DAYS` jours (14 par défaut).

```bash
# Lister les sauvegardes disponibles
docker compose exec backup ls -lh /backups

# Suivre le service de sauvegarde
docker compose logs -f backup
```

⚠️ Un volume Docker nommé reste sur le même disque que le reste de la stack : il ne protège pas contre une panne du disque ou de l'hôte Proxmox. Copiez régulièrement le contenu de `teslacost_backups` ailleurs (job de backup Proxmox sur le volume, `rsync` vers un autre hôte, etc.).

### Restauration

```bash
# 1. Copier un dump hors du conteneur
docker compose cp backup:/backups/teslacost-db-<horodatage>.sql.gz .

# 2. Restaurer la base (écrase les données existantes de la base ciblée)
gunzip -c teslacost-db-<horodatage>.sql.gz | docker compose exec -T postgres psql -U "${DB_USER:-teslacost}" -d "${DB_NAME:-teslacost}"

# 3. Restaurer les documents dans le volume applicatif
docker compose cp backup:/backups/teslacost-documents-<horodatage>.tar.gz .
docker run --rm \
  -v teslacost_teslacost_documents:/data \
  -v "$(pwd)":/backup \
  alpine sh -c "cd /data && tar -xzf /backup/teslacost-documents-<horodatage>.tar.gz --strip-components=1"
```

### Diagnostic d'incident

- **État de santé** : `curl http://localhost:8080/api/health` — renvoie `503`/`unhealthy` si la base est injoignable, `200`/`healthy` sinon. C'est aussi ce qu'utilise le `HEALTHCHECK` Docker (`docker inspect --format='{{json .State.Health}}' teslacost-api`).
- **Logs applicatifs** : `docker compose logs -f api`. Les lignes préfixées `[auto-sync]`, `[sync]`, `[auth]`, `[notification]`, `[security]`, `[panic]` identifient le sous-système concerné.
- **État de la synchronisation TeslaMate** : une panne prolongée de l'API TeslaMate ouvre le circuit breaker par véhicule (log `circuit breaker: OPEN`) ; les tentatives reprennent automatiquement après le cooldown (10 minutes par défaut) sans action manuelle.
- **Rollback** : redéployer avec `TESLACOST_VERSION` pointé sur le tag précédent (`docker compose pull && docker compose up -d`), puis si une migration doit être défaite, appliquer le `.down.sql` correspondant dans `migrations/` manuellement contre la base.

---

## ⚙️ Variables d'Environnement

| Variable | Description | Valeur par défaut |
|---|---|---|
| `PORT` | Port d'écoute du serveur HTTP | `8080` |
| `ENVIRONMENT` | Environnement d'exécution (`production`, `development`) — active les logs JSON, le niveau `INFO` par défaut (jamais `DEBUG`), les cookies `Secure` et le garde-fou sur les secrets par défaut | `production` dans `docker-compose.yml` |
| `LOG_LEVEL` | Force le niveau de log (`DEBUG`, `INFO`, `WARN`, `ERROR`), remplace le défaut lié à `ENVIRONMENT` | *Optionnel* |
| `TESLACOST_VERSION` | Tag d'image à déployer (`ghcr.io/rem7474/teslacost:<tag>`) ; à pinner en production | `latest` |
| `DATABASE_URL` | Chaîne de connexion PostgreSQL (`postgres://...`) ; alternative aux variables `DB_*` | *Optionnel* |
| `JWT_SECRET` | Secret de signature des jetons JWT — **à changer impérativement**, la valeur par défaut est connue publiquement | *Obligatoire* |
| `JWT_ACCESS_EXPIRATION_MINUTES` | Durée de validité du jeton d'accès | `15` |
| `JWT_REFRESH_EXPIRATION_DAYS` | Durée de validité du jeton de rafraîchissement | `30` |
| `TRUSTED_PROXIES` | Adresses ou plages CIDR du reverse proxy autorisé à fournir `X-Forwarded-*` (`none` : aucun) | loopback + plages privées |
| `SECURITY_HEADERS` | Envoi des en-têtes de sécurité par l'application | `true` |
| `CONTENT_SECURITY_POLICY` | Remplace la politique CSP (`off` : aucune) | politique intégrée |
| `APP_ENCRYPTION_KEY` | Clé de chiffrement AES-256 des identifiants TeslaMate — **à changer impérativement**, la valeur par défaut est connue publiquement | *Obligatoire* |
| `APP_TIMEZONE` | Fuseau horaire de calcul et reporting | `Europe/Paris` |
| `STORAGE_DIR` | Répertoire de stockage des documents sur le volume | `/data/documents` |
| `DISABLE_REGISTRATION` | Désactiver la création libre de compte local | `false` |
| `INITIAL_ADMIN_EMAIL` | Email de l'administrateur pré-initialisé | *Optionnel* |
| `INITIAL_ADMIN_PASSWORD` | Mot de passe de l'administrateur pré-initialisé | *Optionnel* |
| `DB_PORT_BIND` | Adresse:port de liaison du conteneur Postgres sur l'hôte | `127.0.0.1:5432` |
| `BACKUP_INTERVAL_HOURS` | Intervalle entre deux cycles de sauvegarde automatique | `24` |
| `BACKUP_RETENTION_DAYS` | Durée de rétention des sauvegardes avant purge | `14` |
| `CORS_ALLOWED_ORIGINS` | Origines CORS autorisées (séparées par virgule), uniquement utile pour un frontend servi depuis une autre origine | *Optionnel* (origine de `APP_BASE_URL`, plus `localhost:3000`/`5173` hors production) |

### Configuration OIDC / SSO (Optionnel)

| Variable | Description | Exemple |
|---|---|---|
| `OIDC_ISSUER_URL` | URL de l'émetteur IdP (OpenID Discovery) | `https://auth.homelab.local/application/o/teslacost/` |
| `OIDC_CLIENT_ID` | Identifiant du client OAuth2 | `teslacost` |
| `OIDC_CLIENT_SECRET` | Secret du client OAuth2 | `secret_fourni_par_votre_idp` |
| `OIDC_REDIRECT_URL` | URL de redirection callback enregistrée | `https://teslacost.homelab.local/api/auth/oidc/callback` |
| `OIDC_PROVIDER_NAME` | Nom du fournisseur affiché sur l'écran de connexion | `Authentik` / `Keycloak` |
| `OIDC_SCOPES` | Scopes OIDC demandés (séparés par un espace) | `openid email profile` |
| `OIDC_ALLOWED_EMAILS` | Whitelist des adresses autorisées (séparées par virgule) | `admin@domaine.fr,moi@domaine.fr` |
| `OIDC_DISABLE_LOCAL_AUTH` | Désactiver le formulaire de connexion/inscription local | `false` |

---

## 🧪 Développement & Tests

```bash
# Lancer les tests unitaires du backend
go test -v ./...

# Lancer les tests d'intégration avec une base de données temporaire
docker run -d --name teslacost-test-pg -e POSTGRES_USER=teslacost -e POSTGRES_PASSWORD=test -e POSTGRES_DB=teslacost_test -p 55432:5432 postgres:14-alpine
TEST_DATABASE_URL="postgres://teslacost:test@localhost:55432/teslacost_test?sslmode=disable" go test -v ./internal/services/

# Compiler le frontend Vue 3
cd web && npm install && npm run build
```

---

## 📄 Licence

Ce projet est distribué sous licence [MIT](LICENSE).

Les données de péage utilisées pour la détection et l'estimation de coût (`internal/tolldata`) proviennent de [OpenTollData](https://github.com/louis2038/OpenTollData), sous licence [ODbL-1.0](https://opendatacommons.org/licenses/odbl/1-0/).