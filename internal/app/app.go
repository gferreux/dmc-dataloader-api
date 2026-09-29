package app

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os/signal"
	"syscall"

	helloworldhandler "github.com/dekuple-labs/dmc-dataloader-api/internal/app/handler/helloworld"
	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/model"
)

type App struct {
	appConfig         model.AppConfig
	helloWorldHandler *helloworldhandler.Handler
}

func New(appConfig model.AppConfig, helloWorldHandler *helloworldhandler.Handler) *App {
	return &App{
		appConfig:         appConfig,
		helloWorldHandler: helloWorldHandler,
	}
}

func (a *App) Run(ctx context.Context) error {
	rootCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	mux := http.NewServeMux()

	mux.Handle("GET /hello", a.helloWorldHandler)

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("OK"))
	})

	server := http.Server{
		Addr:        a.appConfig.Server.Addr,
		ReadTimeout: a.appConfig.Server.ReadTimeout,
		Handler:     mux,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
	}

	errCh := make(chan error, 1)

	go func() {
		slog.Info("Server running...", "addr", a.appConfig.Server.Addr)

		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-rootCtx.Done():
	case err := <-errCh:
		return err
	}

	stop()

	shutdownCtx, cancel := context.WithTimeout(ctx, a.appConfig.Server.ShutdownTimeout)
	defer cancel()

	slog.Info("Shutting down server...")

	return server.Shutdown(shutdownCtx)
}
