// Package health will host the Prometheus-facing healthcheck aggregator
// (cahier des charges §5.3.1: "health/ — Healthchecks, métriques
// Prometheus"). Today it only holds the M0.3 gRPC latency benchmarks:
// real, running Rust and Python GetHealth servers, called for real from
// Go, timed for real. These skip (not fail) when the target server isn't
// running, so `go test ./...` stays fast and dependency-free by default —
// start the servers manually to actually exercise the benchmark:
//
//	rust: cargo run -p herminas-protocol --bin dataplane_server
//	python: python -m herminas_intelligence.grpc_server
package health

import (
	"context"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"herminas/kernel/proto/commonpb"
	"herminas/kernel/proto/dataplanepb"
	"herminas/kernel/proto/intelligencepb"
)

func dial(addr string) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	return grpc.DialContext(ctx, addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
}

type latencyStats struct {
	p50, p99, avg, max time.Duration
}

func measure(n int, call func() error) (latencyStats, error) {
	durations := make([]time.Duration, 0, n)
	for i := 0; i < n; i++ {
		start := time.Now()
		if err := call(); err != nil {
			return latencyStats{}, err
		}
		durations = append(durations, time.Since(start))
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })

	var total time.Duration
	for _, d := range durations {
		total += d
	}

	return latencyStats{
		p50: durations[n/2],
		p99: durations[int(float64(n)*0.99)],
		avg: total / time.Duration(n),
		max: durations[n-1],
	}, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// TestGRPCLatencyRustDataPlane benchmarks Go -> Rust GetHealth against the
// < 5ms budget (cahier des charges §4.2/§8.1).
func TestGRPCLatencyRustDataPlane(t *testing.T) {
	addr := envOr("HERMINAS_DATAPLANE_ADDR", "127.0.0.1:50051")

	conn, err := dial(addr)
	if err != nil {
		t.Skipf("Rust DataPlane server not reachable at %s (%v) — run `cargo run -p herminas-protocol --bin dataplane_server` to exercise this benchmark", addr, err)
	}
	defer conn.Close()

	client := dataplanepb.NewDataPlaneClient(conn)

	for i := 0; i < 10; i++ {
		if _, err := client.GetHealth(context.Background(), &commonpb.HealthRequest{}); err != nil {
			t.Fatalf("warm-up call failed: %v", err)
		}
	}

	stats, err := measure(200, func() error {
		_, err := client.GetHealth(context.Background(), &commonpb.HealthRequest{})
		return err
	})
	if err != nil {
		t.Fatalf("GetHealth failed: %v", err)
	}

	fmt.Printf("Go<->Rust GetHealth: p50=%s p99=%s avg=%s max=%s\n", stats.p50, stats.p99, stats.avg, stats.max)

	const budget = 5 * time.Millisecond
	if stats.p99 > budget {
		t.Errorf("p99 latency %s exceeds %s budget (cahier des charges §4.2/§8.1)", stats.p99, budget)
	}
}

// TestGRPCLatencyPythonIntelligence benchmarks Go -> Python GetHealth
// against the < 50ms budget (cahier des charges §4.2/§8.1).
func TestGRPCLatencyPythonIntelligence(t *testing.T) {
	addr := envOr("HERMINAS_INTELLIGENCE_ADDR", "127.0.0.1:50052")

	conn, err := dial(addr)
	if err != nil {
		t.Skipf("Python Intelligence server not reachable at %s (%v) — run `python -m herminas_intelligence.grpc_server` to exercise this benchmark", addr, err)
	}
	defer conn.Close()

	client := intelligencepb.NewIntelligenceClient(conn)

	for i := 0; i < 10; i++ {
		if _, err := client.GetHealth(context.Background(), &commonpb.HealthRequest{}); err != nil {
			t.Fatalf("warm-up call failed: %v", err)
		}
	}

	stats, err := measure(200, func() error {
		_, err := client.GetHealth(context.Background(), &commonpb.HealthRequest{})
		return err
	})
	if err != nil {
		t.Fatalf("GetHealth failed: %v", err)
	}

	fmt.Printf("Go<->Python GetHealth: p50=%s p99=%s avg=%s max=%s\n", stats.p50, stats.p99, stats.avg, stats.max)

	const budget = 50 * time.Millisecond
	if stats.p99 > budget {
		t.Errorf("p99 latency %s exceeds %s budget (cahier des charges §4.2/§8.1)", stats.p99, budget)
	}
}
