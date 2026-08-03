// Command herminas-control-plane is the Go composition root (L3). Modes,
// dispatched on argv[1]:
//
//   - (none)      kernel-only smoke test: proves settings load cleanly (M0.1)
//   - "run"       supervised startup: Redpanda → ClickHouse → ... (M0.5),
//     ordered per cahier des charges §5.3.1; blocks until SIGINT/SIGTERM
//   - "status"    health-checks the same services without starting anything
//   - "serve-api" starts the real HTTP API (M1.5): auth, RBAC, query
//     broker, datasets, ingest — needs ClickHouse reachable (start it with
//     "run" first, or point HERMINAS_CLICKHOUSE_URL elsewhere)
//
// The Rust data plane / Python intelligence slots in the "run" ordered
// startup are prepared but not yet registered: they only become
// long-running daemons in M1.2 (Rust gRPC server, its own binary) and M4
// (Python FastAPI). Registering today's one-shot bootstrap binaries here
// would misrepresent them as supervised daemons and produce a
// restart-crash loop.
package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"herminas/engine/api"
	"herminas/engine/auth"
	"herminas/engine/clickhouse"
	"herminas/engine/querybroker"
	"herminas/engine/redpanda"
	"herminas/engine/schemamgr"
	"herminas/engine/supervisor"
	"herminas/engine/wshub"
	"herminas/kernel/errors"
	"herminas/kernel/paths"
	"herminas/kernel/permissions"
	wsschemas "herminas/kernel/schemas"
	"herminas/kernel/settings"
)

func main() {
	switch arg(1) {
	case "run":
		runControlPlane()
	case "status":
		statusControlPlane()
	case "serve-api":
		serveAPI()
	default:
		checkKernel()
	}
}

func arg(i int) string {
	if len(os.Args) > i {
		return os.Args[i]
	}
	return ""
}

func loadSettingsOrExit() (*settings.Settings, paths.Layout) {
	layout := paths.Default()

	configPath := os.Getenv("HERMINAS_CONFIG")
	if configPath == "" {
		configPath = "config/herminas.example.yaml"
	}

	cfg, err := settings.Load(configPath)
	if err != nil {
		exitf("HermiNas control plane bootstrap FAILED: %v", err)
	}
	return cfg, layout
}

func checkKernel() {
	cfg, _ := loadSettingsOrExit()
	fmt.Println("HermiNas control plane bootstrap OK")
	fmt.Printf("  environment : %s\n", cfg.Environment())
	fmt.Printf("  http_port   : %d\n", cfg.HTTPPort())
	fmt.Printf("  grpc_port   : %d\n", cfg.GRPCPort())
	fmt.Printf("  data_dir    : %s\n", cfg.DataDir())
	fmt.Printf("  llm_provider: %s\n", cfg.LLMProvider())
}

func clickhouseConfig(layout paths.Layout) clickhouse.Config {
	return clickhouse.Config{
		HTTPPort: 8123,
		TCPPort:  9000,
		DataDir:  filepath.Join(layout.DataDir, "clickhouse"),
		BinDir:   filepath.Join(layout.RuntimeDir, "clickhouse", "bin"),
	}
}

func redpandaConfig(layout paths.Layout) redpanda.Config {
	return redpanda.Config{
		KafkaPort: 9092,
		AdminPort: 9644,
		RPCPort:   33145,
		DataDir:   filepath.Join(layout.DataDir, "redpanda"),
		BinDir:    filepath.Join(layout.RuntimeDir, "redpanda", "bin"),
	}
}

