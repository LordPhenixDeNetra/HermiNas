package api

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	"herminas/engine/auth"
	"herminas/engine/querybroker"
	"herminas/engine/schemamgr"
	"herminas/kernel/errors"
	"herminas/kernel/permissions"
)

// Deps are every dependency the router needs — one struct so wiring it in
// bootstrap.go (or a future cmd/herminas) is a single call, and so tests
// can swap in fresh in-memory-backed stores per test.
type Deps struct {
	AuthStore          *auth.Store
	JWTManager         *auth.JWTManager
	Schemas            *schemamgr.Store
	Broker             *querybroker.Broker
	Inserter           *Inserter
	DDL                schemamgr.DDLExecutor // applies generated DDL to ClickHouse; nil means metadata-only (tests)
	RateLimitPerMinute int                   // per-IP, applied before auth; 0 disables it
	StaticDir          string                // built frontend (web/dist); empty disables static serving (tests, API-only deploys)
}

// NewRouter wires the routes listed in cahier des charges §6.1 that exist
// as of M1.5: /api/v1/health, /api/v1/auth/login, /api/v1/query,
// /api/v1/datasets*, /api/v1/ingest/{dataset}. Routes not yet backed by a
// real implementation (pipelines, alerts, ml, agents, admin/users) aren't
// registered — a 404 is a more honest response than a stub that pretends
// to work.
func NewRouter(deps Deps) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/health", healthHandler)
	mux.HandleFunc("POST /api/v1/auth/login", loginHandler(deps.AuthStore, deps.JWTManager))

	authenticate := auth.Authenticate(deps.JWTManager, deps.AuthStore)

	mux.Handle("POST /api/v1/query",
		authenticate(auth.RequireRole(permissions.RoleViewer)(queryHandler(deps.Broker))))

	// schemamgr's own handler dispatches GET vs POST internally; RBAC here
	// only needs to know "read" (viewer) vs "write" (engineer), not the
	// exact route, so one method-sniffing wrapper covers all of them.
	schemaHandler := schemamgr.NewHandler(deps.Schemas)
	if deps.DDL != nil {
		schemaHandler = schemaHandler.WithDDLExecutor(deps.DDL)
	}
	datasetRoutes := authenticate(datasetRBAC(schemaHandler.Routes()))
	mux.Handle("/api/v1/datasets", datasetRoutes)
	mux.Handle("/api/v1/datasets/", datasetRoutes)

	mux.Handle("POST /api/v1/ingest/{dataset}",
		authenticate(auth.RequireDatasetScope(datasetFromPath)(ingestHandler(deps.Schemas, deps.DDL, deps.Inserter))))

	if staticDirAvailable(deps.StaticDir) {
		mux.Handle("/", SPAHandler(deps.StaticDir))
	}

	var handler http.Handler = mux
	if deps.RateLimitPerMinute > 0 {
		handler = RateLimit(deps.RateLimitPerMinute, ByRemoteIP)(handler)
	}
	return handler
}

func datasetFromPath(r *http.Request) string { return r.PathValue("dataset") }

func datasetRBAC(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := auth.IdentityFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
			return
		}
		min := permissions.RoleViewer
		if r.Method != http.MethodGet {
			min = permissions.RoleEngineer
		}
		if !permissions.AtLeast(id.Role, min) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "insufficient role"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	// Minimal liveness check for M1.5: proves the API process itself is up
	// and answering. Aggregating ClickHouse/Redpanda/Rust/Python health
	// (cahier §6.1's full intent for this route) needs supervisor
	// wiring — engine/supervisor lives in the bootstrap composition root,
	// not in this package, so that aggregation is a follow-up, not scope
	// creep avoided by accident.
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string `json:"token"`
	Role  string `json:"role"`
}

func loginHandler(store *auth.Store, jwtMgr *auth.JWTManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
			return
		}

		user, err := store.VerifyPassword(req.Username, req.Password)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid username or password"})
			return
		}

		token, err := jwtMgr.Issue(user.Username, user.Role)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to issue session token"})
			return
		}
		writeJSON(w, http.StatusOK, loginResponse{Token: token, Role: string(user.Role)})
	}
}

type queryRequest struct {
	SQL string `json:"sql"`
}

func queryHandler(broker *querybroker.Broker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req queryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
			return
		}

		id, _ := auth.IdentityFromContext(r.Context())
		result, err := broker.Execute(r.Context(), id.Subject, req.SQL)
		if err != nil {
			status := http.StatusInternalServerError
			switch {
			case errors.IsResourceExhausted(err):
				status = http.StatusTooManyRequests
			case errors.IsInvalidArgument(err):
				status = http.StatusBadRequest
			}
			writeJSON(w, status, map[string]string{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmtWriteQueryResult(w, result)
	}
}

func fmtWriteQueryResult(w http.ResponseWriter, result *querybroker.Result) {
	var buf bytes.Buffer
	buf.WriteString(`{"row_count":`)
	buf.WriteString(strconv.Itoa(result.RowCount))
	buf.WriteString(`,"cached":`)
	buf.WriteString(strconv.FormatBool(result.Cached))
	buf.WriteString(`,"rows":[`)
	for i, row := range result.Rows {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.Write(row)
	}
	buf.WriteString(`]}`)
	_, _ = w.Write(buf.Bytes())
}

func ingestHandler(schemas *schemamgr.Store, ddl schemamgr.DDLExecutor, inserter *Inserter) http.HandlerFunc {
	// Per-process cache of datasets we've already confirmed have a real
	// ClickHouse table, so a busy dataset doesn't re-issue `CREATE TABLE
	// IF NOT EXISTS` on every single ingest call — only ever the first
	// time this dataset is seen since the process started.
	var ensured sync.Map // dataset name -> struct{}

	return func(w http.ResponseWriter, r *http.Request) {
		dataset := r.PathValue("dataset")

		var records []map[string]any
		scanner := bufio.NewScanner(r.Body)
		for scanner.Scan() {
			line := bytes.TrimSpace(scanner.Bytes())
			if len(line) == 0 {
				continue
			}
			var rec map[string]any
			if err := json.Unmarshal(line, &rec); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON line: " + err.Error()})
				return
			}
			rec["received_at_unix_ms"] = float64(time.Now().UnixMilli())
			records = append(records, rec)
		}
		if len(records) == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no records in request body"})
			return
		}

		def, _, err := schemas.EnsureDataset(dataset, records[0])
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "ensure dataset: " + err.Error()})
			return
		}

		if ddl != nil {
			if _, ok := ensured.Load(dataset); !ok {
				if err := ddl.Exec(r.Context(), schemamgr.GenerateDDL(def)); err != nil {
					writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "create clickhouse table: " + err.Error()})
					return
				}
				ensured.Store(dataset, struct{}{})
			}
		}

		if err := inserter.Insert(r.Context(), dataset, records); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "insert failed: " + err.Error()})
			return
		}

		writeJSON(w, http.StatusAccepted, map[string]int{"accepted": len(records)})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
