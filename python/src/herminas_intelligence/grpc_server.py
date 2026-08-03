"""Minimal real Intelligence gRPC server (M0.3): only GetHealth is
implemented, to prove the gRPC plumbing end to end (proto -> generated
stubs in 3 languages -> real server -> real client -> latency benchmark).
NL->SQL (M4.1), training/export (M4.3) and forecasting (M4.3) implement the
rest of the service; calling them today returns UNIMPLEMENTED via the
generated servicer base class's default behaviour.
"""

from __future__ import annotations

import os
from concurrent import futures

import grpc
from google.protobuf import timestamp_pb2

from herminas_proto import common_pb2, intelligence_pb2_grpc


class IntelligenceService(intelligence_pb2_grpc.IntelligenceServicer):
    def GetHealth(self, request, context):
        checked_at = timestamp_pb2.Timestamp()
        checked_at.GetCurrentTime()
        return common_pb2.HealthStatus(
            service="intelligence",
            state="healthy",
            message="",
            checked_at=checked_at,
        )


def serve(addr: str = "127.0.0.1:50052") -> grpc.Server:
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=4))
    intelligence_pb2_grpc.add_IntelligenceServicer_to_server(IntelligenceService(), server)
    server.add_insecure_port(addr)
    server.start()
    print(f"herminas-intelligence: gRPC server listening on {addr}")
    return server


if __name__ == "__main__":
    grpc_server = serve(os.environ.get("HERMINAS_INTELLIGENCE_ADDR", "127.0.0.1:50052"))
    grpc_server.wait_for_termination()