func runControlPlane() {
	_, layout := loadSettingsOrExit()

	if err := os.MkdirAll(layout.DataDir, 0o755); err != nil {
		exitf("cannot create data dir: %v", err)
	}

	s := supervisor.New()

	// Ordered startup per cahier des charges §5.3.1: Redpanda → ClickHouse
	// → Rust → Python → API. The last three slots join once they expose
	// long-running daemons (M1.2, M4, M1.5).
	if runtime.GOOS == "linux" {
		rpCfg := redpandaConfig(layout)
		if err := redpanda.GenerateConfig(rpCfg); err != nil {
			exitf("generate redpanda config: %v", err)
		}
		s.Register(redpanda.NewProcess(rpCfg), 30*time.Second, supervisor.Backoff{})
	} else {
		fmt.Printf("supervisor: skipping Redpanda on %s/%s — Linux-only binary (engine/redpanda is unit-tested via mocks; see tasks-herminas.md M0.5)\n", runtime.GOOS, runtime.GOARCH)
	}

	chCfg := clickhouseConfig(layout)
	if err := clickhouse.GenerateConfig(chCfg); err != nil {
		exitf("generate clickhouse config: %v", err)
	}
	if _, err := os.Stat(chCfg.BinaryPath()); err != nil {
		exitf("clickhouse binary not found at %s — run clickhouse.Download first", chCfg.BinaryPath())
	}
	s.Register(clickhouse.NewProcess(chCfg), 30*time.Second, supervisor.Backoff{})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	fmt.Println("supervisor: starting services...")
	if err := s.Start(ctx); err != nil {
		_ = s.Stop(context.Background())
		exitf("supervisor: start failed: %v", err)
	}
	fmt.Println("supervisor: all services healthy")

	<-ctx.Done()
	fmt.Println("supervisor: shutting down...")
	if err := s.Stop(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "supervisor: stop error: %v\n", err)
	}
	fmt.Println("supervisor: stopped cleanly")
}

func statusControlPlane() {
	_, layout := loadSettingsOrExit()

	s := supervisor.New()
	if runtime.GOOS == "linux" {
		s.Register(redpanda.NewProcess(redpandaConfig(layout)), 0, supervisor.Backoff{})
	}
	s.Register(clickhouse.NewProcess(clickhouseConfig(layout)), 0, supervisor.Backoff{})

	for _, st := range s.Status(context.Background()) {
		fmt.Printf("%-14s %-10s %s\n", st.Service, st.State, st.Message)
	}
}

