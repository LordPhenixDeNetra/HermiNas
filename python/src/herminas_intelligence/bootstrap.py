"""Python composition root (L1) for the intelligence layer. Wires the L0
kernel (settings) at startup. The nlsql/ml/jobs/llm modules land in M4;
today this only proves the kernel loads cleanly end to end."""

from __future__ import annotations

import os
import sys

from herminas_kernel.settings import load


def main() -> int:
    config_path = os.environ.get("HERMINAS_CONFIG", "config/herminas.example.yaml")

    try:
        settings = load(config_path)
    except FileNotFoundError:
        print(
            f"HermiNas intelligence bootstrap FAILED: config not found at {config_path}",
            file=sys.stderr,
        )
        return 1

    print("HermiNas intelligence bootstrap OK")
    print(f"  environment : {settings.environment}")
    print(f"  grpc_port   : {settings.grpc_port}")
    print(f"  data_dir    : {settings.data_dir}")
    print(f"  llm_provider: {settings.llm_provider}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
