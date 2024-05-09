package server

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/curioswitch/go-curiostack/config"
)

// NewRouter returns a new chi.Router with standard middleware.
func NewRouter() *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	return r
}

// NewServer returns a new http.Server with standard settings to serve the given router.
func NewServer(router http.Handler, conf config.Server) *http.Server {
	return &http.Server{
		Addr:              conf.Address,
		Handler:           h2c.NewHandler(router, &http2.Server{}),
		ReadHeaderTimeout: 3 * time.Second,
	}
}
