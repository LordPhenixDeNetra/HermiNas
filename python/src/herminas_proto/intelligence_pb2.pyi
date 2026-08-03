import datetime

from . import common_pb2 as _common_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class NLQuery(_message.Message):
    __slots__ = ("question", "language", "allowed_datasets")
    QUESTION_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    ALLOWED_DATASETS_FIELD_NUMBER: _ClassVar[int]
    question: str
    language: str
    allowed_datasets: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, question: _Optional[str] = ..., language: _Optional[str] = ..., allowed_datasets: _Optional[_Iterable[str]] = ...) -> None: ...

class SQLCandidate(_message.Message):
    __slots__ = ("sql", "explanation", "valid")
    SQL_FIELD_NUMBER: _ClassVar[int]
    EXPLANATION_FIELD_NUMBER: _ClassVar[int]
    VALID_FIELD_NUMBER: _ClassVar[int]
    sql: str
    explanation: str
    valid: bool
    def __init__(self, sql: _Optional[str] = ..., explanation: _Optional[str] = ..., valid: _Optional[bool] = ...) -> None: ...

class ModelId(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class TrainRequest(_message.Message):
    __slots__ = ("dataset", "model_type")
    DATASET_FIELD_NUMBER: _ClassVar[int]
    MODEL_TYPE_FIELD_NUMBER: _ClassVar[int]
    dataset: str
    model_type: str
    def __init__(self, dataset: _Optional[str] = ..., model_type: _Optional[str] = ...) -> None: ...

class TrainProgress(_message.Message):
    __slots__ = ("progress", "stage", "message")
    PROGRESS_FIELD_NUMBER: _ClassVar[int]
    STAGE_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    progress: float
    stage: str
    message: str
    def __init__(self, progress: _Optional[float] = ..., stage: _Optional[str] = ..., message: _Optional[str] = ...) -> None: ...

class ONNXArtifact(_message.Message):
    __slots__ = ("model_id", "onnx_bytes")
    MODEL_ID_FIELD_NUMBER: _ClassVar[int]
    ONNX_BYTES_FIELD_NUMBER: _ClassVar[int]
    model_id: str
    onnx_bytes: bytes
    def __init__(self, model_id: _Optional[str] = ..., onnx_bytes: _Optional[bytes] = ...) -> None: ...

class ForecastRequest(_message.Message):
    __slots__ = ("dataset", "metric", "horizon_minutes")
    DATASET_FIELD_NUMBER: _ClassVar[int]
    METRIC_FIELD_NUMBER: _ClassVar[int]
    HORIZON_MINUTES_FIELD_NUMBER: _ClassVar[int]
    dataset: str
    metric: str
    horizon_minutes: int
    def __init__(self, dataset: _Optional[str] = ..., metric: _Optional[str] = ..., horizon_minutes: _Optional[int] = ...) -> None: ...

class ForecastResult(_message.Message):
    __slots__ = ("values", "timestamps")
    VALUES_FIELD_NUMBER: _ClassVar[int]
    TIMESTAMPS_FIELD_NUMBER: _ClassVar[int]
    values: _containers.RepeatedScalarFieldContainer[float]
    timestamps: _containers.RepeatedCompositeFieldContainer[_timestamp_pb2.Timestamp]
    def __init__(self, values: _Optional[_Iterable[float]] = ..., timestamps: _Optional[_Iterable[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]]] = ...) -> None: ...
