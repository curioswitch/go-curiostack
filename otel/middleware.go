package otel

import (
	"log"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	"connectrpc.com/otelconnect"
	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
)

// HTTPMiddleware returns http.Handler middleware configured with
// tracing and metrics.
func HTTPMiddleware() func(http.Handler) http.Handler {
	opts := []otelhttp.Option{
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			// OTel semantic convention, unclear why otel-go uses a fixed operation
			// name by default.
			return r.Method
		}),
		otelhttp.WithMeterProvider(meterProvider),
		otelhttp.WithTracerProvider(tracerProvider),
		otelhttp.WithFilter(func(r *http.Request) bool {
			return !strings.HasPrefix(r.URL.Path, "/internal/")
		}),
	}

	mw := otelhttp.NewMiddleware("", opts...)
	return func(h http.Handler) http.Handler {
		// The route pattern is populated after the logic handler is invoked, so we need
		// to make sure we update the name after, not before as WithSpanNameFormatter does.
		// And we need to make sure it is still within the context of the OTel middleware,
		// so we wrap the logic handler.
		wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.ServeHTTP(w, r)
			ctx := r.Context()
			if rctx, s := chi.RouteContext(ctx), trace.SpanFromContext(ctx); rctx != nil {
				s.SetName(rctx.RoutePattern())
			}
		})
		return mw(wrapped)
	}
}

// ConnectInterceptor returns a connect.Interceptor configured with
// tracing and metrics.
func ConnectInterceptor() connect.Interceptor {
	i, err := otelconnect.NewInterceptor(
		otelconnect.WithMeterProvider(meterProvider),
		otelconnect.WithTracerProvider(tracerProvider),
		otelconnect.WithoutServerPeerAttributes(),
		otelconnect.WithTrustRemote(),
	)
	if err != nil {
		log.Fatalf("failed to create connect interceptor: %v\n", err)
	}
	return i
}
