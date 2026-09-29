package app

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os/signal"
	"syscall"

	loadconfighandler "github.com/dekuple-labs/dmc-dataloader-api/internal/app/handler/loadconfig"
	"github.com/dekuple-labs/dmc-dataloader-api/internal/app/httpx"
	"github.com/dekuple-labs/dmc-dataloader-api/internal/app/middleware"
	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/model"
	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/port"
)

// App is the HTTP process.
type App struct {
	appConfig model.AppConfig
	handler   *loadconfighandler.Handler
	authn     port.Authenticator
}

// New builds the application.
func New(appConfig model.AppConfig, handler *loadconfighandler.Handler, authn port.Authenticator) *App {
	return &App{
		appConfig: appConfig,
		handler:   handler,
		authn:     authn,
	}
}

// HTTPHandler returns the fully wired HTTP handler, including CORS and auth.
func (a *App) HTTPHandler() http.Handler {
	api := http.NewServeMux()
	api.HandleFunc("GET /api/v1/load-configs", a.handler.List)
	api.HandleFunc("POST /api/v1/load-configs/validate", a.handler.Validate)
	api.HandleFunc("POST /api/v1/load-configs/test-pattern", a.handler.TestPattern)
	api.HandleFunc("POST /api/v1/load-configs", a.handler.Create)
	api.HandleFunc("GET /api/v1/load-configs/{id}", a.handler.Get)
	api.HandleFunc("PUT /api/v1/load-configs/{id}", a.handler.Update)
	api.HandleFunc("DELETE /api/v1/load-configs/{id}", a.handler.Delete)
	api.HandleFunc("GET /api/v1/templates", a.handler.Templates)
	api.HandleFunc("GET /api/v1/meta", a.handler.Meta)
	api.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "route not found", nil)
	})

	protected := middleware.CORS(middleware.SplitOrigins(a.appConfig.Server.CORSOrigins))(
		middleware.Authenticate(a.authn)(api),
	)

	root := http.NewServeMux()
	root.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("OK"))
	})
	root.Handle("/", protected)

	return root
}

// Run serves HTTP until the process is signaled to stop.
func (a *App) Run(ctx context.Context) error {
	rootCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	server := http.Server{
		Addr:         a.appConfig.Server.Addr,
		ReadTimeout:  a.appConfig.Server.ReadTimeout,
		WriteTimeout: a.appConfig.Server.WriteTimeout,
		Handler:      a.HTTPHandler(),
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
