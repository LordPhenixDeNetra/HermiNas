# Cahier des Charges — HermiNas

## Version 1.0 — Juillet 2026
## Plateforme souveraine de données temps réel — Stack polyglotte Go/Rust/Python/TypeScript
## Basé sur les leçons apprises d'aNtaerus (architecture L0-L3, config immuable, secrets typés)

---

## Table des matières

1. [Contexte et Vision](#1-contexte-et-vision)
2. [Positionnement et Différenciation](#2-positionnement-et-différenciation)
3. [Spécifications Fonctionnelles](#3-spécifications-fonctionnelles)
4. [Spécifications Techniques](#4-spécifications-techniques)
5. [Architecture Logicielle](#5-architecture-logicielle)
6. [Interfaces et API](#6-interfaces-et-api)
7. [Sécurité et Gouvernance](#7-sécurité-et-gouvernance)
8. [Performance et Scalabilité](#8-performance-et-scalabilité)
9. [Déploiement et Distribution](#9-déploiement-et-distribution)
10. [Planning et Jalons](#10-planning-et-jalons)
11. [Leçons apprises d'aNtaerus](#11-leçons-apprises-dantaerus)
12. [Glossaire](#12-glossaire)

---

## 1. Contexte et Vision

### 1.1 Énoncé du problème

Les entreprises génèrent des volumes de données sans précédent : logs applicatifs, métriques système, transactions financières (mobile money notamment), données IoT, flux réseaux sociaux. Les solutions existantes présentent des limitations majeures :

- **Coûts prohibitifs et imprévisibles** : facturation à la requête ou au volume ingéré (Datadog, Snowflake, BigQuery), surprises budgétaires en devises étrangères
- **Latence insuffisante** : approche batch (minutes/heures) au lieu du temps réel (millisecondes)
- **Verrouillage propriétaire** : formats fermés, migration impossible, dépendance cloud
- **Complexité opérationnelle** : empilement de 10+ outils (Kafka + Flink + ClickHouse + Grafana + Airflow + etc.) nécessitant une équipe DevOps senior
- **Perte de souveraineté** : données hébergées sur des clouds étrangers, conformité difficile (réglementations locales diverses — RGPD, cadres sectoriels nationaux, etc.)

### 1.2 Vision produit

**HermiNas** est une plateforme de données **self-hosted**, **unifiée**, **polyglotte** et **véritablement temps réel** qui combine :

- Une **ingestion haute fréquence** via agent Rust léger (objectif : centaines de milliers d'événements/seconde par nœud)
- Un **traitement stream natif** (fenêtres, agrégations, enrichissement, scoring en vol)
- Un **stockage analytique OLAP** (ClickHouse embarqué comme boîte noire)
- Un **bus d'événements** (Redpanda embarqué comme boîte noire)
- Une **intelligence artificielle intégrée** : détection d'anomalies, prédictions, requêtes en langage naturel (français inclus)
- Des **tableaux de bord temps réel** interactifs
- Un **coût prévisible** : hardware uniquement, aucune facturation à l'usage

### 1.3 Principe fondateur

> HermiNas ne réécrit pas les moteurs, il les **unifie**. ClickHouse et Redpanda sont embarqués comme composants internes (à la manière dont aNtaerus embarque Whisper et Piper). Toute la valeur ajoutée d'HermiNas se situe dans l'expérience unifiée, l'intelligence intégrée, la souveraineté et le verticale métier.

### 1.4 Cas d'usage vertical v1 (à confirmer)

**Détection d'anomalies et supervision temps réel sur flux transactionnels** (fintech, mobile money, agrégateurs de paiement) :
- Ingestion des transactions en continu
- Scoring de fraude en vol (< 200ms par transaction)
- Alertes temps réel, dashboards de supervision
- Requêtes en langage naturel : « montre-moi les transactions suspectes de la semaine »
- Données hébergées chez le client, conformité locale facilitée (cadre paramétrable par juridiction)

Verticales secondaires envisagées : monitoring télécom/énergie, supervision logistique, observabilité IT.

### 1.5 Public cible

- **Fintechs et agrégateurs de paiement** : détection fraude, conformité, supervision
- **Télécoms et énergie** : monitoring réseau, métriques temps réel
- **PME/ETI data-driven** : analytics sans DevOps senior ni budget cloud
- **Secteur public et banques** : souveraineté des données, hébergement local obligatoire
- **Développeurs** : plateforme hackable, extensible (WASM), API-first

---

## 2. Positionnement et Différenciation

### 2.1 Matrice concurrentielle

| | HermiNas | Datadog/Snowflake | Stack DIY (Kafka+Flink+CH+Grafana) | Tinybird/Materialize |
|--|----------|--------------------|-------------------------------------|----------------------|
| Hébergement | **Local/souverain** | Cloud étranger | Local possible | Cloud |
| Coût | **Hardware seul, prévisible** | À l'usage, imprévisible | Gratuit mais coût humain élevé | À l'usage |
| Installation | **1 binaire, < 15 min** | SaaS | Jours/semaines, expertise requise | SaaS |
| Temps réel | **Millisecondes** | Secondes/minutes | Millisecondes (si bien configuré) | Millisecondes |
| IA intégrée | **Anomalies + NL→SQL (FR/EN)** | Payant, cloud | À construire soi-même | Non |
| Conformité multi-juridictionnelle | **Cadre paramétrable by design** | Difficile | À construire | Non |
| Extensibilité | **WASM sandbox** | Limitée | Illimitée mais complexe | SQL only |

### 2.2 Différenciateurs clés

1. **Souveraineté** : tout tourne chez le client, zéro donnée sortante, audit complet
2. **Simplicité radicale** : un installeur, un dashboard, zéro assemblage d'outils
3. **NL→SQL multilingue** : requêtes en français naturel, atout unique sur le marché francophone
4. **Verticale fintech** : règles de fraude prêtes à l'emploi, conformité paramétrable par pays
5. **Coût prévisible** : licence + hardware, jamais de facturation au volume
6. **WASM Rules Engine** : les clients écrivent leurs propres règles (fraude, alertes), exécutées en sandbox dans le flux

---

## 3. Spécifications Fonctionnelles

### 3.1 Ingestion (Agent Rust + Connecteurs Go)

| ID | Fonctionnalité | Priorité | Implémentation |
|----|----------------|----------|----------------|
| F-101 | Agent d'ingestion léger déployable (Linux/Windows) | P0 | Rust (binaire statique < 20 MB) |
| F-102 | Collecte fichiers logs (tail, rotation, multiline) | P0 | Rust |
| F-103 | Réception HTTP/JSON (webhook ingestion) | P0 | Rust (axum) |
| F-104 | Réception TCP/UDP (syslog, protocoles binaires) | P1 | Rust |
| F-105 | Parsing structuré (JSON, CSV, regex, grok) | P0 | Rust |
| F-106 | Buffering disque local (résilience coupures réseau) | P0 | Rust (WAL local) |
| F-107 | Compression + batching vers le serveur | P0 | Rust (zstd) |
| F-108 | Connecteur bases SQL (CDC léger / polling) | P1 | Go |
| F-109 | Connecteur S3/MinIO (import batch) | P1 | Go |
| F-110 | Connecteur Kafka externe (bridge) | P2 | Go |
| F-111 | Connecteur APIs REST tierces (polling planifié) | P1 | Go |
| F-112 | Schéma auto-détecté + registry de schémas | P1 | Go + Rust |
| F-113 | Backpressure et rate limiting côté agent | P0 | Rust |

### 3.2 Traitement stream (Moteur Rust)

| ID | Fonctionnalité | Priorité | Implémentation |
|----|----------------|----------|----------------|
| F-201 | Pipeline déclaratif (YAML/SQL) : source → transforms → sink | P0 | Rust |
| F-202 | Transformations : filter, map, enrich, parse | P0 | Rust |
| F-203 | Fenêtres temporelles (tumbling, sliding) | P0 | Rust |
| F-204 | Agrégations en vol (count, sum, avg, percentiles) | P0 | Rust |
| F-205 | Jointures stream-table (enrichissement référentiel) | P1 | Rust |
| F-206 | Scoring ML en vol (modèles ONNX) | P0 | Rust (`ort`) |
| F-207 | Règles utilisateur en WASM sandbox | P1 | Rust (`wasmtime`) |
| F-208 | Détection d'anomalies temps réel (z-score, EWMA, seuils dynamiques) | P0 | Rust |
| F-209 | Dead letter queue (événements invalides) | P0 | Rust |
| F-210 | Exactly-once vers ClickHouse (idempotence) | P1 | Rust |
| F-211 | Latence de traitement < 5ms par événement (p99) | P0 | Rust |

### 3.3 Stockage et requêtes (ClickHouse embarqué + Go)

| ID | Fonctionnalité | Priorité | Notes |
|----|----------------|----------|-------|
| F-301 | ClickHouse embarqué, supervisé par HermiNas | P0 | Boîte noire, config générée |
| F-302 | Gestion tables/schémas via UI (pas de DDL manuel requis) | P0 | Go génère le DDL |
| F-303 | Politiques de rétention (TTL par table/dataset) | P0 | Go |
| F-304 | Tiering chaud/froid (disque local → S3/MinIO) | P2 | ClickHouse natif piloté |
| F-305 | Éditeur SQL avec autocomplétion | P0 | React + Go proxy |
| F-306 | API de requêtes (REST + streaming) | P0 | Go |
| F-307 | Vues matérialisées gérées (rollups automatiques) | P1 | Go |
| F-308 | Export résultats (CSV, JSON, Parquet) | P1 | Go |
| F-309 | Quotas par utilisateur/rôle (protection ressources) | P1 | Go |

### 3.4 Intelligence artificielle (Python)

| ID | Fonctionnalité | Priorité | Implémentation |
|----|----------------|----------|----------------|
| F-401 | NL→SQL : requêtes en langage naturel (FR + EN) | P0 | Python (litellm) + schéma contexte |
| F-402 | Validation/garde-fous SQL généré (read-only, quotas) | P0 | Python + Go |
| F-403 | Entraînement modèles anomalies (par dataset) | P0 | Python (scikit-learn, river) |
| F-404 | Export modèles ONNX vers moteur Rust | P0 | Python |
| F-405 | Prévisions séries temporelles (trafic, volumes) | P1 | Python (Prophet/statsforecast) |
| F-406 | Scoring fraude : modèle de base + règles métier | P0 | Python (train) + Rust (inference) |
| F-407 | Résumés automatiques d'incidents (LLM) | P2 | Python |
| F-408 | Suggestions de dashboards selon les données | P2 | Python |
| F-409 | LLM local (Ollama) ou cloud (Anthropic, Mistral) au choix | P0 | Python (litellm) |
| F-410 | Ré-entraînement planifié (drift detection) | P1 | Python (jobs) |

### 3.5 Alerting et notifications (Go)

| ID | Fonctionnalité | Priorité | Notes |
|----|----------------|----------|-------|
| F-501 | Règles d'alerte sur seuils/requêtes | P0 | Go évaluation périodique + stream |
| F-502 | Alertes déclenchées par le moteur d'anomalies | P0 | Rust → Go |
| F-503 | Canaux : email, webhook, Telegram | P0 | Go |
| F-504 | Canaux : SMS (passerelles locales), Slack | P1 | Go |
| F-505 | Déduplication, groupement, silences | P1 | Go |
| F-506 | Escalade et accusés de réception | P2 | Go |
| F-507 | Historique d'alertes + statistiques | P1 | Go + ClickHouse |

### 3.6 Tableaux de bord (TypeScript/React)

| ID | Écran | Description | Priorité |
|----|-------|-------------|----------|
| UI-101 | **Home / Overview** | Santé plateforme, volumes, alertes actives | P0 |
| UI-102 | **Dashboards** | Grilles de widgets temps réel (lignes, barres, jauges, tables, heatmaps) | P0 |
| UI-103 | **Query Studio** | Éditeur SQL + onglet langage naturel + historique | P0 |
| UI-104 | **Pipelines** | Création/édition pipelines visuels (source → transforms → sink) | P0 |
| UI-105 | **Datasets** | Tables, schémas, rétention, volumes | P0 |
| UI-106 | **Alerting** | Règles, canaux, historique | P0 |
| UI-107 | **ML Studio** | Modèles, entraînements, métriques, déploiement ONNX | P1 |
| UI-108 | **Agents** | Flotte d'agents Rust, état, config à distance | P1 |
| UI-109 | **Admin** | Utilisateurs, rôles, quotas, audit | P0 |
| UI-110 | **Setup Wizard** | Installation guidée, première source en < 15 min | P0 |
| UI-111 | **System Health** | État Go/Rust/Python/ClickHouse/Redpanda, logs, restart | P1 |
| UI-112 | **Compliance Reports** | Rapports d'audit exportables (PDF) | P2 |

### 3.7 Accessibilité et intégrations

| ID | Canal | Priorité | Implémentation |
|----|-------|----------|----------------|
| ACC-101 | Web (navigateur) | P0 | React + Vite |
| ACC-102 | API REST publique (tokens) | P0 | Go |
| ACC-103 | Grafana datasource (compat) | P2 | Go (proxy ClickHouse) |
| ACC-104 | Bot Telegram (alertes + requêtes NL) | P2 | Go |
| ACC-105 | Embed dashboards (iframe signé) | P2 | Go + React |

---

## 4. Spécifications Techniques

### 4.1 Stack technologique

| Couche | Technologie | Version | Justification |
|--------|-------------|---------|---------------|
| **Frontend** | React | 18+ | Écosystème, temps réel |
| | Vite | 5+ | Build rapide, HMR |
| | TypeScript | 5+ | Typage fort |
| | Zustand | 4+ | State léger (acquis aNtaerus) |
| | TanStack Query | 5+ | Cache server state (acquis aNtaerus) |
| | ECharts | 5+ | Visualisations riches temps réel |
| | Monaco Editor | — | Éditeur SQL |
| **Control plane** | Go | 1.22+ | Concurrence, networking, supervision |
| | gRPC-go | — | Go ↔ Rust ↔ Python |
| | Gorilla WebSocket | — | Streaming UI |
| | Viper/Cobra | — | Config + CLI (acquis aNtaerus) |
| | golang-jwt | — | Auth |
| **Data plane** | Rust | 1.78+ | Performance, sûreté mémoire |
| | Tokio | — | Async runtime |
| | tonic | — | gRPC |
| | axum | — | HTTP ingestion |
| | arrow-rs / DataFusion | — | Format colonne + SQL en vol |
| | ort (ONNX Runtime) | — | Inference ML |
| | wasmtime | — | Sandbox règles clients |
| | rdkafka | — | Client Redpanda |
| | zstd | — | Compression |
| **Intelligence** | Python | 3.11+ | ML, LLM, data science |
| | FastAPI | — | API interne (localhost) |
| | litellm | — | Multi-LLM (Anthropic, Mistral, Ollama) |
| | scikit-learn / river | — | ML batch + online |
| | statsforecast / Prophet | — | Séries temporelles |
| | onnx / skl2onnx | — | Export modèles |
| | pydantic | — | Validation (acquis aNtaerus) |
| **Moteurs embarqués** | ClickHouse | 24+ | OLAP, boîte noire supervisée |
| | Redpanda | 24+ | Bus événements, boîte noire supervisée |
| **Base système** | SQLite | 3.45+ | Métadonnées HermiNas (config, users, pipelines) |

### 4.2 Protocoles de communication

| Service A | Service B | Protocole | Format | Latence cible |
|-----------|-----------|-----------|--------|---------------|
| Agent Rust | Serveur Rust | gRPC (TLS) ou HTTP/2 | Protobuf | réseau |
| React | Go | WebSocket (WSS) | JSON | < 50ms |
| React | Go | HTTP/2 (REST) | JSON | < 100ms |
| Go | Rust | gRPC bidirectionnel | Protobuf | < 5ms |
| Go | Python | gRPC ou HTTP interne | Protobuf/JSON | < 50ms |
| Rust | Redpanda | Kafka protocol | binaire | < 5ms |
| Rust | ClickHouse | Native protocol / HTTP | RowBinary | < 10ms |
| Go | ClickHouse | HTTP (requêtes) | JSON/TSV | selon requête |
| Go | SQLite | SQL | — | < 5ms |

### 4.3 Modèles IA supportés

| Fournisseur | Usage | Mode | Priorité |
|-------------|-------|------|----------|
| **Ollama** (Llama, Mistral, Qwen) | NL→SQL, résumés | Local | P0 |
| **Anthropic Claude** | NL→SQL premium, résumés | API cloud | P0 |
| **Mistral** | NL→SQL (bon en français) | API cloud | P1 |
| **scikit-learn / river** | Anomalies, classification fraude | Local (train Python, infer Rust/ONNX) | P0 |
| **statsforecast** | Prévisions | Local | P1 |

### 4.4 Formats de données

| Étape | Format |
|-------|--------|
| Agent → serveur | Protobuf compressé zstd |
| In-flight (moteur Rust) | Apache Arrow (colonne, zero-copy) |
| Bus | Protobuf sur Redpanda |
| Stockage | ClickHouse natif (MergeTree) |
| Export | CSV, JSON, Parquet |
| Modèles ML | ONNX |
| Règles clients | WASM (compilées depuis Rust/AssemblyScript/TinyGo) |

---

## 5. Architecture Logicielle

### 5.1 Principes directeurs (leçons aNtaerus)

1. **Couches strictes** : L0 (kernel) → L1 (providers) → L2 (engine) → L3 (interfaces), validées par CI
2. **Zéro shim racine** : tout sous `herminas/`, pas de `config/` ou `main.py` racine
3. **Configuration immuable** : lue une fois au boot, struct immuable, jamais de mutation runtime
4. **Secrets typés** : `SecretString` (Go), `SecretStr` (Python), `secrecy` (Rust) ; `repr` ne fuite jamais
5. **Moteurs = boîtes noires supervisées** : ClickHouse et Redpanda pilotés, jamais modifiés
6. **Data plane / Control plane séparés** : Rust ne fait que la donnée, Go ne fait que l'orchestration
7. **Sandbox par défaut** : tout code client (règles) = WASM
8. **Testabilité** : chaque service démarre sans dépendance externe pour les tests (mocks ClickHouse/Redpanda)

### 5.2 Architecture en couches

```
┌──────────────────────────────────────────────────────────────┐
│ L3 — INTERFACES                                                │
│                                                                │
│  web/ (React + Vite + TypeScript)                              │
│  ├── Overview, Dashboards, Query Studio, Pipelines             │
│  ├── Datasets, Alerting, ML Studio, Agents, Admin              │
│  └── Setup Wizard, System Health                               │
│                                                                │
│  api/ (Go — REST + WebSocket + gRPC)                           │
│  ├── routers REST v1 (query, pipelines, datasets, alerts, ml)  │
│  ├── WebSocket hub (dashboards temps réel)                     │
│  └── API publique (tokens)                                     │
│                                                                │
│  bootstrap.go (composition root)                               │
└──────────────────────┬───────────────────────────────────────┘
                       │
┌──────────────────────▼───────────────────────────────────────┐
│ L2 — ENGINE (Go, control plane)                                │
│                                                                │
│  supervisor/ — Lance/supervise ClickHouse, Redpanda, Rust, Py  │
│  querybroker/ — Proxy requêtes ClickHouse, quotas, cache       │
│  pipelinemgr/ — CRUD pipelines, déploiement vers Rust          │
│  schemamgr/ — DDL ClickHouse généré, registry schémas          │
│  alerting/ — Évaluation règles, canaux, dédup                  │
│  connectors/ — SQL CDC, S3, REST polling, Kafka bridge         │
│  auth/ — JWT, rôles, tokens API, quotas                        │
│  bus/ — Event bus interne Go (channels)                        │
│  health/ — Healthchecks, métriques Prometheus                  │
└──────────┬────────────────────────────────┬──────────────────┘
           │ gRPC                          │ gRPC/HTTP
    ┌──────▼──────────┐            ┌───────▼────────┐
    │  L1 — RUST      │            │  L1 — PYTHON   │
    │  DATA PLANE     │            │  INTELLIGENCE  │
    │                 │            │                │
    │  agent/         │            │  nlsql/        │
    │    ├── collect  │            │    ├── translator│
    │    ├── parse    │            │    ├── guard    │
    │    ├── buffer   │            │    └── context  │
    │    └── ship     │            │  ml/           │
    │  stream/        │            │    ├── anomaly  │
    │    ├── pipeline │            │    ├── fraud    │
    │    ├── window   │            │    ├── forecast │
    │    ├── aggregate│            │    └── export_onnx│
    │    ├── enrich   │            │  jobs/         │
    │    └── score(ort)│           │    ├── retrain  │
    │  rules/         │            │    └── reports  │
    │    └── wasmtime │            │  llm/          │
    │  sink/          │            │    └── litellm  │
    │    ├── clickhouse│           └────────────────┘
    │    └── redpanda │
    │  anomaly/       │     ┌─────────────────────────┐
    │    └── online   │     │  MOTEURS EMBARQUÉS      │
    │  protocol/      │     │  (boîtes noires)        │
    │    └── tonic    │     │  ClickHouse (OLAP)      │
    └─────────────────┘     │  Redpanda (bus)         │
           │                └─────────────────────────┘
           └──────────┬───────────────────┘
                      │
           ┌──────────▼──────────┐
           │  L0 — KERNEL        │
           │  contracts/ (go/rs/py)│
           │  proto/ (schémas gRPC)│
           │  schemas/ (JSON)      │
           │  settings/ (immuable) │
           │  errors/              │
           │  permissions/         │
           │  paths/               │
           └─────────────────────┘
```

### 5.3 Services détaillés

#### 5.3.1 Go Control Plane

| Module | Rôle |
|--------|------|
| `supervisor/` | Démarrage ordonné, healthcheck, restart, logs des moteurs embarqués |
| `querybroker/` | Reçoit SQL (éditeur ou NL→SQL validé), applique quotas, exécute sur ClickHouse, streame résultats |
| `pipelinemgr/` | CRUD pipelines (SQLite), compile config, pousse au moteur Rust via gRPC |
| `schemamgr/` | Génère DDL MergeTree, TTL, vues matérialisées ; registry versionné |
| `alerting/` | Scheduler d'évaluation, réception événements anomalies Rust, envoi multi-canal |
| `connectors/` | Sources non temps réel (SQL polling/CDC, S3, REST) |
| `auth/` | JWT sessions UI, tokens API, RBAC, quotas |

#### 5.3.2 Rust Data Plane

| Module | Rôle | Crates |
|--------|------|--------|
| `agent/` | Binaire déployable : collect, parse, buffer WAL, ship | `tokio`, `axum`, `zstd` |
| `stream/` | Pipelines : fenêtres, agrégats, enrichissement, Arrow in-flight | `arrow-rs`, `datafusion` |
| `score/` | Inference ONNX en vol | `ort` |
| `rules/` | Exécution règles clients sandboxées | `wasmtime` |
| `anomaly/` | Détection online (z-score, EWMA, quantiles glissants) | — |
| `sink/` | Écriture batch ClickHouse (RowBinary), production Redpanda | `clickhouse-rs`, `rdkafka` |
| `protocol/` | Serveur gRPC (commandes du control plane) | `tonic` |

#### 5.3.3 Python Intelligence

| Module | Rôle | Librairies |
|--------|------|-----------|
| `nlsql/` | Traduction NL→SQL avec contexte schéma, garde-fous (SELECT only, LIMIT forcé) | `litellm` |
| `ml/anomaly/` | Entraînement détecteurs par dataset | `scikit-learn`, `river` |
| `ml/fraud/` | Modèle fraude transactionnelle (features, train, éval) | `scikit-learn` |
| `ml/forecast/` | Prévisions volumes/trafic | `statsforecast` |
| `ml/export/` | Conversion ONNX + tests de parité | `skl2onnx`, `onnxruntime` |
| `jobs/` | Ré-entraînement planifié, détection drift, rapports | `apscheduler` |

---

## 6. Interfaces et API

### 6.1 API REST (Go)

| Endpoint | Méthode | Description | Auth |
|----------|---------|-------------|------|
| `/api/v1/query` | POST | Exécuter SQL (stream résultats) | JWT/Token |
| `/api/v1/nlquery` | POST | Requête langage naturel → SQL → résultats | JWT/Token |
| `/api/v1/datasets` | GET/POST | Lister/créer datasets | JWT |
| `/api/v1/datasets/{id}` | GET/PUT/DELETE | Gérer un dataset (schéma, TTL) | JWT |
| `/api/v1/pipelines` | GET/POST | Lister/créer pipelines | JWT |
| `/api/v1/pipelines/{id}` | GET/PUT/DELETE | Gérer un pipeline | JWT |
| `/api/v1/pipelines/{id}/deploy` | POST | Déployer vers le moteur Rust | JWT (admin) |
| `/api/v1/alerts/rules` | GET/POST | Règles d'alerte | JWT |
| `/api/v1/alerts/history` | GET | Historique | JWT |
| `/api/v1/ml/models` | GET/POST | Modèles ML | JWT |
| `/api/v1/ml/models/{id}/train` | POST | Lancer entraînement | JWT |
| `/api/v1/ml/models/{id}/deploy` | POST | Export ONNX → Rust | JWT (admin) |
| `/api/v1/agents` | GET | Flotte d'agents | JWT |
| `/api/v1/agents/{id}/config` | PUT | Config à distance | JWT (admin) |
| `/api/v1/ingest/{dataset}` | POST | Ingestion HTTP directe | Token |
| `/api/v1/admin/users` | GET/POST | Utilisateurs, rôles | JWT (admin) |
| `/api/v1/health` | GET | Santé tous services | None |
| `/api/v1/metrics` | GET | Prometheus | None (local) |

### 6.2 WebSocket (Go)

| Événement serveur → client | Description |
|----------------------------|-------------|
| `dashboard.data` | Données widget temps réel |
| `query.rows` | Lignes streamées d'une requête |
| `alert.fired` | Alerte déclenchée |
| `anomaly.detected` | Anomalie détectée (Rust) |
| `pipeline.status` | Statut pipeline |
| `system.health` | Heartbeat services |
| `agent.status` | État agent (connecté/déconnecté) |

### 6.3 gRPC (Go ↔ Rust)

```protobuf
service DataPlane {
  rpc DeployPipeline(PipelineSpec) returns (DeployResult);
  rpc StopPipeline(PipelineId) returns (StopResult);
  rpc GetPipelineStats(PipelineId) returns (PipelineStats);
  rpc DeployModel(ModelSpec) returns (DeployResult);      // ONNX
  rpc DeployRule(RuleSpec) returns (DeployResult);        // WASM
  rpc StreamEvents(EventFilter) returns (stream Event);   // anomalies, DLQ
  rpc GetHealth(Empty) returns (HealthStatus);
}
```

### 6.4 gRPC (Go ↔ Python)

```protobuf
service Intelligence {
  rpc TranslateNL(NLQuery) returns (SQLCandidate);        // + explication
  rpc TrainModel(TrainRequest) returns (stream TrainProgress);
  rpc ExportONNX(ModelId) returns (ONNXArtifact);
  rpc Forecast(ForecastRequest) returns (ForecastResult);
  rpc GetHealth(Empty) returns (HealthStatus);
}
```

---

## 7. Sécurité et Gouvernance

### 7.1 Authentification et autorisation

| Élément | Implémentation |
|---------|----------------|
| Sessions UI | JWT courte durée + refresh |
| API publique | Tokens révocables, scopés par dataset |
| RBAC | Rôles : viewer, analyst, engineer, admin |
| Quotas | Requêtes/min, lignes scannées, par rôle et par token |
| NL→SQL | Garde-fous : SELECT only, LIMIT forcé, datasets autorisés seulement |

### 7.2 Secrets (leçon aNtaerus)

| Principe | Implémentation |
|----------|----------------|
| Typage | Go `SecretString`, Python `pydantic.SecretStr`, Rust `secrecy::SecretString` |
| Chiffrement au repos | AES-256-GCM (clés LLM, credentials connecteurs) |
| Logs | Jamais de secret en clair ; test `test_secrets_no_leak` en CI |
| Transport agent | mTLS agent ↔ serveur (certificats auto-gérés) |

### 7.3 Souveraineté et conformité

| Exigence | Réponse HermiNas |
|----------|------------------|
| Localisation données | 100% chez le client ; mode air-gapped supporté (LLM local) |
| Audit | Journal append-only des requêtes, exports, accès admin |
| Rétention | TTL par dataset, purge prouvable |
| Anonymisation | Masquage de colonnes par rôle (PII) | 
| Rapports conformité | Export PDF : accès, volumes, rétentions (UI-112) |

### 7.4 Sandbox règles clients

| Contrainte | Implémentation |
|------------|----------------|
| Isolation mémoire | WASM (`wasmtime`), pas d'accès système |
| Budget CPU | Fuel metering par événement |
| Interface | ABI stable : event in → verdict/enrichment out |
| Validation | Dry-run sur échantillon avant déploiement |

---

## 8. Performance et Scalabilité

### 8.1 Objectifs v1 (mono-nœud)

| Métrique | Objectif |
|----------|----------|
| Ingestion serveur | ≥ 100 000 événements/s (soutenus) |
| Latence traitement stream (p99) | < 5 ms/événement |
| Latence scoring ONNX (p99) | < 2 ms/événement |
| Latence ingestion → visible en requête | < 2 s |
| Latence anomalie → alerte | < 500 ms |
| Requête dashboard type (1 jour de données) | < 500 ms |
| NL→SQL (traduction) | < 3 s |
| Empreinte agent Rust | < 50 MB RAM, < 5% CPU |
| Démarrage plateforme complète | < 30 s |

### 8.2 Profils matériels

| Profil | CPU | RAM | Stockage | Usage |
|--------|-----|-----|----------|-------|
| **Découverte** | 4 cores | 8 GB | 100 GB SSD | POC, < 10k evt/s |
| **Standard** | 8 cores | 32 GB | 1 TB NVMe | Production PME, 50-100k evt/s |
| **Intensif** | 16+ cores | 64+ GB | 2+ TB NVMe | Fintech volume élevé |
| **Air-gapped + LLM local** | +8 GB VRAM GPU | — | +20 GB | NL→SQL local (Ollama) |

### 8.3 Stratégies

| Composant | Stratégie |
|-----------|-----------|
| Rust in-flight | Arrow colonne, zero-copy, batching adaptatif |
| Écriture ClickHouse | Batchs 10k-100k lignes, RowBinary, async inserts |
| Requêtes | Vues matérialisées pour dashboards, cache résultat court TTL (Go) |
| ONNX | Sessions préchargées, batch scoring quand possible |
| Agent | WAL disque, reprise après coupure, compression zstd |
| Scale-out (v2) | Multi-nœuds Redpanda + ClickHouse cluster (hors périmètre v1) |

---

## 9. Déploiement et Distribution

### 9.1 Modes de distribution

| Mode | Cible | Procédure |
|------|-------|-----------|
| **Bundle Linux** | Production (serveur, VM, bare metal) | `.tar.gz` auto-contenu, systemd units générés |
| **Docker Compose** | Évaluation, dev | `docker compose up` |
| **Bundle Windows Server** | Environnements contraints | `.zip` + services Windows |
| **Agent seul** | Machines sources | Binaire unique Rust (Linux/Windows) |
| **Source** | Contributeurs | `git clone`, `task build` |

### 9.2 Structure bundle

```
herminas/
├── herminas                    # Launcher Go (CLI unique)
├── config/
│   ├── herminas.yaml           # Config plateforme
│   └── secrets.enc             # Secrets chiffrés
├── runtime/
│   ├── go/                     # control plane (binaire statique)
│   ├── rust/                   # data plane + agent (binaires statiques)
│   ├── python/                 # venv autonome intelligence
│   ├── clickhouse/             # binaire + config générée
│   └── redpanda/               # binaire + config générée
├── models/                     # ONNX déployés
├── data/
│   ├── clickhouse/             # données OLAP
│   ├── redpanda/               # log bus
│   └── herminas.db             # métadonnées SQLite
├── logs/
└── docs/
```

### 9.3 CLI

| Commande | Action |
|----------|--------|
| `herminas setup` | Wizard web première installation (:8900) |
| `herminas run` | Démarre tous les services (ordre supervisé) |
| `herminas status` | État de chaque service |
| `herminas doctor` | Diagnostic complet |
| `herminas agent install` | Installe l'agent sur une machine source |
| `herminas backup` / `restore` | Sauvegarde métadonnées + config |
| `herminas update` | Mise à jour bundle |

---

## 10. Planning et Jalons

### 10.1 Phases de développement

| Phase | Durée | Livrable | Focus |
|-------|-------|----------|-------|
| **M0 — Fondation** | 3 sem | Monorepo L0-L3, CI 4 langages, contrats gRPC, supervisor embarque CH+RP | Go + Rust + Python + TS squelette |
| **M1 — Ingestion→Requête** | 4 sem | Agent Rust → Redpanda → sink ClickHouse → Query Studio SQL | Chaîne de valeur minimale |
| **M2 — Dashboards temps réel** | 3 sem | Widgets WebSocket, Overview, Datasets UI | React + Go |
| **M3 — Stream processing** | 4 sem | Pipelines déclaratifs, fenêtres, agrégats, DLQ | Rust |
| **M4 — Intelligence** | 4 sem | NL→SQL (FR/EN), anomalies online, train→ONNX→Rust | Python + Rust |
| **M5 — Alerting** | 2 sem | Règles, canaux (email/webhook/Telegram), historique | Go |
| **M6 — Verticale fraude** | 3 sem | Modèle fraude, règles WASM, dashboards fintech prêts à l'emploi | Python + Rust + React |
| **M7 — Admin & gouvernance** | 3 sem | RBAC, quotas, audit, masquage PII, rapports | Go + React |
| **M8 — Bundle & agents** | 3 sem | Bundle Linux/Docker, gestion flotte agents, setup wizard | Go + Rust |
| **M9 — Release** | 3 sem | Benchmarks, hardening, doc, pilote client | Tous |

**Total : ~8 mois** pour v1.0. **MVP démontrable en fin de M2** (~10 semaines) : donnée qui coule d'un agent vers un dashboard temps réel avec SQL.

### 10.2 MVP (fin M2)

> **Agent → ingestion → stockage → requête SQL → dashboard temps réel**, installable en < 15 minutes.

Stack MVP : Rust (agent + sink), Go (supervisor + query broker + API + WS), ClickHouse + Redpanda embarqués, React (Query Studio + 1 dashboard).

**Benchmark MVP** : 50k evt/s ingérés, visibles en requête < 2 s, sur le profil Standard.

### 10.3 Jalons de validation intermédiaires

| Jalon | Critère de sortie |
|-------|-------------------|
| Fin M1 | `herminas run` démarre tout ; un log tail arrive dans ClickHouse ; SELECT depuis l'UI |
| Fin M2 | Dashboard temps réel < 500ms de rafraîchissement ; MVP démontrable client |
| Fin M4 | « montre-moi les erreurs de la dernière heure » (FR) retourne les bonnes lignes ; une anomalie injectée est détectée < 500ms |
| Fin M6 | Démo fraude bout-en-bout : transaction suspecte → score → alerte → dashboard |
| Fin M9 | 1 pilote client en production ; checklist v1.0 verte |

---

## 11. Leçons apprises d'aNtaerus

| Leçon aNtaerus | Application HermiNas |
|----------------|----------------------|
| Couches L0-L3 strictes dès le départ | Reprises telles quelles, validées par CI (import-linter, contrats Go/Rust) |
| Config immuable, 1 seule source | `herminas.yaml` lu au boot, structs immuables dans les 3 langages |
| Secrets typés + test anti-fuite | `SecretString`/`SecretStr`/`secrecy` + `test_secrets_no_leak` dès M0 |
| Composition root unique | `bootstrap.go/.rs/.py` mirrors |
| Services testables sans dépendances externes | Mocks ClickHouse/Redpanda pour tests unitaires ; intégration en lane lourde |
| Bundle relocalisable, binaires statiques | Go + Rust statiques, Python venv autonome, moteurs embarqués versionnés |
| CI 2 lanes (rapide/lourde) | Reprise du modèle : lint+unit sur push, intégration+bench sur main |
| Ne pas réécrire l'irremplaçable (Whisper/Piper) | ClickHouse/Redpanda = boîtes noires supervisées, jamais forkées |
| Suivi tasks.md au niveau sous-tâche | Même discipline documentaire (règles de pilotage identiques) |

---

## 12. Glossaire

| Terme | Définition |
|-------|------------|
| **HermiNas** | Nom du produit. Plateforme souveraine de données temps réel |
| **Agent** | Binaire Rust léger déployé sur les machines sources |
| **Data plane** | Chemin chaud de la donnée (Rust) : ingestion, transform, scoring, sinks |
| **Control plane** | Orchestration (Go) : supervision, API, pipelines, alerting, auth |
| **Pipeline** | Définition déclarative source → transforms → sink |
| **DLQ** | Dead Letter Queue — événements rejetés/invalides |
| **NL→SQL** | Traduction de langage naturel en SQL avec garde-fous |
| **ONNX** | Format d'échange de modèles ML (Python train → Rust infer) |
| **WASM Rules** | Règles clients compilées en WebAssembly, sandboxées dans le flux |
| **MergeTree** | Famille de moteurs de tables ClickHouse |
| **EWMA** | Moyenne mobile pondérée exponentiellement (détection anomalies) |
| **CDC** | Change Data Capture — capture des changements d'une base source |
| **TTL** | Time To Live — rétention automatique des données |
| **mTLS** | TLS mutuel (agent ↔ serveur) |
| **Air-gapped** | Déploiement sans aucun accès Internet (LLM local requis) |

---

## Annexes

### A. Matrice RACI

| Domaine | Responsable | Consulté | Informé |
|---------|-------------|----------|---------|
| Architecture | Architecte | Équipe | PO |
| Data plane Rust | Rust lead | Architecte | QA |
| Control plane Go | Go lead | DevOps | PO |
| Intelligence Python | Python/ML lead | Data scientist | PO |
| UI/UX | Frontend lead | Designer | PO |
| Moteurs embarqués | DevOps | Go lead | Équipe |
| Sécurité/conformité | Security lead | Tous | Direction |
| Verticale fraude | PO + ML lead | Client pilote | Direction |

### B. Checklist validation v1.0

- [ ] Tous les P0 implémentés et testés
- [ ] Couverture tests > 80% (Go, Rust, Python)
- [ ] Ingestion soutenue ≥ 100k evt/s (profil Standard)
- [ ] Latence stream p99 < 5 ms ; scoring ONNX p99 < 2 ms
- [ ] Ingestion → requêtable < 2 s
- [ ] NL→SQL : ≥ 90% de requêtes valides sur le jeu d'éval FR/EN
- [ ] Anomalie injectée → alerte < 500 ms
- [ ] Secrets jamais dans les logs (test anti-fuite vert)
- [ ] Installation bundle Linux < 15 min chrono
- [ ] Mode air-gapped validé (Ollama local)
- [ ] Audit append-only vérifié
- [ ] 1 pilote client en production
- [ ] Documentation complète (installation, API, user guide, runbook)
- [ ] Licence choisie (open core envisagé : cœur ouvert + verticale fraude commerciale)

---

**Document rédigé le 19 juillet 2026**

**HermiNas** — *Vos données. Chez vous. En temps réel.*
