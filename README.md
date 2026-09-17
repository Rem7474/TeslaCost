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
- **📱 PWA & Mode hors-connexion** : saisie des dépenses sans réseau avec file d'attente IndexedDB et déduplication par clé d'idempotence.

---

## 🚀 Fonctionnalités détaillées

### 1. Authentification & Sécurité
- **Authentification locale** : inscription/connexion sécurisée par mot de passe (bcrypt) et émission de tokens JWT HS256.
- **Support OIDC / OAuth2 (SSO)** : délégation d'authentification à votre IdP homelab (Authentik, Keycloak, Authelia, Kanidm).
  - Flux standard *Authorization Code Flow* avec vérification anti-CSRF (`state`) et anti-rejeu (`nonce` chiffré SHA-256).
  - Just-In-Time (JIT) provisioning : création automatique du compte ou liaison avec un compte local existant partageant le même email.
  - Whitelist optionnelle (`OIDC_ALLOWED_EMAILS`) et désactivation possible de l'authentification locale (`OIDC_DISABLE_LOCAL_AUTH`).
- **Chiffrement au repos** : chiffrement symétrique AES-256-GCM des identifiants et tokens de connexion TeslaMate dans PostgreSQL.

### 2. Synchronisation TeslaMate API
- **Odomètre temps réel** : actualisation en direct de l'odomètre du véhicule dès que TeslaMate le remonte.
- **Tâches en arrière-plan** : synchronisation non bloquante avec suivi de progression et exclusion mutuelle par véhicule.
- **Reprise après coupure** : importation de l'historique des trajets (`/drives`) et recharges (`/charges`) reprenant automatiquement là où elle s'est arrêtée.
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

### 4. Rappels d'entretien & Notifications Homelab
- **Double condition de déclenchement** : surveillance combinée de la date d'échéance et/ou du seuil kilométrique calculé sur l'odomètre réel.
- **Seuils d'anticipation configurables** : notification préalable avant le dépassement critique (ex: avertir 500 km ou 15 jours avant).
- **Connecteurs de notification webhook** :
  - **Discord** : envoi de messages avec embed enrichi (couleurs de statut, champs organisés).
  - **Telegram** : messages formatés Markdown via bot HTTP.
  - **Gotify** : notifications push auto-hébergées avec gestion des priorités.
  - **JSON Générique** : intégration directe avec Home Assistant, Node-RED ou n8n.

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

1. Clonez le dépôt et copiez le fichier d'environnement :
   ```bash
   git clone https://github.com/Rem7474/TeslaCost.git
   cd TeslaCost
   cp .env.example .env
   ```

2. Éditez `.env` pour configurer vos clés de sécurité et vos paramètres :
   ```bash
   # Générer une clé de chiffrement AES-256 de 32 octets en hexadécimal :
   openssl rand -hex 32
   ```

3. Lancez la stack :
   ```bash
   docker compose up -d
   ```

L'application est disponible sur `http://localhost:8080`.

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
  -e DATABASE_URL="postgres://user:password@postgres-host:5432/teslacost?sslmode=disable" \
  -e JWT_SECRET="votre_clef_secrete_jwt_robuste" \
  -e ENCRYPTION_KEY="clef_hexadecimale_de_64_caracteres_exactement" \
  -e APP_TIMEZONE="Europe/Paris" \
  ghcr.io/rem7474/teslacost:latest
```

---

## ⚙️ Variables d'Environnement

| Variable | Description | Valeur par défaut |
|---|---|---|
| `PORT` | Port d'écoute du serveur HTTP | `8080` |
| `APP_ENV` | Environnement d'exécution (`production`, `development`) | `production` |
| `DATABASE_URL` | Chaîne de connexion PostgreSQL (`postgres://...`) | *Obligatoire* |
| `JWT_SECRET` | Secret de signature des jetons JWT | *Obligatoire* |
| `JWT_EXPIRATION_HOURS` | Durée de validité des sessions utilisateurs (heures) | `72` |
| `ENCRYPTION_KEY` | Clé hexadécimale AES-256 de 64 caractères | *Obligatoire* |
| `APP_TIMEZONE` | Fuseau horaire de calcul et reporting | `Europe/Paris` |
| `STORAGE_DIR` | Répertoire de stockage des documents sur le volume | `/data/documents` |
| `DISABLE_REGISTRATION` | Désactiver la création libre de compte local | `false` |
| `INITIAL_ADMIN_EMAIL` | Email de l'administrateur pré-initialisé | *Optionnel* |
| `INITIAL_ADMIN_PASSWORD` | Mot de passe de l'administrateur pré-initialisé | *Optionnel* |
| `CORS_ALLOWED_ORIGINS` | Origines autorisées (séparées par virgule) | `http://localhost:8080` |

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