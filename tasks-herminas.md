# Liste complète des tâches — HermiNas

## Règles de pilotage documentaire

- `tasks-herminas.md` est le backlog principal et la source de vérité opérationnelle du projet.
- `cahier-des-charges-herminas.md` est la référence stable de vision produit, d'architecture cible et de contraintes.
- Toute avancée validée doit entraîner une mise à jour immédiate de ce fichier au niveau de la sous-tâche, avec un bloc « État actuel » listant les fichiers matérialisés et les validations rejouées.
- Aucune case ne se coche sans validation exécutée (test, lint, benchmark ou smoke test).

---

## Phase M0 — Fondation (3 semaines)

### M0.1 — Architecture & Bootstrap
- [x] Définir la structure monorepo `herminas/` avec les 4 couches (L0-L3)
- [x] Créer `kernel/` (L0) : `contracts.go`, `contracts.rs`, `contracts.py`, `errors/`, `paths/` (`schemas/` reporté à M0.3, cf. note ci-dessous)
- [x] Implémenter `settings/` L0 : config immuable Go (Viper), `SecretStr` Python (pydantic), `secrecy` Rust
- [x] Créer `permissions/` L0 : rôles viewer/analyst/engineer/admin
- [x] Écrire `bootstrap.go` : composition root Go (control plane)
- [x] Écrire `bootstrap.rs` : composition root Rust (data plane)
- [x] Écrire `bootstrap.py` : composition root Python (intelligence)
- [x] Définir les Protocols/Interfaces entre couches (Go interfaces, Rust traits, Python Protocols)
- [x] **Règle critique** : zéro fichier hors package `herminas/` (pas de `config/` racine, pas de `main.py` racine)

**État actuel (2026-08-01)**

Décision de layout : le dossier racine du dépôt (déjà nommé `HermiNas/`) fait office de package `herminas/` — pas de sous-dossier `herminas/herminas/` redondant. Chaque langage a sa racine d'outillage à plat sous ce même dossier (Go via `go.mod` racine, Rust via `rust/` workspace, Python via `python/` projet), toutes trois construisant sur le même concept L0 kernel.

Fichiers matérialisés :
- Go : `go.mod`, `bootstrap.go`, `kernel/{contracts,errors,paths,permissions,settings}/*.go`
- Rust : `rust/Cargo.toml` (workspace), `rust/kernel/` (crate `herminas-kernel` : `contracts.rs`, `errors.rs`, `paths.rs`, `permissions.rs`, `settings.rs`), `rust/bootstrap/` (crate bin `herminas-dataplane`, fichier `src/bootstrap.rs`)
- Python : `python/pyproject.toml`, `python/src/herminas_kernel/{contracts,errors,paths,permissions,settings}.py`, `python/src/herminas_intelligence/bootstrap.py`
- Config dev : `config/herminas.example.yaml`

Protocols/Interfaces inter-couches livrés : `HealthChecker` (interface Go / trait Rust / `Protocol` Python) + type `Event` partagé conceptuellement dans les trois `contracts.*`.

`kernel/schemas/` (JSON Schemas WebSocket) et `kernel/proto/` sont explicitement scopés M0.3 dans le cahier des charges — non créés ici pour éviter des dossiers vides sans contenu réel.

Validations rejouées :
- `go build ./...`, `go vet ./...`, `go test ./...` → PASS (3 packages testés : `paths`, `permissions`, `settings`)
- `go run bootstrap.go` → `HermiNas control plane bootstrap OK`
- `cargo build --workspace`, `cargo clippy --workspace --all-targets`, `cargo test --workspace` → PASS (3 tests kernel)
- `./rust/target/debug/herminas-dataplane` → `HermiNas data plane bootstrap OK`
- `pip install -e ".[dev]"` (venv `python/.venv`) + `pytest` → PASS (4 tests)
- `python -m herminas_intelligence.bootstrap` → `HermiNas intelligence bootstrap OK`

