package app

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mark3labs/mcp-go/server"
)

const shutdownTimeout = 10 * time.Second

func serveHTTP(mcpServer *server.MCPServer, cfg config) error {
	opts := []server.StreamableHTTPOption{
		server.WithHeartbeatInterval(30 * time.Second),
		server.WithSessionIdleTTL(24 * time.Hour),
	}
	if len(cfg.corsOrigins) > 0 {
		opts = append(opts, server.WithStreamableHTTPCORS(
			server.WithCORSAllowedOrigins(cfg.corsOrigins...),
			server.WithCORSAllowedMethods(http.MethodGet, http.MethodPost, http.MethodDelete, http.MethodOptions),
			server.WithCORSAllowedHeaders("Content-Type", server.HeaderKeySessionID),
			server.WithCORSExposedHeaders(server.HeaderKeySessionID),
		))
	}

	streamableSrv := server.NewStreamableHTTPServer(mcpServer, opts...)
	mux := http.NewServeMux()
	mux.Handle(cfg.mcpEndpoint, streamableSrv)
	mux.HandleFunc(cfg.healthEndpoint, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})

	httpServer := &http.Server{
		Addr:              cfg.addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("PixelsMcp listening on %s%s", cfg.addr, cfg.mcpEndpoint)
		errCh <- httpServer.ListenAndServe()
	}()

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stopCh)

	select {
	case sig := <-stopCh:
		log.Printf("received %s, shutting down", sig)
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		shutdownErr := streamableSrv.Shutdown(ctx)
		if err := httpServer.Shutdown(ctx); err != nil && shutdownErr == nil {
			shutdownErr = err
		}
		return shutdownErr
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
