package supervisor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"herminas/kernel/contracts"
)

// HealthCheckFunc probes a running process and reports its health.
type HealthCheckFunc func(ctx context.Context) (contracts.HealthStatus, error)

// ExecProcess supervises an external command as a subprocess: Start/Stop
// manage its lifecycle, Wait blocks until it exits, and Health delegates to
// a caller-supplied check (typically an HTTP probe against the service's
// own health endpoint) so status queries work even from a separate process
// invocation.
type ExecProcess struct {
	name        contracts.ServiceName
	path        string
	args        []string
	dir         string
	env         []string
	healthCheck HealthCheckFunc
	stopGrace   time.Duration

	mu      sync.Mutex
	cmd     *exec.Cmd
	waitErr error
	done    chan struct{}
}

type ExecOption func(*ExecProcess)

func WithDir(dir string) ExecOption            { return func(p *ExecProcess) { p.dir = dir } }
func WithEnv(env []string) ExecOption          { return func(p *ExecProcess) { p.env = env } }
func WithStopGrace(d time.Duration) ExecOption { return func(p *ExecProcess) { p.stopGrace = d } }
func WithHealthCheck(fn HealthCheckFunc) ExecOption {
	return func(p *ExecProcess) { p.healthCheck = fn }
}

func NewExecProcess(name contracts.ServiceName, path string, args []string, opts ...ExecOption) *ExecProcess {
	p := &ExecProcess{name: name, path: path, args: args, stopGrace: 10 * time.Second}
	for _, o := range opts {
		o(p)
	}
	return p
}

func (p *ExecProcess) Name() contracts.ServiceName { return p.name }

func (p *ExecProcess) Start(_ context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	cmd := exec.Command(p.path, p.args...)
	if p.dir != "" {
		cmd.Dir = p.dir
	}
	if len(p.env) > 0 {
		cmd.Env = append(os.Environ(), p.env...)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", p.name, err)
	}

	p.cmd = cmd
	p.waitErr = nil
	done := make(chan struct{})
	p.done = done

	go func() {
		err := cmd.Wait()
		p.mu.Lock()
		p.waitErr = err
		p.mu.Unlock()
		close(done)
	}()

	return nil
}

// Wait blocks until the process exits. Safe to call concurrently with Stop:
// both read from the same close-once channel.
func (p *ExecProcess) Wait() error {
	p.mu.Lock()
	done := p.done
	p.mu.Unlock()
	if done == nil {
		return fmt.Errorf("%s: Wait called before Start", p.name)
	}
	<-done

	p.mu.Lock()
	defer p.mu.Unlock()
	return p.waitErr
}

func (p *ExecProcess) Stop(ctx context.Context) error {
	p.mu.Lock()
	cmd := p.cmd
	done := p.done
	p.mu.Unlock()

	if cmd == nil || cmd.Process == nil || done == nil {
		return nil
	}

	select {
	case <-done:
		return nil // already exited
	default:
	}

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		return nil // already gone
	}

	select {
	case <-done:
		return nil
	case <-time.After(p.stopGrace):
		_ = cmd.Process.Kill()
		<-done
		return nil
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		return ctx.Err()
	}
}

func (p *ExecProcess) Health(ctx context.Context) (contracts.HealthStatus, error) {
	if p.healthCheck != nil {
		return p.healthCheck(ctx)
	}

	p.mu.Lock()
	done := p.done
	p.mu.Unlock()

	if done == nil {
		return contracts.HealthStatus{Service: p.name, State: contracts.HealthUnknown, CheckedAt: time.Now()}, nil
	}
	select {
	case <-done:
		return contracts.HealthStatus{Service: p.name, State: contracts.HealthUnhealthy, Message: "process exited", CheckedAt: time.Now()}, nil
	default:
		return contracts.HealthStatus{Service: p.name, State: contracts.HealthHealthy, CheckedAt: time.Now()}, nil
	}
}
