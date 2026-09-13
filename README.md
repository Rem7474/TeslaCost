# TeslaCost 🚗⚡

> Application auto-hébergée open-source de suivi des coûts de possession et d'entretien automobile (TCO), particulièrement optimisée pour les véhicules électriques et compatible avec [TeslaMate](https://github.com/teslamate-org/teslamate) via [`teslamateapi`](https://github.com/tobiasehlert/teslamateapi), tout en étant 100% utilisable de façon autonome.

---

## 🚀 Fonctionnalités principales

1. **Multi-véhicules & Authentification JWT :** Gestion multi-utilisateurs et multi-véhicules avec contrôle d'accès sécurisé.
2. **Synchronisation TeslaMate API :**
   - Récupération de l'odomètre en temps réel.
   - Import complet de l'historique des trajets (`/drives`) et des charges (`/charges`), repris automatiquement à la synchronisation suivante en cas d'interruption.
   - Synchronisation incrémentale relisant les 30 derniers jours pour récupérer les coûts complétés après coup dans TeslaMate.
   - Une recharge sans tarif TeslaMate est enregistrée « sans coût » (jamais 0 €) et signalée ; son coût peut être saisi manuellement sans être écrasé par les synchronisations.
   - Saisie des recharges hors TeslaMate (prise d'un tiers, borne non suivie).
   - Support d'authentification Bearer Token et HTTP Basic Auth.
   - Chiffrement symétrique au repos AES-256-GCM des identifiants et tokens API dans la base de données.
3. **Péages, Parkings et Fusion de Trajets :**
   - Création de groupes de trajets (`TripGroup`) pour fusionner des étapes segmentées par des pauses.
   - Affectation granulaire des dépenses de voyage (péages d'autoroutes, parkings, ferries) ; une dépense de groupe est répartie entre les étapes au prorata des kilomètres.
   - File « À qualifier » : trajets de type autoroutier (≥ 40 km, ≥ 70 km/h de moyenne) sans péage renseigné, à compléter ou marquer « sans péage ».
   - Dépenses en devise étrangère avec taux de conversion vers l'euro saisi à la dépense.
4. **Gestion du Cycle de Vie des Pneus :**
   - Fiche produit (marque, modèle, dimensions, saison, prix, dot code).
   - Position dynamique sur véhicule (`FL`, `FR`, `RL`, `RR`, `STORAGE`, `DISPOSED`).
   - Relevés millimétriques de la profondeur de sculpture et projection de l'usure kilométrique restante, calculée sur les kilomètres roulés par le pneu (périodes en stockage exclues).
   - Historique et journal complet des permutations de roues avec odomètre (sessions de montage ouvertes et fermées de façon transactionnelle).
   - Kilométrage initial conservé pour les pneus achetés d'occasion.
5. **Entretien & Coûts Fixes :** Suivi des révisions, assurances, abonnements connectivité, taxes. Une dépense récurrente compte une échéance par période jusqu'à aujourd'hui ou jusqu'à sa date de fin.
6. **Calculateur de TCO :**
   - Montant décaissé (achats de pneus au jour d'achat) et coût complet (usure des pneus amortie au kilomètre).
   - Coût d'usage au km (énergie + péages) et coût complet au km, calculés sur la distance odométrique couverte par les trajets.
   - Ventilation énergie / péages & parkings / pneus / entretien / assurance / abonnements, taxes & autres.
   - Assurance : dépenses « Assurance » enregistrées, sinon prime annuelle de la fiche véhicule répartie au prorata du temps.
   - Indicateur de complétude : recharges sans coût, dépenses non converties, trajets à qualifier, kilomètres non suivis, assurance absente.

---

## 📁 Arborescence du Projet

```text
TeslaCost/
├── cmd/
│   └── server/
│       └── main.go                 # Point d'entrée de l'application
├── internal/
│   ├── config/                     # Configuration d'environnement
│   ├── crypto/                     # Chiffrement AES-256-GCM des secrets
│   ├── database/                   # Pool de connexions pgxpool
│   ├── models/                     # Entités Go fortement typées
│   ├── teslamate/                  # Client Go teslamateapi & DTOs
│   ├── auth/                       # Hachage bcrypt & JWT
│   ├── handlers/                   # Contrôleurs HTTP Chi
│   ├── services/                   # Métier TCO, usure pneus, fusion trajets
│   └── middleware/                 # CORS, authentification, logs
├── migrations/                     # Schémas PostgreSQL versionnés (up/down)
├── web/                            # Frontend Vue 3 + Vite + Tailwind CSS (PWA)
├── docker-compose.yml              # Stack de développement locale
├── Dockerfile                      # Multi-stage build (Vue + Go embed)
├── Makefile                        # Raccourcis de développement
└── .env.example                    # Exemple de configuration d'environnement
```

---

## 🛠️ Démarrage Rapide

### 1. Prérequis
- [Docker](https://www.docker.com/) & Docker Compose
- *Optionnel pour le dev local :* Go 1.25+ et Node.js 20+

### 2. Configuration
Copiez le fichier d'exemple :
```bash
cp .env.example .env
```

### 3. Lancer la stack de développement
```bash
docker compose up -d
```
PostgreSQL démarre avec initialisation automatique du schéma SQL (`migrations/000001_init_schema.up.sql`), et l'API Go est accessible sur `http://localhost:8080`.

Vérification de l'API :
```bash
curl http://localhost:8080/api/health
```

### 4. Utiliser l'image Docker pré-compilée (GHCR)

L'image Docker officielle multi-architecture (`linux/amd64`, `linux/arm64`) est publiée sur **GitHub Container Registry** à chaque release :

```bash
docker pull ghcr.io/rem7474/teslacost:latest
```

Exemple de déploiement autonome :
```bash
docker run -d \
  --name teslacost \
  -p 8080:8080 \
  -e DATABASE_URL="postgres://user:password@db-host:5432/teslacost?sslmode=disable" \
  -e JWT_SECRET="votre_clef_secrete_jwt" \
  -e ENCRYPTION_KEY="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" \
  ghcr.io/rem7474/teslacost:latest
```

### 5. Lancer les tests
```bash
go test -v ./...
```

Les tests d'intégration (requêtes SQL, migrations, TCO) nécessitent une base PostgreSQL jetable ; son schéma `public` est supprimé et recréé :
```bash
docker run -d --name teslacost-test-pg -e POSTGRES_USER=teslacost -e POSTGRES_PASSWORD=test -e POSTGRES_DB=teslacost_test -p 55432:5432 postgres:14-alpine
TEST_DATABASE_URL="postgres://teslacost:test@localhost:55432/teslacost_test?sslmode=disable" go test ./internal/services/
```

### 6. Migrations de base de données
Les migrations embarquées (`migrations/*.up.sql`) sont appliquées au démarrage, chacune dans une transaction, et enregistrées dans la table `schema_migrations`. Le serveur refuse de démarrer si une migration échoue.
