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

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	docshandler "github.com/curioswitch/go-docs-handler"
	protodocs "github.com/curioswitch/go-docs-handler/plugins/proto"
	"github.com/curioswitch/go-usegcp/middleware/requestlog"
	"github.com/go-chi/chi/v5"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

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

	serverInterceptors   []connect.ServerInterceptor
	serviceRegistrations []func(s *connect.Server)

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
func (s *Server) Mux() *chi.Mux {
	return s.mux
}

// AddServerInterceptor adds a server interceptor to the server. Interceptors are applied
// to all connect services registered with the server. They are executed in the order
// they are added after default interceptors applied by curiostack.
func (s *Server) AddServerInterceptor(interceptor connect.ServerInterceptor) {
	s.serverInterceptors = append(s.serverInterceptors, interceptor)
}

// AddConnectService registers a connect service with the server. registerFn is
// the registration function in the generated stub, and svc is the service implementation.
func (s *Server) AddConnectService[Service any](registerFn func(s *connect.Server, svc Service), svc Service) {
	s.serviceRegistrations = append(s.serviceRegistrations, func(cs *connect.Server) {
		registerFn(cs, svc)
	})
}

// AddSampleRequests adds sample requests for a given procedure to be used in the docs handler.
func (s *Server) AddSampleRequests[Req proto.Message](procedure string, reqs ...Req) {
	p, ok := strings.CutPrefix(procedure, "/")
	if !ok {
		panic("procedure must be a constant from connect generated code, such as curioapi.CurioServiceMethodProcedure")
	}
	svcName, methodName, ok := strings.Cut(p, "/")
	if !ok {
		panic("procedure must be a constant from connect generated code, such as curioapi.CurioServiceMethodProcedure")
	}

	svc, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(svcName))
	if err != nil {
		// As long as the procedure was passed from generated code, we are sure the proto was imported and automatically
		// added to the global registry.
		panic("procedure must be a constant from connect generated code, such as curioapi.CurioServiceMethodProcedure")
	}

	svcDesc, ok := svc.(protoreflect.ServiceDescriptor)
	if !ok {
		panic("svc must be a service descriptor")
	}
	method := svcDesc.Methods().ByName(protoreflect.Name(methodName))
	if method == nil {
		panic("procedure must be a constant from connect generated code, such as curioapi.CurioServiceMethodProcedure")
	}

	reqProtos := make([]proto.Message, len(reqs))
	for i, r := range reqs {
		if !checkMatchesType(r, method.Input()) {
			panic("request type must match the input type of the procedure in the proto")
		}
		reqProtos[i] = r
	}

	s.protoDocsRequests = append(s.protoDocsRequests, protoDocsRequests{
		procedure: procedure,
		reqs:      reqProtos,
	})
}

// EnableDocsFirebaseAuth enables Firebase auth for the docs handler. The domain
// must be the auth domain of the Firebase project.
//
// Firebase credentials of the browser, generally set by running the actual web app
// locally, will be read and added as authorization headers in requests to the
// server.
func (s *Server) EnableDocsFirebaseAuth(domain string) {
	s.docsFirebaseDomain = domain
}

// Start starts the server, listening on the configured server address for requests
// based on its configuration. This method will block until program exit.
func (s *Server) Start(ctx context.Context) error {
	s.startCalled = true

	interceptors := append([]connect.ServerInterceptor{rpcLogger(), otel.ConnectServerInterceptor()}, s.serverInterceptors...)
	connectSrv := connect.NewServer(interceptors...)
	for _, reg := range s.serviceRegistrations {
		reg(connectSrv)
	}
	connecthttp.Mount(s.mux, connectSrv)

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
func Main[T config.CurioStack](conf T, confFiles fs.FS, run func(ctx context.Context, conf T, s *Server) error) int {
	ctx := context.Background()

	otel.Initialize(conf) // initialize as early as possible to instrument globals

	if err := config.Load(conf, confFiles); err != nil {
		slog.Error(fmt.Sprintf("Failed to load config: %v", err))
		return 1
	}

	logging.Initialize(&conf.GetCommon().Logging)

	s := &Server{
		mux: NewMux(),

		conf: conf.GetCommon(),
	}

	if err := run(ctx, conf, s); err != nil {
		slog.Error(fmt.Sprintf("Failed to configure server: %v", err))
		return 1
	}

	if !s.startCalled {
		slog.Error("Start was not called on server.Builder, it must be called in your run callback to start the server.")
		return 1
	}

	return 0
}

func (s *Server) mountDefaultEndpoints() error {
	docsDefined := false
	healthDefined := false
	// Define /internal/health if not already defined.
	_ = chi.Walk(s.mux, func(_, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
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
		for _, r := range s.protoDocsRequests {
			svc, _, _ := strings.Cut(r.procedure[1:], "/")
			if !slices.Contains(services, svc) {
				services = append(services, svc)
			}
		}
		if len(services) > 0 {
			var docopts []docshandler.Option
			protodocopts := make([]protodocs.Option, 0, len(s.protoDocsRequests)+len(services)-1)

			for _, r := range s.protoDocsRequests {
				protodocopts = append(protodocopts, protodocs.WithExampleRequests(r.procedure, r.reqs[0], r.reqs[1:]...))
			}

			for _, svc := range services[1:] {
				protodocopts = append(protodocopts, protodocs.WithAdditionalService(svc))
			}

			if s.docsFirebaseDomain != "" {
				script := docsFirebaseAuthScript(s.docsFirebaseDomain)
				docopts = append(docopts, docshandler.WithInjectedScriptSupplier(func() string {
					return script
				}))
			}

			docs, err := docshandler.New(protodocs.NewPlugin(services[0], protodocopts...), docopts...)
			if err != nil {
				return fmt.Errorf("server: create docs handler: %w", err)
			}
			s.mux.Handle("/internal/docs/*", http.StripPrefix("/internal/docs", docs))
		}
	}

	if !healthDefined {
		s.mux.Get("/internal/health", func(w http.ResponseWriter, _ *http.Request) {
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

func checkMatchesType(p any, desc protoreflect.Descriptor) bool {
	if r, ok := p.(proto.Message); ok {
		return r.ProtoReflect().Descriptor() == desc
	}
	return false
}

func rpcLogger() connect.ServerInterceptor {
	return func(next connect.ServerFunc) connect.ServerFunc {
		return func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
			svc, method := parseFullMethod(spec.Procedure)
			requestlog.AddExtraAttr(ctx, slog.String("rpc.service", svc))
			requestlog.AddExtraAttr(ctx, slog.String("rpc.method", method))

			var err error
			defer func() {
				grpcCode := 0
				if p := recover(); p != nil {
					defer panic(p)
					grpcCode = int(connect.CodeUnknown)
				} else if err != nil {
					grpcCode = int(connect.CodeOf(err))
					requestlog.AddExtraAttr(ctx, slog.String("error", err.Error()))
				}

				requestlog.AddExtraAttr(ctx, slog.Int("rpc.grpc.status_code", grpcCode))
			}()

			err = next(ctx, spec, stream)
			return err
		}
	}
}

// We assume a well formed method since we only use this from an interceptor.
// /grpc.service/method.
func parseFullMethod(m string) (string, string) {
	m = m[1:]
	pos := strings.LastIndexByte(m, '/')
	return m[:pos], m[pos+1:]
}
