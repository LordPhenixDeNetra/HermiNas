# Secrets — HermiNas

## Typage

| Langage | Type | Fichier |
|---|---|---|
| Go | `settings.SecretString` | `kernel/settings/secret.go` |
| Rust | `secrecy::SecretString` | `rust/kernel/src/settings.rs` |
| Python | `pydantic.SecretStr` | `python/src/herminas_kernel/settings.py` |

Aucun de ces types n'expose la valeur réelle via `String()`/`Debug`/`repr`/sérialisation JSON par défaut. Seul un accès explicite donne la valeur en clair :
- Go : `SecretString.Reveal()`
- Rust : `secrecy::ExposeSecret::expose_secret()`
- Python : `SecretStr.get_secret_value()`

Ne jamais logger le résultat de ces accesseurs explicites.

## Immuabilité runtime

`Settings` est chargé une seule fois au boot (`settings.Load` / `Settings::load` / `herminas_kernel.settings.load`) à partir de `config/herminas.example.yaml` (dev) ou `config/herminas.yaml` (bundle, M8). Aucune mutation n'est possible après le chargement :
- Go : champs non exportés, accès uniquement par getters
- Rust : champs privés, accès uniquement par getters (`&self`)
- Python : `model_config = ConfigDict(frozen=True)` (pydantic)

## Test anti-fuite

- `scripts/validation/test-secrets-no-leak.sh` : grep du dépôt entier pour des motifs de secrets en clair (clés `sk-...`, clés AWS, clés privées PEM).
- Tests unitaires par langage : `kernel/settings/secret_test.go`, `rust/kernel/src/settings.rs` (module `tests`), `python/tests/test_secrets_no_leak.py`. Ils vérifient que `SecretString`/`secrecy::SecretString`/`SecretStr` ne fuient jamais via formatage, `Debug`, JSON ou texte.

À rejouer à chaque lot (règle de pilotage `tasks-herminas.md`, tâche transversale « Sécurité continue »).

## Rotation

Non implémentée en M0. Prévue en M7 (RBAC & gouvernance) : rotation des tokens API et des clés LLM stockées chiffrées (`secrets.enc`, AES-256-GCM, M0.4/M8).
