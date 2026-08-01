from __future__ import annotations

from enum import Enum


class Code(str, Enum):
    """Mirrors (conceptually) kernel/errors/errors.go's Code and Rust's
    herminas_kernel::errors::Code — same vocabulary, three implementations."""

    INVALID_ARGUMENT = "invalid_argument"
    NOT_FOUND = "not_found"
    UNAUTHORIZED = "unauthorized"
    INTERNAL = "internal"
    UNAVAILABLE = "unavailable"
    ALREADY_EXISTS = "already_exists"


class HerminasError(Exception):
    def __init__(self, code: Code, message: str, cause: Exception | None = None) -> None:
        super().__init__(f"[{code.value}] {message}")
        self.code = code
        self.message = message
        if cause is not None:
            self.__cause__ = cause
