import datetime

from . import common_pb2 as _common_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class PipelineId(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class PipelineSpec(_message.Message):
    __slots__ = ("id", "name", "yaml_config")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    YAML_CONFIG_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    yaml_config: str
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., yaml_config: _Optional[str] = ...) -> None: ...

class DeployResult(_message.Message):
    __slots__ = ("success", "message")
    SUCCESS_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    success: bool
    message: str
    def __init__(self, success: _Optional[bool] = ..., message: _Optional[str] = ...) -> None: ...

class StopResult(_message.Message):
    __slots__ = ("success", "message")
    SUCCESS_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    success: bool
    message: str
    def __init__(self, success: _Optional[bool] = ..., message: _Optional[str] = ...) -> None: ...

class PipelineStats(_message.Message):
    __slots__ = ("pipeline_id", "events_in", "events_out", "events_error", "p99_latency_ms")
    PIPELINE_ID_FIELD_NUMBER: _ClassVar[int]
    EVENTS_IN_FIELD_NUMBER: _ClassVar[int]
    EVENTS_OUT_FIELD_NUMBER: _ClassVar[int]
    EVENTS_ERROR_FIELD_NUMBER: _ClassVar[int]
    P99_LATENCY_MS_FIELD_NUMBER: _ClassVar[int]
    pipeline_id: str
    events_in: int
    events_out: int
    events_error: int
    p99_latency_ms: float
    def __init__(self, pipeline_id: _Optional[str] = ..., events_in: _Optional[int] = ..., events_out: _Optional[int] = ..., events_error: _Optional[int] = ..., p99_latency_ms: _Optional[float] = ...) -> None: ...

class ModelSpec(_message.Message):
    __slots__ = ("model_id", "onnx_artifact", "dataset")
    MODEL_ID_FIELD_NUMBER: _ClassVar[int]
    ONNX_ARTIFACT_FIELD_NUMBER: _ClassVar[int]
    DATASET_FIELD_NUMBER: _ClassVar[int]
    model_id: str
    onnx_artifact: bytes
    dataset: str
    def __init__(self, model_id: _Optional[str] = ..., onnx_artifact: _Optional[bytes] = ..., dataset: _Optional[str] = ...) -> None: ...

class RuleSpec(_message.Message):
    __slots__ = ("rule_id", "wasm_artifact", "dataset")
    RULE_ID_FIELD_NUMBER: _ClassVar[int]
    WASM_ARTIFACT_FIELD_NUMBER: _ClassVar[int]
    DATASET_FIELD_NUMBER: _ClassVar[int]
    rule_id: str
    wasm_artifact: bytes
    dataset: str
    def __init__(self, rule_id: _Optional[str] = ..., wasm_artifact: _Optional[bytes] = ..., dataset: _Optional[str] = ...) -> None: ...

class EventFilter(_message.Message):
    __slots__ = ("datasets", "event_types")
    DATASETS_FIELD_NUMBER: _ClassVar[int]
    EVENT_TYPES_FIELD_NUMBER: _ClassVar[int]
    datasets: _containers.RepeatedScalarFieldContainer[str]
    event_types: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, datasets: _Optional[_Iterable[str]] = ..., event_types: _Optional[_Iterable[str]] = ...) -> None: ...

class Event(_message.Message):
    __slots__ = ("id", "dataset", "type", "timestamp", "payload", "metadata")
    class MetadataEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    ID_FIELD_NUMBER: _ClassVar[int]
    DATASET_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    TIMESTAMP_FIELD_NUMBER: _ClassVar[int]
    PAYLOAD_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    id: str
    dataset: str
    type: str
    timestamp: _timestamp_pb2.Timestamp
    payload: bytes
    metadata: _containers.ScalarMap[str, str]
    def __init__(self, id: _Optional[str] = ..., dataset: _Optional[str] = ..., type: _Optional[str] = ..., timestamp: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., payload: _Optional[bytes] = ..., metadata: _Optional[_Mapping[str, str]] = ...) -> None: ...
