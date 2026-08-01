"""Python half of the M0.4 anti-leak test (test_secrets_no_leak): the raw
secret must never appear in repr/str/JSON dumps of Settings."""

from herminas_kernel.settings import Settings


def _settings_with(fake_secret: str) -> Settings:
    return Settings(
        environment="test",
        http_port=8080,
        grpc_port=9090,
        data_dir="./data",
        llm_provider="ollama",
        llm_api_key=fake_secret,
    )


def test_secret_not_leaked_in_repr_and_str():
    fake = "sk-supersecrettoken1234567890"
    settings = _settings_with(fake)

    outputs = [repr(settings), str(settings), settings.model_dump_json()]

    for out in outputs:
        assert fake not in out


def test_secret_reveal_requires_explicit_call():
    fake = "sk-supersecrettoken1234567890"
    settings = _settings_with(fake)

    assert settings.llm_api_key.get_secret_value() == fake
