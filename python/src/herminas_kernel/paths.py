from __future__ import annotations

import os
from dataclasses import dataclass
from pathlib import Path


@dataclass(frozen=True)
class Layout:
    """On-disk layout resolved relative to a single root, so the bundle
    stays relocatable (leçon aNtaerus). Mirrors kernel/paths/paths.go and
    Rust's herminas_kernel::paths::Layout."""

    root: Path
    config_dir: Path
    runtime_dir: Path
    models_dir: Path
    data_dir: Path
    logs_dir: Path


def resolve(root: str | Path) -> Layout:
    root = Path(root)
    return Layout(
        root=root,
        config_dir=root / "config",
        runtime_dir=root / "runtime",
        models_dir=root / "models",
        data_dir=root / "data",
        logs_dir=root / "logs",
    )


def default_layout() -> Layout:
    """Resolves from HERMINAS_HOME, falling back to the current directory
    (dev mode; the bundle launcher always sets HERMINAS_HOME, see M8.1)."""
    root = Path(os.environ.get("HERMINAS_HOME", Path.cwd()))
    return resolve(root)
