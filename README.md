# TeslaCost 🚗⚡

> Application auto-hébergée open-source de suivi des coûts de possession et d'entretien automobile (TCO), particulièrement optimisée pour les véhicules électriques et compatible avec [TeslaMate](https://github.com/teslamate-org/teslamate) via [`teslamateapi`](https://github.com/tobiasehlert/teslamateapi), tout en étant 100% utilisable de façon autonome.

---

## 🚀 Fonctionnalités principales

1. **Multi-véhicules & Authentification JWT :** Gestion multi-utilisateurs et multi-véhicules avec contrôle d'accès sécurisé.
2. **Synchronisation TeslaMate API :**
   - Récupération de l'odomètre en temps réel.
   - Synchronisation lancée en arrière-plan : l'API répond immédiatement et l'interface suit l'avancement ; une seule synchronisation à la fois par véhicule (manuelle ou planifiée).
   - Contrôle de continuité de l'odomètre : trous entre trajets consécutifs, odomètre en recul, distance différente du relevé (`GET /api/vehicles/{id}/data-quality`).
   - Import complet de l'historique des trajets (`/drives`) et des charges (`/charges`), repris automatiquement à la synchronisation suivante en cas d'interruption.
   - Synchronisation incrémentale relisant les 30 derniers jours pour récupérer les coûts complétés après coup dans TeslaMate.
   - Une recharge sans tarif TeslaMate est enregistrée « sans coût » (jamais 0 €) et signalée ; son coût peut être saisi manuellement sans être écrasé par les synchronisations.
   - Saisie des recharges hors TeslaMate (prise d'un tiers, borne non suivie).
   - Trajets et recharges supprimés dans TeslaMate exclus des calculs (tags et liens conservés, restaurés s'ils réapparaissent). Si plus de 20 % de la période relue disparaît d'un coup, rien n'est retiré et un avertissement est affiché.
   - Support d'authentification Bearer Token et HTTP Basic Auth.
   - Chiffrement symétrique au repos AES-256-GCM des identifiants et tokens API dans la base de données.
3. **Péages, Parkings et Fusion de Trajets :**
   - Création de groupes de trajets (`TripGroup`) pour fusionner des étapes segmentées par des pauses.
   - Affectation granulaire des dépenses de voyage (péages d'autoroutes, parkings, ferries) ; une dépense de groupe est répartie entre les étapes au prorata des kilomètres.
   - Sélection de trajets conservée d'une page à l'autre ; frais d'un trajet ou d'un voyage modifiables et supprimables depuis la fenêtre de coût du trajet.
   - Onglet « Voyages » : renommer, retirer ou ajouter des trajets, supprimer un voyage en conservant ou non ses frais.
   - File « À qualifier » : trajets de type autoroutier (≥ 40 km et ≥ 70 km/h de moyenne, ou ≥ 20 km et Vmax > 125 km/h) sans péage renseigné, à compléter ou marquer « sans péage ».
   - Dépenses en devise étrangère avec taux de conversion vers l'euro saisi à la dépense.
4. **Gestion du Cycle de Vie des Pneus :**
   - Fiche produit (marque, modèle, dimensions, saison, prix, dot code).
   - Position dynamique sur véhicule (`FL`, `FR`, `RL`, `RR`, `STORAGE`, `DISPOSED`).
   - Relevés millimétriques de la profondeur de sculpture et projection de l'usure kilométrique restante, calculée sur les kilomètres roulés par le pneu (périodes en stockage exclues).
   - Historique et journal complet des permutations de roues avec odomètre (sessions de montage ouvertes et fermées de façon transactionnelle).
   - Kilométrage initial conservé pour les pneus achetés d'occasion.
   - Modification d'un pneu ou par lot (marque, dimensions, prix unitaire ou total réparti au centime, date et odomètre du montage en cours), relevés d'usure modifiables et supprimables.
   - Mise au rebut (montage clôturé, historique et coût conservés) ou suppression d'une saisie erronée.
5. **Covoiturage (BlaBlaCar & directs) :**
   - Un covoiturage est une suite d'étapes : un trajet TeslaMate par étape (énergie mesurée, péages du trajet, usure, entretien et assurance au km) ou des étapes saisies à la main.
   - Chaque passager a un arrêt de montée, un arrêt de descente et un nombre de places.
   - Le coût de chaque étape est partagé à parts égales entre les personnes à bord, conducteur compris : la part d'un passager est la somme des étapes parcourues, l'arrondi reste au conducteur. Pour chaque passager, le montant payé est comparé à sa part.
6. **Saisie hors connexion (PWA) :** les péages, dépenses, recharges et qualifications de trajets saisis sans réseau sont conservés dans le navigateur (IndexedDB) puis envoyés au retour de la connexion. Chaque envoi porte un en-tête `Idempotency-Key` : une requête rejouée après une réponse perdue n'est appliquée qu'une fois.
7. **Entretien & Coûts Fixes :** Suivi des révisions, assurances, abonnements connectivité, taxes. Une dépense récurrente compte une échéance par période jusqu'à aujourd'hui ou jusqu'à sa date de fin.
8. **Calculateur de TCO :**
   - Registre des coûts unique (vue SQL `cost_ledger`) : recharges, péages, dépenses récurrentes générées échéance par échéance, pneus, assurance, achat du véhicule. Totaux, historique mensuel et taux au km en sont tous dérivés.
   - Montants stockés et calculés en centimes exacts (`NUMERIC` en base, entiers en Go).
   - Dépenses courantes décaissées (pneus au jour d'achat, achat du véhicule exclu) et coût complet (usure des pneus amortie au kilomètre, décote du véhicule).
   - Acquisition et financement par véhicule : achat comptant, achat à crédit (échéancier des intérêts calculé mois par mois, frais de dossier, assurance emprunteur), LOA ou LLD (apport, loyers, frais de dossier, dépôt de garantie, frais de restitution, forfait kilométrique et pénalité de dépassement estimée, services inclus, option d'achat et sa levée). Les flux sont générés automatiquement dans le registre des coûts.
   - Décote linéaire (prix + frais − aides − revente estimée sur la durée de détention), figée sur le prix de revente réel à la fin de détention ; les dépenses récurrentes s'arrêtent à cette date.
   - Coût complet : apport et frais de location étalés sur la durée du contrat, frais de restitution et dépassement kilométrique provisionnés au fil du contrat.
   - Coût d'usage au km (énergie + péages), coût complet au km et coût net des recettes de covoiturage, calculés sur la plus grande distance entre les trajets suivis, l'odomètre couvert par les trajets et le kilométrage depuis l'acquisition.
   - Ventilation énergie / péages & parkings / pneus / entretien & réparations / assurance / financement & location / décote / abonnements, taxes & autres.
   - Assurance : primes enregistrées en dépense récurrente (aide « prime annuelle → mensualité ») ; le coût d'un trajet en reprend une quote-part calculée sur les primes et les kilomètres réellement parcourus sur 12 mois. Une assurance incluse dans la location n'est pas signalée comme manquante.
   - Score de complétude pondéré (« TCO consolidé à X % ») : recharges avec coût, kilomètres couverts par des trajets, trajets autoroutiers qualifiés, assurance, acquisition, continuité de l'odomètre, conversion des devises ; chaque manque est détaillé avec un lien pour le corriger.

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
