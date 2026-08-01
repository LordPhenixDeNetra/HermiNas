#!/usr/bin/env bash
set -euo pipefail

# M0.4 anti-leak test (test_secrets_no_leak): greps every tracked file for
# secret-shaped strings. Complements the per-language SecretString/SecretStr
# /secrecy unit tests, which check that the *typed wrappers* never leak —
# this catches secrets pasted in plain text anywhere else in the repo.
# Rejouer à chaque lot (règle "Sécurité continue", tasks-herminas.md).

cd "$(git rev-parse --show-toplevel)"

PATTERN='sk-[A-Za-z0-9]{20,}|AKIA[0-9A-Z]{16}|-----BEGIN [A-Z ]*PRIVATE KEY-----'

# Known fixtures: these three files intentionally contain a fake secret
# literal ("sk-supersecrettoken...") to prove SecretString/SecretStr/secrecy
# never leak it. Excluding them by path, not by pattern, keeps this script
# a real grep rather than something the fixtures could quietly defeat.
EXCLUDE='^scripts/validation/test-secrets-no-leak\.sh$|^kernel/settings/secret_test\.go$|^python/tests/test_secrets_no_leak\.py$|^rust/kernel/src/settings\.rs$'

MATCHES=$(git ls-files \
  | grep -vE "$EXCLUDE" \
  | xargs grep -InE "$PATTERN" 2>/dev/null || true)

if [ -n "$MATCHES" ]; then
  echo "Potential secret found in tracked files:" >&2
  echo "$MATCHES" >&2
  exit 1
fi

echo "test_secrets_no_leak: OK — no secret-shaped string in tracked files."
