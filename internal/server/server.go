package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/sssstore/sssstore/internal/s3api"
	"github.com/sssstore/sssstore/internal/storage"
)

type metrics struct {
	requests uint64
	errors   uint64
}

func Run(bindAddr, dataDir string) error {
	store := storage.New(dataDir)
	s3 := s3api.New(store)
	m := &metrics{}

	handler := loggingMiddleware(m, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	srv := &http.Server{Addr: bindAddr, Handler: handler}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("sssstore listening on %s", bindAddr)
		errCh <- srv.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Printf("received signal: %s; shutting down", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(ctx)
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
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

func loggingMiddleware(m *metrics, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		cw := &captureWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(cw, r)
		atomic.AddUint64(&m.requests, 1)
		if cw.status >= 400 {
			atomic.AddUint64(&m.errors, 1)
		}
		log.Printf("method=%s path=%s status=%d remote=%s duration=%s", r.Method, r.URL.Path, cw.status, r.RemoteAddr, time.Since(start))
	})
}
