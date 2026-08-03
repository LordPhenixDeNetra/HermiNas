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
- [x] Définir `kernel/proto/dataplane.proto` : DeployPipeline, StopPipeline, GetPipelineStats, DeployModel, DeployRule, StreamEvents, GetHealth
- [x] Définir `kernel/proto/intelligence.proto` : TranslateNL, TrainModel, ExportONNX, Forecast, GetHealth
- [x] Définir `kernel/proto/agent.proto` : ShipBatch, GetAgentConfig, Heartbeat
- [x] Générer les stubs Go (`protoc-gen-go` + `protoc-gen-go-grpc`)
- [x] Générer les stubs Rust (`tonic-build`)
- [x] Générer les stubs Python (`grpcio-tools`)
- [x] Définir les schémas JSON WebSocket Go ↔ React (`kernel/schemas/`)
- [x] Implémenter le bus d'événements interne Go (channels + goroutines)
- [x] Benchmark latence Go ↔ Rust gRPC (< 5ms) avec script de validation
- [x] Benchmark latence Go ↔ Python gRPC (< 50ms) avec script de validation

**État actuel (2026-08-02)**

Toolchains installées localement (mêmes principes que Go/Node en M0.1) : `protoc` 35.1 (`~/sdk/protoc`), `protoc-gen-go`/`protoc-gen-go-grpc` (`go install`), `grpcio`/`grpcio-tools` (venv Python). Les 4 `.proto` vivent sous `kernel/proto/` et sont la source de vérité unique pour les 3 langages : `common.proto` (HealthRequest/HealthStatus partagés) + `dataplane.proto`, `intelligence.proto`, `agent.proto`, tous générés avec `-I kernel/proto` et des imports par nom de fichier nu (`import "common.proto";`) pour rester cohérents entre `protoc`, `tonic-build` et `grpc_tools.protoc`.

