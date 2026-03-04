package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/sssstore/sssstore/internal/audit"
	"github.com/sssstore/sssstore/internal/auth"
	"github.com/sssstore/sssstore/internal/config"
	"github.com/sssstore/sssstore/internal/s3api"
	"github.com/sssstore/sssstore/internal/storage"
)

type metrics struct {
	requests uint64
	errors   uint64
}

func Run(cfg config.Config) error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	store := storage.New(cfg.DataDir)
	defer store.Close()
	if cfg.ReplicationMode == "local_mirror" && cfg.ReplicationDir != "" {
		store.EnableReplication(cfg.ReplicationDir)
	}

	auditLog, err := audit.New(cfg.AuditLogPath)
	if err != nil {
		return err
	}
	defer auditLog.Close()

	adminUser := config.User{
		Name:      "admin",
		AccessKey: cfg.AdminAccessKey,
		SecretKey: cfg.AdminSecretKey,
		Policy:    auth.PolicyAdmin,
	}
	users, _ := config.LoadUsers(cfg.DataDir)
	users = append(users, adminUser)
	a := auth.NewSigV4Authenticator(users)
	s3 := s3api.New(store, a)
	m := &metrics{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go runLifecycleWorker(ctx, logger, auditLog, store, time.Duration(cfg.MultipartMaxAgeHours)*time.Hour)

	handler := loggingMiddleware(logger, auditLog, m, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz", "/readyz":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
			return
		case "/metrics":
			w.Header().Set("Content-Type", "text/plain; version=0.0.4")
			_, _ = fmt.Fprintf(w, "sssstore_requests_total %d\n", atomic.LoadUint64(&m.requests))
			_, _ = fmt.Fprintf(w, "sssstore_request_errors_total %d\n", atomic.LoadUint64(&m.errors))
			return
		default:
			s3.ServeHTTP(w, r)
		}
	}))

	srv := &http.Server{Addr: cfg.BindAddr, Handler: handler}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("sssstore listening", "addr", cfg.BindAddr, "tls", cfg.TLSCertFile != "")
		if cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
			errCh <- srv.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile)
			return
		}
		errCh <- srv.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		logger.Info("received signal; shutting down", "signal", sig.String())
		cancel()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		cancel()
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func runLifecycleWorker(ctx context.Context, logger *slog.Logger, auditLog *audit.Logger, store *storage.Store, maxAge time.Duration) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			removed, err := store.CleanupStaleMultipartUploads(maxAge)
			if err != nil {
				logger.Error("lifecycle cleanup failed", "error", err)
				auditLog.Log(audit.Event{Action: "lifecycle.cleanup", Message: err.Error()})
				continue
			}
			if removed > 0 {
				logger.Info("lifecycle cleanup removed stale multipart uploads", "removed", removed)
				auditLog.Log(audit.Event{Action: "lifecycle.cleanup", Message: fmt.Sprintf("removed=%d", removed)})
			}
		}
	}
}

type captureWriter struct {
	http.ResponseWriter
	status int
}

func (w *captureWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func loggingMiddleware(logger *slog.Logger, auditLog *audit.Logger, m *metrics, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		cw := &captureWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(cw, r)
		atomic.AddUint64(&m.requests, 1)
		if cw.status >= 400 {
			atomic.AddUint64(&m.errors, 1)
		}
		key := auth.AccessKeyFromRequest(r)
		auditLog.Log(audit.Event{Action: "http.request", Method: r.Method, Path: r.URL.Path, Status: cw.status, Principal: key, Remote: r.RemoteAddr})
		logger.Info("http request", "method", r.Method, "path", r.URL.Path, "status", cw.status, "principal", key, "remote", r.RemoteAddr, "duration_ms", time.Since(start).Milliseconds())
	})
}
