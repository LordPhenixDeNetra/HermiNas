// Package supervisor starts, health-gates, restarts and stops HermiNas'
// managed processes in a fixed order: leçon aNtaerus applied to ClickHouse
// and Redpanda today (M0.5), and to the Rust data plane / Python
// intelligence / Go API once they expose long-running servers (M1-M4).
package supervisor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"herminas/kernel/contracts"
)

// Process is anything the supervisor can start, stop, wait on and
// health-check.
type Process interface {
	Name() contracts.ServiceName
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	// Wait blocks until the process exits (crash or intentional Stop) and
	// returns the exit error, if any.
	Wait() error
	Health(ctx context.Context) (contracts.HealthStatus, error)
}

// Backoff controls the delay between restart attempts after an unexpected
// exit.
type Backoff struct {
	Initial    time.Duration
	Max        time.Duration
	Multiplier float64
}

func DefaultBackoff() Backoff {
	return Backoff{Initial: 500 * time.Millisecond, Max: 30 * time.Second, Multiplier: 2}
}

func (b Backoff) next(current time.Duration) time.Duration {
	if current <= 0 {
		return b.Initial
	}
	next := time.Duration(float64(current) * b.Multiplier)
	if next > b.Max {
		return b.Max
	}
	return next
}

type registration struct {
	process      Process
	readyTimeout time.Duration
	backoff      Backoff

	mu        sync.Mutex
	stopping  bool
	watchDone chan struct{}
}

// Supervisor starts, health-gates, restarts and stops a fixed, ordered list
// of processes (cahier des charges §5.3.1 supervisor/): start order is
// declared order, stop order is the reverse.
type Supervisor struct {
	mu   sync.Mutex
	regs []*registration
}

func New() *Supervisor {
	return &Supervisor{}
}

// Register adds a process to the end of the start order. A non-positive
// readyTimeout defaults to 30s; a zero-value Backoff defaults to
// DefaultBackoff().
func (s *Supervisor) Register(p Process, readyTimeout time.Duration, backoff Backoff) {
	if readyTimeout <= 0 {
		readyTimeout = 30 * time.Second
	}
	if backoff == (Backoff{}) {
		backoff = DefaultBackoff()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.regs = append(s.regs, &registration{process: p, readyTimeout: readyTimeout, backoff: backoff})
}

// Start launches every registered process in order, waiting for each to
// report healthy before starting the next. If a process never becomes
// healthy within its readyTimeout, Start aborts and returns an error;
// processes already started are left running — call Stop to tear them down.
func (s *Supervisor) Start(ctx context.Context) error {
	s.mu.Lock()
	regs := append([]*registration(nil), s.regs...)
	s.mu.Unlock()

	for _, reg := range regs {
		if err := reg.process.Start(ctx); err != nil {
			return fmt.Errorf("start %s: %w", reg.process.Name(), err)
		}

		reg.watchDone = make(chan struct{})
		go s.watch(reg)

		if err := s.awaitHealthy(ctx, reg); err != nil {
			return err
		}
	}
	return nil
}

func (s *Supervisor) awaitHealthy(ctx context.Context, reg *registration) error {
	deadline := time.Now().Add(reg.readyTimeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		status, err := reg.process.Health(ctx)
		if err == nil && status.State == contracts.HealthHealthy {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%s did not become healthy within %s", reg.process.Name(), reg.readyTimeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// watch restarts reg.process with backoff whenever it exits unexpectedly
// (i.e. not because Stop marked it as stopping).
func (s *Supervisor) watch(reg *registration) {
	defer close(reg.watchDone)
	delay := time.Duration(0)

	for {
		waitErr := reg.process.Wait()

		reg.mu.Lock()
		stopping := reg.stopping
		reg.mu.Unlock()
		if stopping {
			return
		}

		for {
			delay = reg.backoff.next(delay)
			fmt.Printf("supervisor: %s exited (%v), restarting in %s\n", reg.process.Name(), waitErr, delay)
			time.Sleep(delay)

			if err := reg.process.Start(context.Background()); err != nil {
				fmt.Printf("supervisor: %s restart failed: %v\n", reg.process.Name(), err)
				continue
			}
			break
		}
	}
}

// Stop stops every registered process in reverse start order and waits for
// each watch goroutine to exit before returning.
func (s *Supervisor) Stop(ctx context.Context) error {
	s.mu.Lock()
	regs := append([]*registration(nil), s.regs...)
	s.mu.Unlock()

	var firstErr error
	for i := len(regs) - 1; i >= 0; i-- {
		reg := regs[i]

		reg.mu.Lock()
		reg.stopping = true
		reg.mu.Unlock()

		if err := reg.process.Stop(ctx); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("stop %s: %w", reg.process.Name(), err)
		}

		if reg.watchDone != nil {
			<-reg.watchDone
		}
	}
	return firstErr
}

// Status health-checks every registered process without starting anything,
// so it works from a fresh CLI invocation against services a *different*
// process started (`herminas status` after `herminas run`).
func (s *Supervisor) Status(ctx context.Context) []contracts.HealthStatus {
	s.mu.Lock()
	regs := append([]*registration(nil), s.regs...)
	s.mu.Unlock()

	statuses := make([]contracts.HealthStatus, 0, len(regs))
	for _, reg := range regs {
		status, err := reg.process.Health(ctx)
		if err != nil {
			status = contracts.HealthStatus{
				Service:   reg.process.Name(),
				State:     contracts.HealthUnknown,
				Message:   err.Error(),
				CheckedAt: time.Now(),
			}
		}
		statuses = append(statuses, status)
	}
	return statuses
}
