package server

import (
	"net/http"
	"strings"
	"time"

	"github.com/curioswitch/go-usegcp/middleware/requestlog"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/curioswitch/go-curiostack/config"
	"github.com/curioswitch/go-curiostack/otel"
)

// NewMux returns a new chi.Mux with standard middleware.
func NewMux() *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(otel.HTTPMiddleware())
	r.Use(middleware.Maybe(requestlog.NewMiddleware(), func(r *http.Request) bool {
		return !strings.HasPrefix(r.URL.Path, "/internal/")
	}))

	r.Get("/internal/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	return r
}

// NewServer returns a new http.Server with standard settings to serve the given router.
func NewServer(router http.Handler, conf *config.Common) *http.Server {
	return &http.Server{
		Addr:              conf.Server.Address,
		Handler:           h2c.NewHandler(router, &http2.Server{}),
		ReadHeaderTimeout: 3 * time.Second,
	}
}
