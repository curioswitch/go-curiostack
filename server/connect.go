package server

import (
	"context"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	"github.com/curioswitch/go-curiostack/otel"
)

// HandleConnectUnary mounts a connect unary handler for the given procedure with the
// given handler. Sample requests will be displayed in the docs interface and is recommended
// to be provided whenever possible. The type of the sample requests must match the type of
// the request parameter of the handler.
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

	method := svc.(protoreflect.ServiceDescriptor).Methods().ByName(protoreflect.Name(methodName))
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
			sampleReqs[i] = any(r).(proto.Message)
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
		connect.WithInterceptors(otel.ConnectInterceptor()),
	}
}
