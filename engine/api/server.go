package api

import (
	"context"
	"net/http"
	"time"

	"herminas/kernel/errors"
)

// Config configures the HTTP server (cahier des charges §4.1: "serveur
// HTTP/2 + TLS optionnel + CORS dev").
type Config struct {
	Addr string
	// TLSCertFile/TLSKeyFile: both set enables TLS (and HTTP/2 via Go's
	// automatic h2 support over TLS); both empty serves plain HTTP/1.1 —
	// fine for local dev and for sitting behind a TLS-terminating proxy.
	TLSCertFile string
	TLSKeyFile  string
	// DevCORS, when true, sets permissive CORS headers (any origin) for
	// local frontend development (Vite dev server on a different port).
	// Never enable in a real deployment — M1.6 UI serving is same-origin.
	DevCORS bool
}

type Server struct {
	httpServer *http.Server
	cfg        Config
}

func NewServer(cfg Config, handler http.Handler) *Server {
	if cfg.DevCORS {
		handler = corsMiddleware(handler)
	}
	return &Server{
		cfg: cfg,
		httpServer: &http.Server{
			Addr:              cfg.Addr,
			Handler:           handler,
			ReadHeaderTimeout: 10 * time.Second,
		},
	}
}

// Start blocks until the server stops (Shutdown is called, or it fails to
// bind). Callers typically run it in a goroutine.
func (s *Server) Start() error {
	var err error
	if s.cfg.TLSCertFile != "" && s.cfg.TLSKeyFile != "" {
		err = s.httpServer.ListenAndServeTLS(s.cfg.TLSCertFile, s.cfg.TLSKeyFile)
	} else {
		err = s.httpServer.ListenAndServe()
	}
	if err != nil && err != http.ErrServerClosed {
		return errors.Wrap(errors.CodeInternal, "http server failed", err)
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
