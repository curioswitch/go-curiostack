package server

import (
	"context"
	"log/slog"
	"strings"

	"connectrpc.com/connect"
	"github.com/curioswitch/go-usegcp/middleware/requestlog"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	"github.com/curioswitch/go-curiostack/otel"
)

// HandleConnectUnary mounts a connect unary handler for the given procedure with the
// given handler. Sample requests will be displayed in the docs interface and is recommended
// to be provided whenever possible.
func HandleConnectUnary[Req any, Resp any](
	s *Server,
	procedure string,
	handler func(ctx context.Context, req *Req) (*Resp, error),
	sampleRequests []*Req,
	opts ...connect.HandlerOption,
) {
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

	var req Req
	if !checkMatchesType(&req, method.Input()) {
		panic("request type must match the input type of the procedure in the proto")
	}
	var resp Resp
	if !checkMatchesType(&resp, method.Output()) {
		panic("response type must match the output type of the procedure in the proto")
	}

	opts = append(ConnectHandlerOptions(), opts...)
	h := connect.NewUnaryHandler(
		procedure,
		func(ctx context.Context, r *connect.Request[Req]) (*connect.Response[Resp], error) {
			resp, err := handler(ctx, r.Msg)
			if err != nil {
				return nil, err
			}
			return connect.NewResponse(resp), nil
		},
		connect.WithSchema(svc),
		connect.WithHandlerOptions(opts...),
	)
	s.mux.Handle(procedure, h)

	if len(sampleRequests) > 0 {
		sampleReqs := make([]proto.Message, len(sampleRequests))
		for i, r := range sampleRequests {
			// We already verified Req matches the handler type defined in proto so know this
			// type cast works.
			sampleReqs[i] = any(r).(proto.Message) //nolint:forcetypeassert
		}
		s.protoDocsRequests = append(s.protoDocsRequests, protoDocsRequests{
			procedure: procedure,
			reqs:      sampleReqs,
		})
	}
}

func checkMatchesType(p any, desc protoreflect.Descriptor) bool {
	if r, ok := p.(proto.Message); ok {
		return r.ProtoReflect().Descriptor() == desc
	}
	return false
}

// ConnectHandlerOptions returns the default options for connect handlers.
func ConnectHandlerOptions() []connect.HandlerOption {
	return []connect.HandlerOption{
		connect.WithInterceptors(rpcLogger(), otel.ConnectInterceptor()),
	}
}

func rpcLogger() connect.Interceptor {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (res connect.AnyResponse, err error) {
			svc, method := parseFullMethod(req.Spec().Procedure)
			requestlog.AddExtraAttr(ctx, slog.String("rpc.service", svc))
			requestlog.AddExtraAttr(ctx, slog.String("rpc.method", method))

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

			res, err = next(ctx, req)
			return res, err
		}
	})
}

// We assume a well formed method since we only use this from an interceptor.
// /grpc.service/method.
func parseFullMethod(m string) (string, string) {
	m = m[1:]
	pos := strings.LastIndexByte(m, '/')
	return m[:pos], m[pos+1:]
}
