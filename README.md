# CurioStack

Very full stack to help satisfy curiosity. The successor to https://github.com/curioswitch/curiostack, in Go.

A full example using the stack can be found in [tasuke](https://github.com/curioswitch/tasuke/tree/main/frontend/server).

## Components

### Build tasks

CurioStack defines uild tasks intended to be used with [go-build](https://github.com/curioswitch/go-build),
specific to server development.

- Docker image build and push via [ko](https://ko.build)
- Protobuf linting / generation via [buf](https://buf.build)

### Server framework

A lightweight [server framework](./server) intends to handle boiler plate common to all production-grade servers,
such as setting up observability and providing debug experiences for API development.

CurioStack uses [connect](https://connectrpc.com) for exposing API services from proto files. This example
sets up an RPC endpoint with Firebase authentication middleware, with support wired into the docs handler
served at `/internal/docs/`.

```go
//go:embed config/*.yaml
var confFiles embed.FS

func main() {
	os.Exit(server.Main(&config.Config{}, confFiles, setupServer))
}

func setupServer(ctx context.Context, conf *config.Config, s *server.Server) error {
	fbApp, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: conf.Google.Project})
	if err != nil {
		return fmt.Errorf("main: create firebase app: %w", err)
	}

	fbAuth, err := fbApp.Auth(ctx)
	if err != nil {
		return fmt.Errorf("main: create firebase auth client: %w", err)
	}

	firestore, err := fbApp.Firestore(ctx)
	if err != nil {
		return fmt.Errorf("main: create firestore client: %w", err)
	}
	defer firestore.Close()

	server.Mux(s).Use(middleware.Maybe(firebaseauth.NewMiddleware(fbAuth), func(r *http.Request) bool {
		return strings.HasPrefix(r.URL.Path, "/"+frontendapiconnect.FrontendServiceName+"/")
	}))

	saveUser := saveuser.NewHandler(firestore)
	server.HandleConnectUnary(s,
		frontendapiconnect.FrontendServiceSaveUserProcedure,
		saveUser.SaveUser,
		[]*frontendapi.SaveUserRequest{
			{
				User: &frontendapi.User{
					ProgrammingLanguageIds: []uint32{
						132, // golang
					},
					MaxOpenReviews: 5,
				},
			},
		},
	)

	server.EnableDocsFirebaseAuth(s)

	return server.Start(ctx, s)
}
```
