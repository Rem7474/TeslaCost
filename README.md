# TeslaCost 🚗⚡

> Application auto-hébergée open-source de suivi des coûts de possession et d'entretien automobile (TCO), particulièrement optimisée pour les véhicules électriques et compatible avec [TeslaMate](https://github.com/teslamate-org/teslamate) via [`teslamateapi`](https://github.com/tobiasehlert/teslamateapi), tout en étant 100% utilisable de façon autonome.

---

## 🚀 Fonctionnalités principales

1. **Multi-véhicules & Authentification JWT :** Gestion multi-utilisateurs et multi-véhicules avec contrôle d'accès sécurisé.
2. **Synchronisation TeslaMate API :**
   - Récupération de l'odomètre en temps réel.
   - Synchronisation incrémentale des trajets (`/drives`) et des charges (`/charges` avec coûts kWh et devises).
   - Support d'authentification Bearer Token et HTTP Basic Auth.
   - Chiffrement symétrique au repos AES-256-GCM des identifiants et tokens API dans la base de données.
3. **Péages, Parkings et Fusion de Trajets :**
   - Création de groupes de trajets (`TripGroup`) pour fusionner des étapes segmentées par des pauses.
   - Affectation granulaire des dépenses de voyage (péages d'autoroutes, parkings, ferries).
4. **Gestion du Cycle de Vie des Pneus :**
   - Fiche produit (marque, modèle, dimensions, saison, prix, dot code).
   - Position dynamique sur véhicule (`FL`, `FR`, `RL`, `RR`, `STORAGE`, `DISPOSED`).
   - Relevés millimétriques de la profondeur de sculpture et projection de l'usure kilométrique restante.
   - Historique et journal complet des permutations de roues avec odomètre.
5. **Entretien & Coûts Fixes :** Suivi des révisions, assurances, abonnements connectivité, taxes.
6. **Calculateur de TCO (€ total et €/km décomposé) :** Répartition claire énergie / péages / pneus / entretien.

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

### 4. Lancer les tests unitaires
```bash
go test -v ./...
```
