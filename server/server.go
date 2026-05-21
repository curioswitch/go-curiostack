package server

import (
	"net/http"
	"strings"
	"time"

	"github.com/curioswitch/go-usegcp/middleware/requestlog"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

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

	return r
}

// NewServer returns a new http.Server with standard settings to serve the given router.
func NewServer(router http.Handler, conf *config.Common) *http.Server {
	protocols := &http.Protocols{}
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)

	return &http.Server{
		Addr:              conf.Server.Address,
		Handler:           router,
		Protocols:         protocols,
		ReadHeaderTimeout: 3 * time.Second,
	}
}