func serveAPI() {
	cfg, layout := loadSettingsOrExit()

	if err := os.MkdirAll(layout.DataDir, 0o755); err != nil {
		exitf("cannot create data dir: %v", err)
	}

	clickhouseURL := os.Getenv("HERMINAS_CLICKHOUSE_URL")
	if clickhouseURL == "" {
		clickhouseURL = "http://127.0.0.1:8123"
	}

	authStore, err := auth.Open(filepath.Join(layout.DataDir, "auth.db"))
	if err != nil {
		exitf("open auth store: %v", err)
	}

	schemas, err := schemamgr.Open(filepath.Join(layout.DataDir, "schemamgr.db"))
	if err != nil {
		exitf("open schemamgr store: %v", err)
	}

	broker, err := querybroker.New(querybroker.Config{
		ClickHouseURL: clickhouseURL,
		AuditLogPath:  filepath.Join(layout.DataDir, "audit.jsonl"),
	})
	if err != nil {
		exitf("open query broker: %v", err)
	}

	if err := bootstrapAdminUser(authStore); err != nil {
		exitf("bootstrap admin user: %v", err)
	}

	staticDir := os.Getenv("HERMINAS_STATIC_DIR")
	if staticDir == "" {
		staticDir = "web/dist"
	}

	jwtManager := auth.NewJWTManager(jwtSecretFromEnv(), time.Hour)
	devCORS := os.Getenv("HERMINAS_DEV_CORS") == "1"

	router := api.NewRouter(api.Deps{
		AuthStore:          authStore,
		JWTManager:         jwtManager,
		Schemas:            schemas,
		Broker:             broker,
		Inserter:           api.NewInserter(clickhouseURL),
		DDL:                api.NewClickHouseDDLExecutor(clickhouseURL),
		RateLimitPerMinute: 600,
		StaticDir:          staticDir,
	})

	// M2.1: the WebSocket hub lives outside engine/api (a sibling layer,
	// not nested under it) so engine/api doesn't need to depend on it;
	// mounting "/api/v1/ws" on an outer mux ahead of the API router is
	// enough for Go 1.22's ServeMux to route that one exact path to the
	// hub and everything else through to the router unchanged.
	wsSchemas, err := wsschemas.LoadWebSocketSchemas()
	if err != nil {
		exitf("load websocket schemas: %v", err)
	}
	hub := wshub.New(wsSchemas)
	mux := http.NewServeMux()
	mux.Handle("/api/v1/ws", wshub.ServeWS(hub, jwtManager, brokerQueryRunner{broker}, devCORS))
	mux.Handle("/", router)

	addr := fmt.Sprintf(":%d", cfg.HTTPPort())
	srv := api.NewServer(api.Config{Addr: addr, DevCORS: devCORS}, mux)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	startSystemHealthPoller(ctx, hub, clickhouseURL)

	go func() {
		<-ctx.Done()
		fmt.Println("herminas-api: shutting down...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	fmt.Printf("herminas-api: listening on %s (clickhouse: %s, static: %s)\n", addr, clickhouseURL, staticDir)
	if err := srv.Start(); err != nil {
		exitf("api server failed: %v", err)
	}
	fmt.Println("herminas-api: stopped cleanly")
}

// brokerQueryRunner adapts *querybroker.Broker (M1.4) to wshub.QueryRunner
// (M2.1) — the two packages don't need to know about each other beyond
// this shape, matching how api.NewClickHouseDDLExecutor adapts ClickHouse
// to schemamgr.DDLExecutor.
type brokerQueryRunner struct {
	broker *querybroker.Broker
}

func (r brokerQueryRunner) Execute(ctx context.Context, userID, sql string) (wshub.QueryResult, error) {
	result, err := r.broker.Execute(ctx, userID, sql)
	if err != nil {
		return wshub.QueryResult{}, err
	}
	return wshub.QueryResult{Rows: result.Rows}, nil
}

// startSystemHealthPoller pings ClickHouse every 10s and broadcasts the
// result as a system.health event (M2.1). It's deliberately independent
// of engine/supervisor's own health checks — serve-api doesn't start or
// own the supervisor (that's the separate "run" mode), it only knows
// ClickHouse's URL, so this is a minimal, honest ping rather than a
// re-use of machinery that isn't running in this process.
func startSystemHealthPoller(ctx context.Context, hub *wshub.Hub, clickhouseURL string) {
	client := &http.Client{Timeout: 3 * time.Second}

	poll := func() {
		state, message := "healthy", ""
		resp, err := client.Get(clickhouseURL + "/ping")
		switch {
		case err != nil:
			state, message = "unhealthy", err.Error()
		case resp.StatusCode != http.StatusOK:
			state, message = "unhealthy", fmt.Sprintf("ping returned HTTP %d", resp.StatusCode)
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		_ = hub.BroadcastSystemHealth("clickhouse", state, message, time.Now().UTC().Format(time.RFC3339))
	}

	go func() {
		poll()
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				poll()
			}
		}
	}()
}

// jwtSecretFromEnv reads HERMINAS_JWT_SECRET, or falls back to an
// ephemeral random secret for local dev. Persisting a generated secret
// (so sessions survive a restart) depends on the AES-256-GCM
// secrets-at-rest work planned for M0.4/M8 — not implemented yet, so this
// is honest about the tradeoff rather than silently writing a secret to
// disk in plaintext.
func jwtSecretFromEnv() []byte {
	if s := os.Getenv("HERMINAS_JWT_SECRET"); s != "" {
		return []byte(s)
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		exitf("generate ephemeral JWT secret: %v", err)
	}
	fmt.Fprintln(os.Stderr, "herminas-api: WARNING: HERMINAS_JWT_SECRET not set — using an ephemeral secret, sessions won't survive a restart")
	return buf
}

// bootstrapAdminUser creates the initial admin account from
// HERMINAS_BOOTSTRAP_ADMIN_USER/_PASSWORD if both are set and no such user
// exists yet — a minimal stand-in for M8.2's Setup Wizard, useful today
// for smoke tests and local dev without a UI to create the first account.
func bootstrapAdminUser(store *auth.Store) error {
	username := os.Getenv("HERMINAS_BOOTSTRAP_ADMIN_USER")
	password := os.Getenv("HERMINAS_BOOTSTRAP_ADMIN_PASSWORD")
	if username == "" || password == "" {
		return nil
	}
	if _, err := store.VerifyPassword(username, password); err == nil {
		return nil // already bootstrapped with this exact password
	}
	_, err := store.CreateUser(username, password, permissions.RoleAdmin)
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	if err == nil {
		fmt.Printf("herminas-api: bootstrapped admin user %q\n", username)
	}
	return nil
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