Fichiers matérialisés :
- Go : `kernel/proto/{common,dataplane,intelligence,agent}pb/*.go` (générés)
- Rust : nouveau crate `rust/protocol/` (`herminas-protocol`) — `build.rs` compile les 4 `.proto` via `tonic-build` en deux passes (`common.proto` d'abord, puis les autres avec `extern_path(".herminas.common.v1", "crate::common::v1")` — la résolution relative automatique de prost échouait sur les préfixes de package partagés, cf. commentaire dans `build.rs`) ; `src/bin/dataplane_server.rs` implémente réellement `GetHealth`, le reste renvoie `UNIMPLEMENTED` avec un message pointant vers la phase qui l'implémentera (M3.2/M4.3/M6.2/M4.2)
- Python : `python/src/herminas_proto/*_pb2*.py` (générés, imports relatifs corrigés après coup — `grpc_tools.protoc` émet des imports absolus qui ne fonctionnent pas une fois le module empaqueté) ; `python/src/herminas_intelligence/grpc_server.py` implémente réellement `GetHealth`, le reste hérite du `UNIMPLEMENTED` par défaut du servicer généré
- `kernel/schemas/websocket/*.schema.json` : enveloppe + 7 schémas de payload (un par événement `cahier §6.2`), validés par `kernel/schemas/websocket_test.go` avec un vrai validateur JSON Schema (`santhosh-tekuri/jsonschema/v5`), pas juste un contrôle de syntaxe JSON
- `engine/bus/bus.go` : bus pub/sub générique (topics = noms d'événements WebSocket), `Publish` non bloquant (drop + compteur si un abonné est saturé), `Unsubscribe` idempotent
- `engine/health/grpc_bench_test.go` : bench réel Go→Rust et Go→Python, skip proprement si les serveurs ne tournent pas (lane rapide reste sans dépendance externe)

**Un vrai bug de concurrence trouvé par le race detector, pas avant** : la première version de `engine/bus` lisait la liste des abonnés sous `RLock` puis envoyait sur les canaux après avoir relâché le verrou — une `Unsubscribe` concurrente pouvait fermer un canal entre-temps, causant un `panic: send on closed channel`. `go test -race` l'a détecté immédiatement sur le test de concurrence. Corrigé en unifiant sur un seul `sync.Mutex` couvrant tout le cycle lecture+envoi ; revérifié avec `-race -count=5`, stable.

**RPCs au-delà de GetHealth non implémentés (attendu, pas un manque)** : `DeployPipeline`, `TrainModel`, etc. n'ont pas de logique métier — M0.3 ne demande que les contrats + la preuve que la plomberie gRPC fonctionne. Chaque méthode non implémentée renvoie `UNIMPLEMENTED` avec un message explicite plutôt qu'un succès simulé.

Validations rejouées :
- `go build ./...`, `go vet ./...`, `go test ./...` → PASS (nouveaux packages `engine/bus`, `engine/health`, `kernel/schemas`, `kernel/proto/*pb` inclus)
- `go test ./engine/bus/... -race -count=5` → PASS, aucune race
- `cargo build --workspace`, `cargo clippy --workspace --all-targets`, `cargo test --workspace` → PASS (crate `herminas-protocol` inclus)
- `pytest` (venv) → PASS
- **Benchmark gRPC réel** (serveurs Rust et Python réellement lancés, pas mockés) : Go↔Rust `GetHealth` p50=356µs p99=1.1ms (budget 5ms) ; Go↔Python `GetHealth` p50=426µs p99=0.9ms (budget 50ms) — les deux confortablement sous budget
- `scripts/validation/test-secrets-no-leak.sh` → OK

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
- [x] Implémenter `agent/collect/file.rs` : tail de fichiers logs (rotation, multiline, checkpoint)
- [x] Implémenter `agent/collect/http.rs` : endpoint HTTP/JSON local (axum)
- [x] Implémenter `agent/parse/` : JSON, CSV, regex nommées (grok-like)
- [x] Implémenter `agent/buffer/wal.rs` : write-ahead log disque, reprise après crash
- [ ] Implémenter `agent/ship/` : batching, compression zstd, envoi gRPC mTLS — batching+zstd faits et testés réellement ; transport gRPC mTLS non fait, cf. note
- [x] Implémenter backpressure : ralentissement collecte si WAL > seuil
- [ ] Implémenter config agent YAML + rechargement à la demande (commande serveur) — YAML fait et testé ; rechargement à la demande fait via SIGHUP (local), pas encore via commande serveur distante, cf. note
- [ ] Binaire statique < 20 MB (musl), test empreinte RAM < 50 MB sous charge — RAM validée réellement (< 50 MB) ; binaire natif macOS 4,4 Mo (< 20 MB) mais pas de build statique musl Linux, cf. note
- [x] Tests : coupure réseau simulée → zéro perte après reconnexion (via un `Shipper` défaillant simulé, cf. note)

**État actuel (2026-08-02)**

Fichiers matérialisés — nouveau crate `rust/agent/` (workspace member) :
- `src/collect/file.rs` : `FileTailer` — tail incrémental avec checkpoint persistant (reprise exacte après redémarrage), détection de rotation (changement d'inode), jointure multiligne par regex de début d'enregistrement
- `src/collect/http.rs` : serveur axum (`/ingest`, `/healthz`), accepte un ou plusieurs objets JSON newline-delimited
- `src/parse/{json,csv_format,regex_format}.rs` : les 3 formats demandés, `regex_format` couvre le besoin « grok-like » via les groupes nommés natifs de la crate `regex` (`(?P<name>...)`), sans dépendance grok dédiée
- `src/buffer/wal.rs` : WAL fsync à chaque append, fichier d'ack séparé, `pending()`/`ack()`/`compact()`
- `src/backpressure.rs` : délai croissant au-delà du seuil configuré, plafonné
- `src/ship/mod.rs` : trait `Shipper` + `FileShipper` (batching + compression zstd réels, framing longueur-préfixée)
- `src/main.rs` : boucle complète (tail + HTTP → parse → WAL → backpressure → ship périodique), reload de config sur SIGHUP, flush différé des enregistrements multilignes en attente après 4 polls inactifs (~2 s)

**Bug réel trouvé et corrigé pendant les tests** : la première implémentation de `push_line` mettait en attente la ligne courante même en mode non-multiligne, donc chaque ligne n'apparaissait qu'au poll suivant. Les 5 tests `collect::file` ont échoué immédiatement après écriture, révélant le bug ; corrigé, puis les 5 tests passent.

**Transport gRPC mTLS non fait (honnête, pas contourné)** : dépend du contrat `agent.proto` (M0.3, non fait) et du récepteur `stream/receiver.rs` (M1.2, non fait). `ship::Shipper` est le point d'extension : `FileShipper` fait tourner pour de vrai le chemin batching+compression zstd contre un fichier local ; un futur `GrpcShipper` implémentera le même trait sans rien changer en amont.

**Rechargement à la demande partiel** : `config::load` est rejouable (testé), et `main.rs` installe un handler SIGHUP qui recharge le fichier de config et logge — mais ne redémarre pas encore dynamiquement les tailers avec la nouvelle liste de sources. Le rechargement piloté par une commande serveur distante dépend de `GetAgentConfig` (M0.3) et de la gestion de flotte (M8.3).

**Binaire statique musl non fait** : cross-compiler vers `x86_64-unknown-linux-musl` depuis ce Mac demanderait un toolchain supplémentaire (`cargo-zigbuild` ou équivalent) non installé. Le build natif macOS release fait 4,4 Mo (3,6 Mo strippé), largement sous la barre des 20 Mo — mais ça ne valide pas le binaire musl Linux visé par le bundle (M8.1).

**Test « coupure réseau simulée »** : pas de vrai réseau à couper puisque le transport gRPC n'existe pas encore. À la place, `ship::tests::flaky_shipper_causes_zero_loss_until_it_recovers` utilise un `Shipper` qui échoue 2 fois avant de réussir, contre un vrai WAL : la garantie testée (rien n'est acqui­tté tant que le ship échoue, donc zéro perte) est exactement celle qu'un `GrpcShipper` devra respecter plus tard.

Validations rejouées :
- `cargo build --workspace`, `cargo clippy --workspace --all-targets` → PASS, zéro warning
- `cargo test --workspace` → PASS (31 tests `herminas-agent` + 3 `herminas-kernel`)
- **Smoke test réel bout-en-bout** : binaire `herminas-agent` lancé pour de vrai, événement initial de fichier + ligne ajoutée en cours de route + requête HTTP POST réelle → les 3 enregistrements retrouvés corrects après décompression zstd dans le fichier shippé (vérifié avec un exemple `decode_shipped` temporaire, supprimé après usage)
- **Test de charge réel** : 20 000 lignes fichier + 500 requêtes HTTP → RSS stable à ~4,3 Mo (budget 50 Mo), WAL et ack progressent de façon cohérente, arrêt brutal (`kill`) laisse un petit reliquat non acquitté par conception (sera rejoué au prochain démarrage — comportement voulu du WAL, pas un bug)
- `go`/Python/Rust kernel : aucune régression (déjà vérifié en M0)

### M1.2 — Réception serveur & bus (Rust)
- [x] Implémenter `stream/receiver.rs` : serveur gRPC réception batches agents
- [x] Implémenter validation schéma + horodatage + enrichissement métadonnées (agent_id, dataset)
- [ ] Implémenter `sink/redpanda.rs` : production vers topics par dataset (rdkafka) — non fait, bloqué sur le toolchain `librdkafka` (pas de `cmake` sur cette machine), cf. note
- [ ] Implémenter `sink/clickhouse.rs` : consommation topics → batchs RowBinary → async inserts — sink ClickHouse fait et testé réellement, mais insertion directe (pas de topic Redpanda en amont) et format JSONEachRow (pas RowBinary), cf. note
- [x] Implémenter DLQ : événements invalides vers topic dédié + compteurs — fichier local JSON lines au lieu d'un topic dédié, même logique de compteur
- [x] Idempotence de base : clé de déduplication par batch
- [ ] Benchmark : ≥ 100k evt/s soutenus en local (profil dev), script `bench-ingest.sh` — non fait cette session, cf. note

**État actuel (2026-08-03)**

Fichiers matérialisés — crate `rust/bootstrap/` (`herminas-dataplane-bootstrap`) étendu avec un vrai `lib.rs` :
- `src/receiver.rs` : `AgentService` implémente le service gRPC `Agent` (`agent.proto`, M0.3) — `ShipBatch` décode le batch (zstd + longueur-préfixée, même framing que `rust/agent`), déduplique, valide chaque enregistrement JSON, enrichit (`agent_id`, `dataset`, `received_at_unix_ms`), écrit les valides via `Sink`, les invalides via `Dlq` ; `GetAgentConfig` renvoie `UNIMPLEMENTED` (dépend de M8.3) ; `Heartbeat` répond réellement
- `src/sink/clickhouse.rs` : `ClickHouseSink` insère via l'interface HTTP de ClickHouse (JSONEachRow)
- `src/dlq.rs` : `Dlq` — fichier JSON lines local, compteur atomique
- `src/dedup.rs` : `Deduplicator` — clé de hash (agent_id, dataset, payload compressé), ensemble borné avec éviction totale au dépassement (pas de vraie LRU)
- `src/bootstrap.rs` : sous-commande `serve` démarre le vrai serveur gRPC receiver (en plus du mode kernel-check existant de M0.1)

Côté agent (`rust/agent/`), le trait `Shipper` est devenu asynchrone (`async-trait`) — nécessaire pour un vrai client gRPC. Nouveau `ship::GrpcShipper` implémente `Shipper` en appelant `AgentClient::ship_batch` pour de vrai. Nouveaux champs de config : `receiver_addr` (bascule `GrpcShipper`/`FileShipper`), `agent_id`, `dataset`. `ship::file`/`ship::grpc` partagent le même code d'encodage/décodage de batch (`encode_batch`/`decode_batch`), garantissant que le récepteur décode exactement ce que l'agent envoie, que ce soit vers un fichier ou vers gRPC.

**Redpanda reporté (honnête, pas contourné)** : `rdkafka` (choix du cahier des charges) nécessite `librdkafka`, une bibliothèque C, compilée via `cmake` — absent de cette machine (pas de Homebrew, cf. M0.1/M0.5). Plutôt que d'écrire du code Rust jamais compilé ni vérifié dans ce sandbox (contraire à la discipline du projet), le sink ClickHouse insère aujourd'hui **directement** depuis le récepteur, sans passer par un bus Redpanda intermédiaire. `sink::Sink` est le point d'extension : un sink Redpanda producteur + un consommateur séparé vers ClickHouse s'insèrent plus tard sans changer `receiver::AgentService`. Cette limitation sera revue sur cible Linux (CI/bundle, comme pour le binaire Redpanda lui-même, M0.5).

**RowBinary reporté** : l'insertion utilise JSONEachRow (simple, sans dépendance supplémentaire) plutôt que RowBinary (nécessite un encodeur conscient du schéma, qui dépend de schemamgr/DDL généré, M1.3). À revoir pour la performance avant le benchmark d'ingestion de M9.

**Benchmark 100k evt/s non fait** : nécessite son propre travail de perf (générateur de charge, mesure soutenue sur 1s+, éventuellement RowBinary + vrai Redpanda pour représenter fidèlement l'architecture cible) — pas fait cette session faute de temps, et probablement irréaliste avec l'insertion JSONEachRow directe actuelle de toute façon.

**Deux vrais bugs trouvés par les tests, pas avant** :
1. `Box::new(Arc<RecordingRealSink>)` ne satisfaisait pas le trait `Sink` dans les tests du récepteur — erreur de compilation immédiate, corrigée en clonant la struct (déjà `Clone`, état partagé via son `Arc<Mutex<_>>` interne) plutôt que de la re-envelopper.
2. **ClickHouse renvoie `411 Length Required` pour toute requête POST sans `Content-Length`, et `reqwest` n'ajoute pas cet en-tête pour un corps vide** (`.body("")` seul ne suffit pas) — découvert en lançant le test d'intégration contre un vrai ClickHouse, pas en mock. Corrigé en fixant explicitement l'en-tête `Content-Length: 0` sur les appels DDL/SELECT du test (l'insertion de production n'est pas concernée : son corps n'est jamais vide).

Validations rejouées :
- `cargo build --workspace`, `cargo clippy --workspace --all-targets` → PASS, zéro warning
- `cargo test --workspace` → PASS (31 tests agent + 11 tests bootstrap + 3 kernel), y compris `sink::clickhouse::tests::insert_and_select_round_trip_against_real_clickhouse` contre un vrai ClickHouse (skip proprement si absent)
- **Smoke test réel bout-en-bout, transport gRPC réel** : `herminas-dataplane serve` (récepteur réel) + `herminas-agent` configuré avec `receiver_addr` réel → tail de fichier + HTTP + ligne JSON malformée (rejetée côté agent par le parseur, jamais expédiée) → `SELECT * FROM e2e_logs` sur le vrai ClickHouse confirme les 4 événements valides, correctement enrichis (`agent_id`, `dataset`, `received_at_unix_ms`) ; WAL entièrement acquitté après drain (`ack offset == wal.log size`) ; c'est le jalon « Fin M1 » du cahier des charges (§10.3), atteint avec le vrai transport gRPC et non plus le `FileShipper` de secours
- `go build/vet/test`, `pytest` → aucune régression
- `scripts/validation/test-secrets-no-leak.sh` → OK

### M1.3 — Schémas & datasets (Go)
- [x] Implémenter `engine/schemamgr/` : modèle dataset (nom, colonnes, types, TTL) en SQLite
- [x] Générer DDL ClickHouse MergeTree (ORDER BY, PARTITION BY date, TTL)
- [x] Implémenter création/évolution de dataset via API (`POST /api/v1/datasets`) — handlers HTTP prêts, pas encore montés dans un serveur réel (dépend de M1.5), cf. note
- [x] Implémenter registry de schémas versionné (compatibilité ascendante)
- [x] Auto-création de dataset à la première ingestion (option, schéma inféré)

**État actuel (2026-08-03)**

Fichiers matérialisés — nouveau package `engine/schemamgr/` :
- `dataset.go` : `Dataset`/`Column`, `Validate()`, `CheckBackwardCompatible()` — n'autorise que l'ajout de colonnes (pas de suppression, pas de changement de type, pas de passage non-nullable→... plus strict)
- `infer.go` : `InferColumns()` déduit les colonnes depuis un enregistrement JSON échantillon (bool→Bool, nombre entier→Int64, nombre à virgule→Float64, chaîne/null/objet/tableau→String), sorties triées par nom pour un DDL déterministe
- `ddl.go` : `GenerateDDL()` — `CREATE TABLE ... ENGINE = MergeTree()` avec `PARTITION BY toYYYYMM(...)`, `ORDER BY (...)` (ou `tuple()` si aucune clé), `TTL ... + INTERVAL n DAY`, identifiants entre backticks
- `store.go` : persistance SQLite (`modernc.org/sqlite`, pilote pur Go — pas de CGO, compatible avec les binaires statiques `CGO_ENABLED=0`) ; table `datasets` (dernière version) + `dataset_versions` (historique complet) ; `Create`, `Get`, `List`, `AddColumns` (bascule de version + contrôle de compatibilité), `EnsureDataset` (auto-création avec `OrderBy=[received_at_unix_ms]` si ce champ d'enrichissement du récepteur M1.2 est présent)
- `http.go` : handlers `net/http.ServeMux` (patterns Go 1.22+, aucune dépendance routeur) — `POST/GET /api/v1/datasets`, `GET /api/v1/datasets/{name}`, `GET /api/v1/datasets/{name}/versions`, `POST /api/v1/datasets/{name}/columns`
- `kernel/errors/errors.go` étendu : `Is`, `IsNotFound`, `IsAlreadyExists`, `IsInvalidArgument` — nécessaires pour mapper proprement les erreurs du store vers les codes HTTP (404/409/400)

**Handlers HTTP pas encore montés dans un vrai serveur** : `Handler.Routes()` retourne un `http.Handler` complet et testé (via `httptest`), mais aucun binaire ne l'expose encore — le serveur HTTP/2 + TLS + JWT est explicitement le périmètre de M1.5. Monter ces routes dans un serveur qui n'existe pas encore aurait été prématuré ; `Routes()` est le point de montage prêt à l'emploi.

**Auto-création pas encore branchée au récepteur Rust** : `EnsureDataset` existe et fonctionne, mais `rust/bootstrap`'s `AgentService` (M1.2) insère toujours directement par nom de table codé en dur — le brancher pour de vrai demanderait soit une nouvelle RPC (Go schemamgr appelé depuis Rust), soit que le récepteur Go lui-même absorbe l'ingestion HTTP. Pas fait cette session ; le composant Go est prêt et testé indépendamment.

Validations rejouées :
- `go build ./...`, `go vet ./...`, `go test ./...` → PASS (30 nouveaux tests `engine/schemamgr` : validation, compatibilité ascendante, inférence, DDL, store SQLite, handlers HTTP)
- **DDL généré vérifié contre un vrai ClickHouse** (`TestGeneratedDDLIsValidClickHouseSQL`, skip proprement si absent) : `CREATE TABLE` avec le DDL généré à partir d'un schéma inféré → `INSERT` JSONEachRow → `SELECT` confirme la valeur insérée — prouve que le DDL généré n'est pas juste une chaîne plausible mais du SQL ClickHouse valide et exécutable
- **Un processus orphelin de M0.5 découvert et nettoyé pendant cette vérification** : le superviseur Go (`go run bootstrap.go run`) lancé lors d'une session précédente tournait toujours en arrière-plan (PID reparenté à `init` après la sortie du wrapper `go run`), et sa propre logique de restart-backoff (`engine/supervisor`, M0.5) relançait ClickHouse à chaque tentative d'arrêt — comportement correct du superviseur, juste un oubli de nettoyage de ma part. Arrêté proprement en ciblant le vrai PID du binaire compilé plutôt que celui du wrapper `go run`.
- `scripts/validation/test-secrets-no-leak.sh` → OK

### M1.4 — Query broker (Go)
- [x] Implémenter `engine/querybroker/` : exécution SQL sur ClickHouse (HTTP), timeout, annulation
- [x] Implémenter streaming des résultats (chunks) vers l'API
- [x] Implémenter quotas basiques : requêtes/min par utilisateur, max lignes scannées
- [x] Implémenter cache résultats court TTL pour requêtes identiques
- [x] Journaliser chaque requête dans l'audit (append-only) — version M1.4 (fichier JSON lines) ; filtres/export/redaction PII restent M7.2

**État actuel (2026-08-03)**

Fichiers matérialisés — nouveau package `engine/querybroker/` :
- `broker.go` : `Execute` (résultat bufferisé, avec cache+audit) et `Stream` (livraison ligne par ligne au fur et à mesure via `bufio.Scanner` sur le corps HTTP en direct, pas de bufferisation complète — annulation via `context.Context` propagée nativement par `http.NewRequestWithContext`) ; force `FORMAT JSONEachRow` sur chaque requête pour un résultat ligne-par-ligne
- `quota.go` : `RequestsPerMinute` (fenêtre glissante par utilisateur) + `MaxRowsToRead` — passé directement à ClickHouse comme réglage `max_rows_to_read`, donc c'est le moteur lui-même qui interrompt une requête trop gourmande, pas une vérification a posteriori après avoir tout scanné
- `cache.go` : cache clé=SQL exacte, expiration paresseuse à la lecture, TTL=0 désactive le cache
- `audit.go` : `AuditLog` — une ligne JSON par requête (utilisateur, SQL, durée, nb lignes, succès/erreur, cache hit ou non), fichier ouvert en append-only

**Un choix de conception qui a évité un bug avant même de l'écrire** : `request()` fixe explicitement `Content-Length: 0` sur toute requête sans corps — leçon directement reprise du bug ClickHouse `411 Length Required` trouvé dans `rust/bootstrap` (M1.2) et revérifié dans `engine/schemamgr` (M1.3). Cette fois, corrigé dès l'écriture plutôt que découvert en testant.

Validations rejouées :
- `go build ./...`, `go vet ./...` → PASS
- `go test ./engine/querybroker/...` → PASS, 17 tests dont **7 contre un vrai ClickHouse** (skip proprement si absent) :
  - `SELECT ... FROM system.numbers` retourne de vraies lignes décodables
  - une requête identique en 2ᵉ appel est servie par le cache (`Cached: true`)
  - `RequestsPerMinute: 1` → 2ᵉ requête rejetée avec `resource_exhausted`
  - `MaxRowsToRead: 1000` sur `SELECT sum(number) FROM numbers(1000000)` → ClickHouse interrompt lui-même la requête, erreur `resource_exhausted` propagée
  - `Stream` livre 10 lignes de façon incrémentale sur `LIMIT 10`
  - `Stream` sur `SELECT sleep(3)` avec un contexte à 200ms s'interrompt réellement en ~200ms, pas après les 3s
  - `Stream` s'arrête proprement après exactement 3 appels quand le callback renvoie une erreur
- **Un processus ClickHouse orphelin retrouvé et nettoyé à nouveau pendant cette session** (même cause qu'en M1.3 : `go run` laisse le binaire compilé tourner même après avoir tué son propre PID wrapper) — cette fois identifié directement via le chemin `exe/bootstrap` dans `ps aux`
- `scripts/validation/test-secrets-no-leak.sh` → OK

**Pas encore fait** : le broker n'est monté dans aucun serveur HTTP réel (comme `schemamgr`, dépend de M1.5) ; pas de limite de taille de résultat côté `Execute` (un résultat énorme sans `LIMIT` explicite serait entièrement bufferisé en mémoire — `Stream` est la bonne réponse à ce cas, mais rien n'empêche aujourd'hui un appelant d'utiliser `Execute` par erreur sur une grosse requête).

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
