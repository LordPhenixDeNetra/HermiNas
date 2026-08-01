package supervisor

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"herminas/kernel/contracts"
)

// fakeProcess is the in-memory mock used to unit test ordering, health
// gating, restart-on-crash and reverse-order stop without spawning any real
// subprocess.
type fakeProcess struct {
	name         contracts.ServiceName
	startOrder   *[]contracts.ServiceName
	stopOrder    *[]contracts.ServiceName
	mu           *sync.Mutex
	neverHealthy bool

	started int
	healthy bool
	done    chan struct{}
}

func newFakeProcess(name contracts.ServiceName, startOrder *[]contracts.ServiceName, mu *sync.Mutex) *fakeProcess {
	return &fakeProcess{name: name, startOrder: startOrder, mu: mu}
}

func (f *fakeProcess) Name() contracts.ServiceName { return f.name }

func (f *fakeProcess) Start(_ context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.started++
	f.healthy = !f.neverHealthy
	f.done = make(chan struct{})
	if f.startOrder != nil {
		*f.startOrder = append(*f.startOrder, f.name)
	}
	return nil
}

func (f *fakeProcess) Stop(_ context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.healthy = false
	if f.stopOrder != nil {
		*f.stopOrder = append(*f.stopOrder, f.name)
	}
	if f.done != nil {
		select {
		case <-f.done:
		default:
			close(f.done)
		}
	}
	return nil
}

func (f *fakeProcess) Wait() error {
	f.mu.Lock()
	done := f.done
	f.mu.Unlock()
	if done == nil {
		return errors.New("not started")
	}
	<-done
	return nil
}

func (f *fakeProcess) Health(_ context.Context) (contracts.HealthStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	state := contracts.HealthUnhealthy
	if f.healthy {
		state = contracts.HealthHealthy
	}
	return contracts.HealthStatus{Service: f.name, State: state}, nil
}

func TestStartRunsInOrderAndWaitsForHealth(t *testing.T) {
	var order []contracts.ServiceName
	var mu sync.Mutex

	a := newFakeProcess(contracts.ServiceRedpanda, &order, &mu)
	b := newFakeProcess(contracts.ServiceClickHouse, &order, &mu)

	s := New()
	s.Register(a, time.Second, Backoff{})
	s.Register(b, time.Second, Backoff{})

	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer s.Stop(context.Background())

	mu.Lock()
	got := append([]contracts.ServiceName(nil), order...)
	mu.Unlock()

	if len(got) != 2 || got[0] != contracts.ServiceRedpanda || got[1] != contracts.ServiceClickHouse {
		t.Fatalf("unexpected start order: %v", got)
	}
}

func TestStartFailsIfNeverHealthy(t *testing.T) {
	var order []contracts.ServiceName
	var mu sync.Mutex

	p := newFakeProcess(contracts.ServiceClickHouse, &order, &mu)
	p.neverHealthy = true

	s := New()
	s.Register(p, 200*time.Millisecond, Backoff{})

	if err := s.Start(context.Background()); err == nil {
		t.Fatal("expected Start to fail when process never becomes healthy")
	}
}

func TestStopRunsInReverseOrder(t *testing.T) {
	var startOrder, stopOrder []contracts.ServiceName
	var mu sync.Mutex

	a := newFakeProcess(contracts.ServiceRedpanda, &startOrder, &mu)
	a.stopOrder = &stopOrder
	b := newFakeProcess(contracts.ServiceClickHouse, &startOrder, &mu)
	b.stopOrder = &stopOrder

	s := New()
	s.Register(a, time.Second, Backoff{})
	s.Register(b, time.Second, Backoff{})

	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if err := s.Stop(context.Background()); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	mu.Lock()
	got := append([]contracts.ServiceName(nil), stopOrder...)
	mu.Unlock()

	if len(got) != 2 || got[0] != contracts.ServiceClickHouse || got[1] != contracts.ServiceRedpanda {
		t.Fatalf("unexpected stop order: %v", got)
	}
}

func TestWatchRestartsOnUnexpectedExit(t *testing.T) {
	var order []contracts.ServiceName
	var mu sync.Mutex

	p := newFakeProcess(contracts.ServiceClickHouse, &order, &mu)

	s := New()
	s.Register(p, time.Second, Backoff{Initial: 10 * time.Millisecond, Max: 20 * time.Millisecond, Multiplier: 2})

	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer s.Stop(context.Background())

	// Simulate a crash: close done directly, bypassing Stop (so `stopping`
	// stays false and the watch loop must restart the process).
	mu.Lock()
	close(p.done)
	mu.Unlock()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		started := p.started
		mu.Unlock()
		if started >= 2 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("process was not restarted after unexpected exit")
}

func TestStatusReportsHealthWithoutStarting(t *testing.T) {
	var order []contracts.ServiceName
	var mu sync.Mutex

	p := newFakeProcess(contracts.ServiceClickHouse, &order, &mu)

	s := New()
	s.Register(p, time.Second, Backoff{})

	statuses := s.Status(context.Background())
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].State != contracts.HealthUnhealthy {
		t.Fatalf("expected unhealthy before Start, got %s", statuses[0].State)
	}
}
