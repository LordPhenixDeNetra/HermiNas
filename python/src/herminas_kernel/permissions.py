from __future__ import annotations

from enum import Enum


class Role(str, Enum):
    """HermiNas' four RBAC roles (cahier des charges §7.1). Mirrors
    kernel/permissions/permissions.go and Rust's
    herminas_kernel::permissions::Role. Full RBAC middleware lands in
    M1.5/M7.1."""

    VIEWER = "viewer"
    ANALYST = "analyst"
    ENGINEER = "engineer"
    ADMIN = "admin"


_RANK = {
    Role.VIEWER: 0,
    Role.ANALYST: 1,
    Role.ENGINEER: 2,
    Role.ADMIN: 3,
}


def rank(role: Role) -> int:
    return _RANK[role]


def at_least(role: Role, minimum: Role) -> bool:
    return rank(role) >= rank(minimum)
