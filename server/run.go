package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	docshandler "github.com/curioswitch/go-docs-handler"
	protodocs "github.com/curioswitch/go-docs-handler/plugins/proto"
	"github.com/go-chi/chi/v5"
	"google.golang.org/protobuf/proto"

	"github.com/curioswitch/go-curiostack/config"
	"github.com/curioswitch/go-curiostack/logging"
	"github.com/curioswitch/go-curiostack/otel"
)

type protoDocsRequests struct {
	procedure string
	reqs      []proto.Message
}

// Server is the configuration of the server that will be run.
//
// Note that operations on Server are all functions rather than methods to
// allow use of generics where appropriate.
type Server struct {
	mux *chi.Mux

	conf *config.Common

	protoDocsRequests  []protoDocsRequests
	docsFirebaseDomain string

	startCalled bool
}

// Mux returns the [chi.Mux] that will be served and can be used to add
// route handlers.
//
// For connect handlers it is strongly preferred to use HandleConnectUnary
// instead to allow registration of default interceptors and docs handler.
func Mux(s *Server) *chi.Mux {
	return s.mux
}

// EnableDocsFirebaseAuth enables Firebase auth for the docs handler. The domain
// must be the auth domain of the Firebase project.
//
// Firebase credentials of the browser, generally set by running the actual web app
// locally, will be read and added as authorization headers in requests to the
// server.
func EnableDocsFirebaseAuth(s *Server, domain string) {
	s.docsFirebaseDomain = domain
}

// Start starts the server, listening on the configured server address for requests
// based on its configuration. This method will block until program exit.
func Start(ctx context.Context, s *Server) error {
	s.startCalled = true

	if err := s.mountDefaultEndpoints(); err != nil {
		return err
	}

	srv := NewServer(s.mux, s.conf)
	defer func() {
		if err := srv.Close(); err != nil {
			slog.WarnContext(ctx, "Failed to close server", "error", err)
		}
	}()

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

func (b *Server) mountDefaultEndpoints() error {
	docsDefined := false
	healthDefined := false
	// Define /internal/health if not already defined.
	_ = chi.Walk(b.mux, func(_, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		switch route {
		case "/internal/docs/*":
			docsDefined = true
		case "/internal/health":
			healthDefined = true
		}
		return nil
	})

	if !docsDefined {
		var services []string
		for _, r := range b.protoDocsRequests {
			svc, _, _ := strings.Cut(r.procedure[1:], "/")
			if !slices.Contains(services, svc) {
				services = append(services, svc)
			}
		}
		if len(services) > 0 {
			var docopts []docshandler.Option
			protodocopts := make([]protodocs.Option, 0, len(b.protoDocsRequests)+len(services)-1)

			for _, r := range b.protoDocsRequests {
				protodocopts = append(protodocopts, protodocs.WithExampleRequests(r.procedure, r.reqs[0], r.reqs[1:]...))
			}

			for _, svc := range services[1:] {
				protodocopts = append(protodocopts, protodocs.WithAdditionalService(svc))
			}

			if b.docsFirebaseDomain != "" {
				script := docsFirebaseAuthScript(b.docsFirebaseDomain)
				docopts = append(docopts, docshandler.WithInjectedScriptSupplier(func() string {
					return script
				}))
			}

			docs, err := docshandler.New(protodocs.NewPlugin(services[0], protodocopts...), docopts...)
			if err != nil {
				return fmt.Errorf("server: create docs handler: %w", err)
			}
			b.mux.Handle("/internal/docs/*", http.StripPrefix("/internal/docs", docs))
		}
	}

	if !healthDefined {
		b.mux.Get("/internal/health", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
	}

	return nil
}

func docsFirebaseAuthScript(domain string) string {
	return fmt.Sprintf(`
		function include(url) {
			return new Promise((resolve, reject) => {
				var script = document.createElement('script');
				script.type = 'text/javascript';
				script.src = url;

				script.onload = function() {
					resolve({ script });
				};

				document.getElementsByTagName('head')[0].appendChild(script);
			});
		}

		async function loadScripts() {
			await include("https://%s/__/firebase/8.10.1/firebase-app.js");
			await include("https://%s/__/firebase/8.10.1/firebase-auth.js");
			await include("https://%s/__/firebase/init.js");
			firebase.auth();
		}
		loadScripts();

		async function getAuthorization() {
			const token = await firebase.auth().currentUser.getIdToken();
			return {"Authorization": "Bearer " + token};
		}
		window.armeria.registerHeaderProvider(getAuthorization);
	`, domain, domain, domain)
}
