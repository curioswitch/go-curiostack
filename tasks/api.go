package tasks

import (
	"os"

	"github.com/curioswitch/go-build"
	"github.com/goyek/goyek/v2"
	"github.com/goyek/x/cmd"
)

// DefineAPI defines tasks such as protobuf generation for API projects.
func DefineAPI() {
	build.RegisterLintTask(goyek.Define(goyek.Task{
		Name:  "format-proto",
		Usage: "Formats protobuf code.",
		Action: func(a *goyek.A) {
			cmd.Exec(a, "go tool buf format -w")
		},
	}))

	build.RegisterGenerateTask(goyek.Define(goyek.Task{
		Name:  "generate-proto",
		Usage: "Generates protobuf code.",
		Action: func(a *goyek.A) {
			cmd.Exec(a, "go tool buf generate")

			if err := os.MkdirAll("pb", 0o755); err != nil { //nolint:gosec
				a.Errorf("failed to create pb directory: %v", err)
			}
			cmd.Exec(a, "go tool buf build --as-file-descriptor-set -o ./descriptors/descriptorset.pb")
		},
	}))

	build.RegisterLintTask(goyek.Define(goyek.Task{
		Name:  "lint-proto",
		Usage: "Lints protobuf code.",
		Action: func(a *goyek.A) {
			cmd.Exec(a, "go tool buf lint")
		},
	}))
}
