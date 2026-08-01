from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime
from enum import Enum
from typing import Protocol, runtime_checkable


class ServiceName(str, Enum):
    CLICKHOUSE = "clickhouse"
    REDPANDA = "redpanda"
    DATA_PLANE = "dataplane"
    INTELLIGENCE = "intelligence"
    API = "api"


class HealthState(str, Enum):
    UNKNOWN = "unknown"
    STARTING = "starting"
    HEALTHY = "healthy"
    DEGRADED = "degraded"
    UNHEALTHY = "unhealthy"


@dataclass(frozen=True)
class HealthStatus:
    service: ServiceName
    state: HealthState
    message: str
    checked_at: datetime


@runtime_checkable
class HealthChecker(Protocol):
    def health(self) -> HealthStatus: ...


@dataclass(frozen=True)
class Event:
    """Minimal event envelope shared by ingestion, stream processing and
    storage. Mirrors kernel/contracts/contracts.go's Event and Rust's
    herminas_kernel::contracts::Event."""

    id: str
    dataset: str
    timestamp: datetime
    payload: bytes
    metadata: dict[str, str] = field(default_factory=dict)
