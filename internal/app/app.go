package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"simple_auth_by_doubao/internal/config"
	"simple_auth_by_doubao/internal/httpapi"
)

func Run(ctx context.Context) error {
	cfg, err := config.Load(".env")
	if err != nil {
		return err
	}

	server := &http.Server{
		Addr:              ":" + cfg.ServerPort,
		Handler:           httpapi.NewServer(cfg),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("auth service listening on http://127.0.0.1:%s", cfg.ServerPort)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("start http server: %w", err)
	}
}
