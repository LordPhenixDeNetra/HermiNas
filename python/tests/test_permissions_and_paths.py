from pathlib import Path

from herminas_kernel.paths import resolve
from herminas_kernel.permissions import Role, at_least


def test_at_least_respects_hierarchy():
    assert at_least(Role.ADMIN, Role.ENGINEER)
    assert not at_least(Role.VIEWER, Role.ANALYST)


def test_resolve_builds_expected_subpaths():
    layout = resolve("/tmp/herminas")
    assert layout.data_dir == Path("/tmp/herminas/data")
    assert layout.config_dir == Path("/tmp/herminas/config")
