package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/curioswitch/go-curiostack/config"
	"github.com/curioswitch/go-curiostack/logging"
)

// Run starts a server with the given configuration and options.
// It returns an integer error code based on success or failure.
// Run should generally be the last line in your main function and
// passed to [os.Exit].
//
//	func main() {
//	  os.Exit(server.Run(ctx, conf.Common, opts...))
//	}
func Run(ctx context.Context, conf *config.Common, opts ...Option) int {
	c := &runConfig{}
	for _, o := range opts {
		o.apply(c)
	}

	logging.Initialize(&conf.Logging)

	mux := NewMux()

	for _, setup := range c.setupMux {
		if err := setup(mux); err != nil {
			slog.ErrorContext(ctx, fmt.Sprintf("Failed to setup mux: %v", err))
			return 1
		}
	}

	srv := NewServer(mux, conf)

	slog.InfoContext(ctx, fmt.Sprintf("Starting server on address %v", srv.Addr))
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		slog.ErrorContext(ctx, fmt.Sprintf("Failed to start server: %v", err))
		return 1
	}

	return 0
}

type runConfig struct {
	setupMux []func(mux *chi.Mux) error
}

// Option is a configuration option for Run.
type Option interface {
	apply(conf *runConfig)
}

// SetupMux adds a function to setup the [chi.Mux] served by the server.
func SetupMux(setup func(mux *chi.Mux) error) Option {
	return setupMuxOption(setup)
}

type setupMuxOption func(mux *chi.Mux) error

func (o setupMuxOption) apply(conf *runConfig) {
	conf.setupMux = append(conf.setupMux, o)
}