### M0.2 — CI/CD & Tooling
- [x] Initialiser repo Git avec `.gitignore` robuste (pas de secrets, pas de `data/`, pas de `bundle/`)
- [ ] Configurer GitHub Actions : lane rapide (push/PR) + lane lourde (main + hebdo)
- [ ] Intégrer `ruff` + `mypy` + `import-linter` (Python, contrats de couches)
- [ ] Intégrer `golangci-lint` (Go)
- [ ] Intégrer `clippy` + `cargo fmt` + `cargo check` (Rust)
- [ ] Intégrer `eslint` + `prettier` + `tsc --noEmit` (TypeScript)
- [ ] Configurer `pytest` : suite unitaire + suite intégration séparées
- [ ] Créer `Taskfile.yml` : `task test`, `task lint`, `task typecheck`, `task build`, `task bench`
- [ ] Configurer `pre-commit` hooks
- [ ] Créer `scripts/validation/` : smoke tests démarrage froid par service (`.sh` + `.ps1`) — `test-secrets-no-leak.sh` livré (M0.4), smoke tests de démarrage par service restent à faire

**État actuel (2026-08-01)** : `git init` fait, `.gitignore` couvre `target/`, `.venv/`, `node_modules/`, `data/`, `bundle/`, `*.enc`, `.env`. Reste de M0.2 (GitHub Actions, linters par langage, Taskfile.yml, pre-commit) non commencé.

### M0.3 — Contrats gRPC & communication
- [ ] Définir `kernel/proto/dataplane.proto` : DeployPipeline, StopPipeline, GetPipelineStats, DeployModel, DeployRule, StreamEvents, GetHealth
- [ ] Définir `kernel/proto/intelligence.proto` : TranslateNL, TrainModel, ExportONNX, Forecast, GetHealth
- [ ] Définir `kernel/proto/agent.proto` : ShipBatch, GetAgentConfig, Heartbeat
- [ ] Générer les stubs Go (`protoc-gen-go` + `protoc-gen-go-grpc`)
- [ ] Générer les stubs Rust (`tonic-build`)
- [ ] Générer les stubs Python (`grpcio-tools`)
- [ ] Définir les schémas JSON WebSocket Go ↔ React (`kernel/schemas/`)
- [ ] Implémenter le bus d'événements interne Go (channels + goroutines)
- [ ] Benchmark latence Go ↔ Rust gRPC (< 5ms) avec script de validation
- [ ] Benchmark latence Go ↔ Python gRPC (< 50ms) avec script de validation

### M0.4 — Sécurité fondamentale
- [x] Implémenter `SecretString` Go (marshal masqué : String, GoString, MarshalJSON, MarshalText)
- [x] Implémenter `SecretStr` Python (pydantic, modèle figé)
- [x] Implémenter `secrecy::SecretString` Rust
- [x] Écrire le test anti-fuite `test_secrets_no_leak` (grep `sk-`, credentials, exclusions caches)
- [ ] Implémenter chiffrement au repos AES-256-GCM (`secrets.enc`)
- [ ] Générer et gérer les certificats mTLS agent ↔ serveur (autorité interne)
- [x] Documenter `docs/security/SECRETS.md` : typage, immuabilité runtime, rotation

