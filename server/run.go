package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/curioswitch/go-curiostack/config"
	"github.com/curioswitch/go-curiostack/logging"
	"github.com/curioswitch/go-curiostack/otel"
)

// Server is the configuration of the server that will be run.
type Server struct {
	mux *chi.Mux

	conf *config.Common

	startCalled bool
}

// Mux returns the [chi.Mux] that will be served and can be used to add
// route handlers.
func (b *Server) Mux() *chi.Mux {
	return b.mux
}

// Start starts the server, listening on the configured server address for requests
// based on its configuration. This method will block until program exit.
func (b *Server) Start(ctx context.Context) error {
	b.startCalled = true

	srv := NewServer(b.mux, b.conf)
	defer srv.Close()

	slog.InfoContext(ctx, fmt.Sprintf("Starting server on address %v", srv.Addr))
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server: failed to start server: %w", err)
	}
	return nil
}

// Main is the entrypoint for starting a server using CurioStack.
// It should be called from your main function and passed a pointer to your
// configuration object which embeds [config.Common]. The setup callback
// should set up handlers and such using methods on Server to define the
// resulting server. It must finish setup by calling Server.Start
// to start the server.
//
// If you need to define default values before loading user config, they should
// be set on the config before passing, or otherwise it is fine to just pass a
// pointer to an empty struct. confFiles is a [fs.FS] to resolve config files as
// used by [config.Load].
//
// An exit code is returned, so the general pattern for this function will
// be to call [os.Exit] with the result of this function.
func Main[T config.CurioStack](conf T, confFiles fs.FS, run func(ctx context.Context, conf T, b *Server) error) int {
	ctx := context.Background()

	otel.Initialize() // initialize as early as possible to instrument globals

	if err := config.Load(conf, confFiles); err != nil {
		slog.Error(fmt.Sprintf("Failed to load config: %v", err))
		return 1
	}

	logging.Initialize(&conf.GetCommon().Logging)

	b := &Server{
		mux: NewMux(),

		conf: conf.GetCommon(),
	}

	if err := run(ctx, conf, b); err != nil {
		slog.Error(fmt.Sprintf("Failed to configure server: %v", err))
		return 1
	}

	if !b.startCalled {
		slog.Error("Start was not called on server.Builder, it must be called in your run callback to start the server.")
		return 1
	}

	return 0
}
