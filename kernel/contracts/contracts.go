// Package contracts holds the domain types and interfaces shared across
// HermiNas' layers (L0-L3). Rust and Python keep language-native mirrors of
// the same concepts (see rust/kernel/src/contracts.rs and
// python/src/herminas_kernel/contracts.py) until the protobuf contracts
// land in M0.3 (kernel/proto/*.proto) and become the single source of truth
// for cross-process serialization.
package contracts

import (
	"context"
	"time"
)

// ServiceName identifies a service the L2 supervisor manages or probes.
type ServiceName string

const (
	ServiceClickHouse   ServiceName = "clickhouse"
	ServiceRedpanda     ServiceName = "redpanda"
	ServiceDataPlane    ServiceName = "dataplane"
	ServiceIntelligence ServiceName = "intelligence"
	ServiceAPI          ServiceName = "api"
)

// HealthState is the coarse health of a managed service.
type HealthState string

const (
	HealthUnknown   HealthState = "unknown"
	HealthStarting  HealthState = "starting"
	HealthHealthy   HealthState = "healthy"
	HealthDegraded  HealthState = "degraded"
	HealthUnhealthy HealthState = "unhealthy"
)

// HealthStatus is the cross-layer health report: same shape is returned by
// GetHealth (gRPC, M0.3), exposed at /api/v1/health (REST) and printed by
// `herminas status` (CLI, M8).
type HealthStatus struct {
	Service   ServiceName
	State     HealthState
	Message   string
	CheckedAt time.Time
}

// HealthChecker is implemented by anything the supervisor (L2) can probe:
// the embedded engines (ClickHouse, Redpanda) and the Rust/Python runtimes.
type HealthChecker interface {
	Health(ctx context.Context) (HealthStatus, error)
}

// Event is the minimal envelope shared by ingestion, stream processing and
// storage. Kept manually in sync with the Rust and Python equivalents until
// the protobuf schema (kernel/proto/agent.proto, M0.3) replaces it as the
// wire contract.
type Event struct {
	ID        string
	Dataset   string
	Timestamp time.Time
	Payload   []byte
	Metadata  map[string]string
}
