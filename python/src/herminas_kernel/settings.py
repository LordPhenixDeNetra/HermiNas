from __future__ import annotations

import os
from pathlib import Path

import yaml
from pydantic import BaseModel, ConfigDict, SecretStr


class Settings(BaseModel):
    """HermiNas' immutable, boot-time configuration for the intelligence
    layer. Loaded once via `load`; frozen so nothing can mutate it at
    runtime (leçon aNtaerus: config immuable, 1 seule source)."""

    model_config = ConfigDict(frozen=True)

    environment: str
    http_port: int
    grpc_port: int
    data_dir: str
    llm_provider: str
    llm_api_key: SecretStr


def load(path: str | Path) -> Settings:
    path = Path(path)
    with path.open("r", encoding="utf-8") as fh:
        raw = yaml.safe_load(fh) or {}

    llm = raw.get("llm", {}) or {}
    http = raw.get("http", {}) or {}
    grpc = raw.get("grpc", {}) or {}

    api_key = os.environ.get("HERMINAS_LLM_API_KEY", llm.get("api_key", ""))

    return Settings(
        environment=raw.get("environment", "development"),
        http_port=http.get("port", 8080),
        grpc_port=grpc.get("port", 9090),
        data_dir=raw.get("data_dir", "./data"),
        llm_provider=llm.get("provider", "ollama"),
        llm_api_key=api_key,
    )