**État actuel (2026-08-01)** : typage des secrets fait et testé dans les 3 langages (voir État actuel M0.1 pour les fichiers). Script `scripts/validation/test-secrets-no-leak.sh` exécuté sur le dépôt entier → `OK`. Chiffrement AES-256-GCM et mTLS agent↔serveur restent à faire (dépendent de M0.5/M1.1, pas encore de bundle ni d'agent).

### M0.5 — Supervisor & moteurs embarqués
- [x] Implémenter `engine/supervisor/` Go : cycle de vie processus enfants (start/stop/restart/backoff)
- [x] Intégrer ClickHouse embarqué : téléchargement versionné, config XML générée depuis `herminas.yaml`
- [ ] Intégrer Redpanda embarqué : config générée, mode single-node (config + healthcheck faits et testés ; binaire réel non lançable sur macOS, cf. note)
- [x] Implémenter healthchecks ClickHouse (HTTP ping + requête témoin)
- [ ] Implémenter healthchecks Redpanda (metadata request) — fait via l'API admin HTTP (`/v1/status/ready`) plutôt qu'une requête metadata protocole Kafka natif ; fonctionnellement équivalent et plus simple côté Go (pas de client Kafka requis), à confirmer si un vrai metadata request est préféré plus tard
- [ ] Implémenter démarrage ordonné : Redpanda → ClickHouse → Rust → Python → API (ordre générique implémenté et testé ; seuls Redpanda+ClickHouse sont câblés aujourd'hui, cf. note)
- [x] Implémenter arrêt propre (drain puis stop, ordre inverse)
- [x] Créer mocks ClickHouse/Redpanda pour tests unitaires (aucune dépendance externe en lane rapide)
- [x] Smoke test : `herminas run` démarre tous les services, `herminas status` les liste verts (validé pour ClickHouse en conditions réelles ; Redpanda validé via mocks, cf. note)

**État actuel (2026-08-01)**

Fichiers matérialisés :
- `engine/supervisor/supervisor.go` + `exec_process.go` : `Process` (interface), `ExecProcess` (subprocess supervisé avec restart-backoff), `Supervisor` (démarrage ordonné avec attente de santé, arrêt en ordre inverse, `Status()` sans dépendance à un état en mémoire)
- `engine/clickhouse/clickhouse.go` : génération `config.xml`/`users.xml`, téléchargement du binaire officiel (macOS/Linux), `NewProcess`, healthcheck HTTP `/ping` + requête témoin `SELECT 1`
- `engine/redpanda/redpanda.go` : génération `redpanda.yaml` (single-node), `Download` (échoue proprement hors Linux avec message explicite), `NewProcess`, healthcheck API admin `/v1/status/ready`
- `bootstrap.go` étendu : sous-commandes `run` (démarrage supervisé, ordre Redpanda→ClickHouse, bloque jusqu'à SIGINT/SIGTERM) et `status` (health-check sans état partagé, fonctionne depuis un process séparé)

**Limite de plateforme (honnête, pas contournée)** : Redpanda ne publie que des binaires Linux. Sur cette machine de dev (macOS, pas de Docker disponible), `redpanda.Download` échoue avec un message clair au lieu d'échouer silencieusement plus tard ; la génération de config et le healthcheck sont testés via un serveur HTTP mock (`mockRedpandaAdminServer`). L'intégration réelle du binaire Redpanda sera validée sur cible Linux (CI ou bundle, M8.1). `bootstrap.go` détecte `runtime.GOOS` et n'enregistre Redpanda dans le supervisor que sur Linux ; sur macOS, seul ClickHouse est supervisé et un message explicite est loggé.

**Rust/Python/API non câblés dans l'ordre de démarrage** : les binaires `herminas-dataplane` (Rust) et `herminas_intelligence.bootstrap` (Python) issus de M0.1 sont des smoke tests qui s'exécutent puis se terminent — pas des démons. Les enregistrer dans le supervisor aujourd'hui provoquerait une boucle de redémarrage infinie. Ils rejoindront l'ordonnancement une fois qu'ils exposent un vrai service long-running (serveur gRPC Rust en M1.2, FastAPI Python en M4, API Go en M1.5). La capacité générique d'ordonnancement est déjà implémentée et testée (`TestStartRunsInOrderAndWaitsForHealth`, `TestStopRunsInReverseOrder`).

Validations rejouées :
- `go build ./...`, `go vet ./...`, `go test ./...` → PASS (17 tests au total : 5 supervisor, 4 clickhouse, 4 redpanda, 4 kernel)
- **Smoke test réel** (pas seulement mocké) : téléchargement du vrai binaire ClickHouse 26.8.1 (macOS x86_64, 160 Mo), `herminas-cp run` → ClickHouse démarre réellement, devient healthy en ~3-10s (ping + `SELECT 1` via HTTP), `herminas-cp status` depuis un processus séparé confirme `healthy`, `SIGTERM` → arrêt propre confirmé (« supervisor: stopped cleanly », process terminé)
- `scripts/validation/test-secrets-no-leak.sh` → OK
- Binaire ClickHouse téléchargé sous `runtime/` (ajouté à `.gitignore`, ~700 Mo avec les données de test — jamais commité)

---

## Phase M1 — Ingestion → Requête (4 semaines)

### M1.1 — Agent Rust (collecte)
- [ ] Implémenter `agent/collect/file.rs` : tail de fichiers logs (rotation, multiline, checkpoint)
- [ ] Implémenter `agent/collect/http.rs` : endpoint HTTP/JSON local (axum)
- [ ] Implémenter `agent/parse/` : JSON, CSV, regex nommées (grok-like)
- [ ] Implémenter `agent/buffer/wal.rs` : write-ahead log disque, reprise après crash
- [ ] Implémenter `agent/ship/` : batching, compression zstd, envoi gRPC mTLS
- [ ] Implémenter backpressure : ralentissement collecte si WAL > seuil
- [ ] Implémenter config agent YAML + rechargement à la demande (commande serveur)
- [ ] Binaire statique < 20 MB (musl), test empreinte RAM < 50 MB sous charge
- [ ] Tests : coupure réseau simulée → zéro perte après reconnexion

### M1.2 — Réception serveur & bus (Rust)
- [ ] Implémenter `stream/receiver.rs` : serveur gRPC réception batches agents
- [ ] Implémenter validation schéma + horodatage + enrichissement métadonnées (agent_id, dataset)
- [ ] Implémenter `sink/redpanda.rs` : production vers topics par dataset (rdkafka)
- [ ] Implémenter `sink/clickhouse.rs` : consommation topics → batchs RowBinary → async inserts
- [ ] Implémenter DLQ : événements invalides vers topic dédié + compteurs
- [ ] Idempotence de base : clé de déduplication par batch
- [ ] Benchmark : ≥ 100k evt/s soutenus en local (profil dev), script `bench-ingest.sh`

### M1.3 — Schémas & datasets (Go)
- [ ] Implémenter `engine/schemamgr/` : modèle dataset (nom, colonnes, types, TTL) en SQLite
- [ ] Générer DDL ClickHouse MergeTree (ORDER BY, PARTITION BY date, TTL)
- [ ] Implémenter création/évolution de dataset via API (`POST /api/v1/datasets`)
- [ ] Implémenter registry de schémas versionné (compatibilité ascendante)
- [ ] Auto-création de dataset à la première ingestion (option, schéma inféré)

### M1.4 — Query broker (Go)
- [ ] Implémenter `engine/querybroker/` : exécution SQL sur ClickHouse (HTTP), timeout, annulation
- [ ] Implémenter streaming des résultats (chunks) vers l'API
- [ ] Implémenter quotas basiques : requêtes/min par utilisateur, max lignes scannées
- [ ] Implémenter cache résultats court TTL pour requêtes identiques
- [ ] Journaliser chaque requête dans l'audit (append-only)

### M1.5 — API & Auth (Go)
- [ ] Implémenter serveur HTTP/2 + TLS optionnel + CORS dev
- [ ] Implémenter JWT (sessions UI) + tokens API révocables scopés
- [ ] Implémenter RBAC middleware (4 rôles)
- [ ] Implémenter routes : `/api/v1/query`, `/api/v1/datasets`, `/api/v1/ingest/{dataset}`, `/api/v1/health`
- [ ] Implémenter rate limiting HTTP par IP/token
- [ ] Tests : `go test ./...` couvrant auth, quotas, query broker

### M1.6 — Query Studio (React)
- [ ] Initialiser `interfaces/web/` : Vite + React + TypeScript + Zustand + TanStack Query
- [ ] Implémenter layout applicatif (navigation, thème clair/sombre)
- [ ] Implémenter `pages/QueryStudio.tsx` : Monaco Editor SQL + exécution + tableau résultats
- [ ] Implémenter autocomplétion SQL (datasets + colonnes depuis l'API)
- [ ] Implémenter historique de requêtes (local + serveur)
- [ ] Implémenter export CSV/JSON des résultats
- [ ] Implémenter `pages/Login.tsx` + gestion session JWT
- [ ] Build statique servi par Go (fallback SPA)
- [ ] Tests composants : Query Studio, auth, table résultats

### M1.7 — Validation jalon M1
- [ ] Smoke test bout-en-bout : agent tail un fichier log → Redpanda → ClickHouse → SELECT depuis Query Studio
- [ ] Latence ingestion → requêtable < 2 s vérifiée par script
- [ ] `herminas doctor` : diagnostic complet vert

---

## Phase M2 — Dashboards temps réel (3 semaines)

### M2.1 — WebSocket & data feed (Go)
- [ ] Implémenter hub WebSocket (goroutine par client, heartbeat)
- [ ] Implémenter souscriptions widgets : requête + intervalle → push `dashboard.data`
- [ ] Implémenter mode streaming : requêtes incrémentales (depuis dernier timestamp)
- [ ] Implémenter événements `system.health` et `agent.status`
- [ ] Rate limiting messages WebSocket

### M2.2 — Dashboards UI (React)
- [ ] Implémenter `pages/Dashboards.tsx` : grille responsive de widgets (drag/resize)
- [ ] Implémenter widgets ECharts : ligne temporelle, barres, jauge, compteur, table, heatmap
- [ ] Implémenter éditeur de widget : requête SQL + type + options + intervalle
- [ ] Implémenter sauvegarde dashboards (SQLite via API)
- [ ] Implémenter `pages/Overview.tsx` : santé plateforme, volumes ingérés, top datasets
- [ ] Implémenter `pages/Datasets.tsx` : liste, schéma, volumes, TTL, édition rétention
- [ ] Implémenter `hooks/useLiveWidget.ts` : souscription WebSocket + fallback polling

### M2.3 — Validation jalon M2 (MVP)
- [ ] Dashboard temps réel avec rafraîchissement < 500 ms sur données qui coulent
- [ ] Benchmark MVP : 50k evt/s ingérés, visibles < 2 s, profil Standard
- [ ] Parcours démo scripté : installation → première source → premier dashboard en < 15 min
- [ ] Démo enregistrable pour prospects

---

## Phase M3 — Stream processing (4 semaines)

### M3.1 — Moteur de pipelines (Rust)
- [ ] Définir le format de pipeline déclaratif (YAML) : sources, transforms, sinks
- [ ] Implémenter `stream/pipeline.rs` : graphe d'opérateurs, exécution Tokio
- [ ] Implémenter représentation in-flight Arrow (RecordBatch)
- [ ] Implémenter transforms : filter (expressions), map/rename, parse, cast
- [ ] Implémenter enrichissement : lookup table référentielle (chargée depuis ClickHouse)
- [ ] Implémenter fenêtres tumbling et sliding (event-time, watermarks simples)
- [ ] Implémenter agrégations : count, sum, avg, min/max, percentiles approx
- [ ] Implémenter DLQ par pipeline + métriques (in/out/err/latence)
- [ ] Benchmark : latence p99 < 5 ms/événement, script `bench-stream.sh`

### M3.2 — Gestion des pipelines (Go)
- [ ] Implémenter `engine/pipelinemgr/` : CRUD pipelines en SQLite, versionnage
- [ ] Implémenter compilation config → PipelineSpec protobuf → `DeployPipeline` gRPC
- [ ] Implémenter statuts et stats en continu (`GetPipelineStats` → WebSocket `pipeline.status`)
- [ ] Implémenter pause/reprise/redéploiement sans perte (drain)

### M3.3 — Pipelines UI (React)
- [ ] Implémenter `pages/Pipelines.tsx` : liste, statuts, métriques par pipeline
- [ ] Implémenter éditeur de pipeline : formulaire structuré (source → transforms → sink) + vue YAML
- [ ] Implémenter visualisation du flux (débit in/out, erreurs, latence)
- [ ] Implémenter dry-run sur échantillon avant déploiement

### M3.4 — Connecteurs control plane (Go)
- [ ] Implémenter connecteur SQL polling (PostgreSQL/MySQL) → ingestion
- [ ] Implémenter connecteur S3/MinIO import batch (CSV/JSON/Parquet)
- [ ] Implémenter connecteur REST polling planifié
- [ ] Tests d'intégration connecteurs (lane lourde)

---

## Phase M4 — Intelligence (4 semaines)

### M4.1 — NL→SQL (Python)
- [ ] Implémenter `nlsql/context.py` : construction du contexte schéma (datasets, colonnes, exemples)
- [ ] Implémenter `nlsql/translator.py` : prompt + appel litellm (Ollama local / Anthropic / Mistral)
- [ ] Implémenter `nlsql/guard.py` : validation AST SQL (SELECT only, LIMIT forcé, datasets autorisés)
- [ ] Implémenter explication de la requête générée (retour utilisateur)
- [ ] Constituer le jeu d'évaluation FR/EN (≥ 100 questions annotées)
- [ ] Atteindre ≥ 90% de requêtes valides sur le jeu d'éval, script `eval-nlsql.py`
- [ ] Exposer `TranslateNL` en gRPC + route Go `/api/v1/nlquery`
- [ ] UI : onglet langage naturel dans Query Studio (question → SQL affiché → résultats)

### M4.2 — Anomalies online (Rust)
- [ ] Implémenter `anomaly/zscore.rs` : z-score glissant par série
- [ ] Implémenter `anomaly/ewma.rs` : EWMA + bandes dynamiques
- [ ] Implémenter détection sur agrégats fenêtrés (sorties de pipelines)
- [ ] Émettre événements `anomaly.detected` via `StreamEvents` gRPC → Go → WebSocket
- [ ] Test : anomalie injectée détectée < 500 ms

### M4.3 — ML train → ONNX → Rust (Python + Rust)
- [ ] Implémenter `ml/anomaly/train.py` : IsolationForest/OneClassSVM par dataset
- [ ] Implémenter `ml/export/onnx.py` : conversion skl2onnx + test de parité (mêmes prédictions)
- [ ] Implémenter `score/onnx.rs` : chargement session `ort`, scoring en vol dans pipeline
- [ ] Implémenter `DeployModel` gRPC : artefact ONNX → data plane, hot swap
- [ ] Benchmark scoring p99 < 2 ms/événement
- [ ] Implémenter `ml/forecast/` : prévisions volumes (statsforecast) + route API

### M4.4 — ML Studio (React)
- [ ] Implémenter `pages/MLStudio.tsx` : liste modèles, statuts, métriques d'entraînement
- [ ] Implémenter lancement d'entraînement + suivi progression (stream gRPC → WS)
- [ ] Implémenter déploiement ONNX en 1 clic + rollback

---

## Phase M5 — Alerting (2 semaines)

### M5.1 — Moteur d'alertes (Go)
- [ ] Implémenter modèle de règle : requête/seuil/fenêtre/sévérité (SQLite)
- [ ] Implémenter scheduler d'évaluation périodique (requêtes ClickHouse)
- [ ] Brancher les événements `anomaly.detected` du data plane sur les règles
- [ ] Implémenter déduplication + groupement + silences (maintenance)
- [ ] Implémenter canaux : email (SMTP), webhook, Telegram
- [ ] Historiser les alertes dans ClickHouse (dataset système)

### M5.2 — Alerting UI (React)
- [ ] Implémenter `pages/Alerting.tsx` : règles CRUD, test d'envoi, canaux
- [ ] Implémenter historique + statistiques (fréquence, MTTA)
- [ ] Notifications in-app temps réel (`alert.fired` WebSocket)

---

## Phase M6 — Verticale fraude (3 semaines)

### M6.1 — Modèle & features (Python)
- [ ] Définir le schéma transactionnel de référence (mobile money : montant, canal, agent, device, geo, horodatage)
- [ ] Implémenter feature engineering : vélocité, récurrence, écarts au profil, heures inhabituelles
- [ ] Entraîner modèle fraude de base (supervisé si labels, sinon anomalies) + évaluation (precision/recall)
- [ ] Export ONNX + parité + déploiement dans le pipeline

### M6.2 — Règles WASM (Rust)
- [ ] Définir l'ABI stable des règles : event in → verdict/score/enrichment out
- [ ] Implémenter runtime `rules/wasmtime.rs` : fuel metering, mémoire bornée, timeout
- [ ] Implémenter `DeployRule` gRPC + validation dry-run sur échantillon
- [ ] Fournir template de règle (Rust → WASM) + doc + 3 règles d'exemple (seuil montant, vélocité, liste noire)

### M6.3 — Pack fintech (React + Go)
- [ ] Dashboards prêts à l'emploi : supervision transactions, top alertes, carte des scores
- [ ] Pipeline fraude préconfiguré : ingestion → features → score ONNX → règles WASM → alertes
- [ ] Démo bout-en-bout scriptée : transaction suspecte → score → alerte → dashboard (< 1 s)
- [ ] Documentation verticale : guide d'intégration fintech, cadre de conformité configurable par juridiction

---

## Phase M7 — Admin & gouvernance (3 semaines)

### M7.1 — RBAC & quotas (Go)
- [ ] Implémenter gestion utilisateurs/rôles complète (CRUD, invitations)
- [ ] Implémenter quotas fins par rôle et par token (requêtes, lignes scannées, exports)
- [ ] Implémenter tokens API scopés par dataset avec expiration

### M7.2 — Audit & PII (Go + ClickHouse)
- [ ] Implémenter journal d'audit append-only (requêtes, exports, admin, auth)
- [ ] Implémenter masquage de colonnes PII par rôle (vue générée)
- [ ] Implémenter purge prouvable (TTL + rapport de purge)
- [ ] Implémenter rapports de conformité exportables (PDF) : accès, volumes, rétentions

### M7.3 — Admin UI (React)
- [ ] Implémenter `pages/Admin.tsx` : utilisateurs, rôles, tokens, quotas
- [ ] Implémenter consultation d'audit avec filtres
- [ ] Implémenter `pages/SystemHealth.tsx` : état Go/Rust/Python/ClickHouse/Redpanda, logs, restart service

---

## Phase M8 — Bundle & agents (3 semaines)

### M8.1 — Bundle & CLI (Go)
- [ ] Implémenter CLI `herminas` : setup, run, status, doctor, backup, restore, update
- [ ] Construire bundle Linux `.tar.gz` auto-contenu (binaires statiques + venv + moteurs)
- [ ] Générer units systemd à l'installation
- [ ] Construire image Docker Compose (évaluation/dev)
- [ ] Bundle Windows Server `.zip` + services Windows
- [ ] Chemins relatifs partout (bundle relocalisable, leçon aNtaerus)

### M8.2 — Setup Wizard (React + Go)
- [ ] Implémenter `pages/Setup.tsx` : admin initial, TLS, LLM (local/cloud + clés), première source
- [ ] Chronométrer le parcours : installation → premier dashboard < 15 min
- [ ] Implémenter `herminas doctor` complet : ports, disque, mémoire, versions moteurs

### M8.3 — Gestion flotte d'agents
- [ ] Implémenter `herminas agent install` (script + binaire, Linux/Windows)
- [ ] Implémenter enregistrement agent (mTLS bootstrap par token)
- [ ] Implémenter `pages/Agents.tsx` : flotte, état, dernière activité, config à distance
- [ ] Implémenter mise à jour de config agent poussée depuis le serveur

---

## Phase M9 — Release (3 semaines)

### M9.1 — Tests & qualité
- [ ] Couverture > 80% Go, Rust, Python (rapports en CI)
- [ ] Suite d'intégration bout-en-bout (≥ 20 scénarios) en lane lourde
- [ ] Tests de charge : 100k evt/s soutenus 1 h sans dégradation, profil Standard
- [ ] Tests de résilience : kill ClickHouse/Redpanda/Rust → supervisor restaure, zéro perte (WAL)
- [ ] Test air-gapped complet (Ollama local, aucune connexion sortante)
- [ ] Audit sécurité interne : authz, injection SQL (broker + NL→SQL guard), secrets, sandbox WASM
- [ ] Test anti-fuite secrets vert sur tout le dépôt

### M9.2 — Documentation
- [ ] Guide d'installation (bundle Linux, Docker, Windows)
- [ ] Guide utilisateur (Query Studio, dashboards, pipelines, alerting, NL→SQL)
- [ ] Guide administrateur (RBAC, quotas, audit, backup/restore, runbook incidents)
- [ ] Référence API REST + WebSocket
- [ ] Guide développeur de règles WASM (template + ABI)
- [ ] Guide verticale fintech (intégration, conformité)
- [ ] Architecture Decision Records (ADR) des choix majeurs

### M9.3 — Release & pilote
- [ ] Checklist validation v1.0 du cahier des charges entièrement verte
- [ ] Benchmarks publiés (ingestion, latences, NL→SQL éval)
- [ ] Choix de licence acté (open core : cœur ouvert, verticale fraude commerciale)
- [ ] Déploiement pilote chez 1 client (fintech cible) + période d'observation 2 semaines
- [ ] Rétrospective pilote + backlog v1.1
- [ ] Tag `v1.0.0` + bundles publiés

---

## Tâches transversales (tout au long du projet)

### Documentation continue
- [ ] Mettre à jour ce fichier au niveau sous-tâche après chaque lot validé (bloc « État actuel »)
- [ ] Tenir les ADR à jour à chaque décision structurante

### Sécurité continue
- [ ] Rejouer `test_secrets_no_leak` à chaque lot
- [ ] Dépendances scannées (dependabot / cargo audit / pip-audit / npm audit) en lane lourde

### Performance continue
- [ ] Benchmarks `bench-ingest`, `bench-stream`, `bench-score` rejoués à chaque fin de phase
- [ ] Budget de régression : aucune latence p99 ne se dégrade de > 10% entre phases

---

## Résumé par phase

| Phase | Semaines | Tâches clés | Livrable |
|-------|----------|-------------|----------|
| M0 | 3 | Monorepo L0-L3, CI 4 langages, gRPC, supervisor + moteurs embarqués | Squelette compilable, `herminas run` vert |
| M1 | 4 | Agent Rust, Redpanda→ClickHouse, datasets, query broker, Query Studio | **Chaîne ingestion → requête** |
| M2 | 3 | WebSocket feed, dashboards ECharts, Overview, Datasets | **MVP démontrable (< 15 min d'install)** |
| M3 | 4 | Pipelines déclaratifs, fenêtres, agrégats, connecteurs | Stream processing opérationnel |
| M4 | 4 | NL→SQL FR/EN, anomalies online, train→ONNX→Rust, ML Studio | **Intelligence intégrée** |
| M5 | 2 | Règles d'alerte, canaux, historique | Alerting complet |
| M6 | 3 | Modèle fraude, règles WASM, pack fintech | **Verticale fraude démontrable** |
| M7 | 3 | RBAC, quotas, audit, PII, rapports | Gouvernance entreprise |
| M8 | 3 | Bundle, CLI, setup wizard, flotte agents | Distribution production |
| M9 | 3 | Tests charge/résilience, doc, pilote client | **v1.0.0** |

---

**Total : ~230 tâches détaillées sur ~8 mois**
