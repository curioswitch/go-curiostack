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
)

// Builder allows configuring the server that will be launched by Main.
type Builder struct {
	mux *chi.Mux
}

func (b *Builder) Mux() *chi.Mux {
	return b.mux
}

// Main is the entrypoint for starting a server using CurioStack.
// It should be called from your main function and passed a pointer to your
// configuration object which embeds [config.Common]. The setup callback
// should set up handlers and such using methods on Builder to define the
// resulting server.
//
// If you need to define default values before loading user config, they should
// be set on the config before passing, or otherwise it is fine to just pass a
// pointer to an empty struct. confFiles is a [fs.FS] to resolve config files as
// used by [config.Load].
//
// An exit code is returned, so the general pattern for this function will
// be to call [os.Exit] with the result of this function.
func Main[T config.CurioStack](conf T, confFiles fs.FS, setup func(ctx context.Context, conf T, b *Builder) error) int {
	ctx := context.Background()

	if err := config.Load(conf, confFiles); err != nil {
		slog.Error(fmt.Sprintf("Failed to load config: %v", err))
		return 1
	}

	logging.Initialize(&conf.GetCommon().Logging)

	b := &Builder{
		mux: NewMux(),
	}
	if err := setup(ctx, conf, b); err != nil {
		slog.Error(fmt.Sprintf("Failed to build server: %v", err))
		return 1
	}

	srv := NewServer(b.mux, conf.GetCommon())

	slog.InfoContext(ctx, fmt.Sprintf("Starting server on address %v", srv.Addr))
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		slog.ErrorContext(ctx, fmt.Sprintf("Failed to start server: %v", err))
		return 1
	}

	return 0
}
