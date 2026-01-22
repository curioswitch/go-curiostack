package tasks

import (
	"fmt"
	"os"

	"github.com/curioswitch/go-build"
	"github.com/goyek/goyek/v3"
	"github.com/goyek/x/cmd"
)

// DefineAPI defines tasks such as protobuf generation for API projects.
func DefineAPI() {
	runBuf := "go run github.com/bufbuild/buf/cmd/buf@" + verBuf
	build.RegisterCommandDownloads(runBuf + " -h")

	build.RegisterFormatTask(goyek.Define(goyek.Task{
		Name:  "format-proto",
		Usage: "Formats protobuf code.",
		Action: func(a *goyek.A) {
			cmd.Exec(a, runBuf+" format -w")
		},
	}))

	build.RegisterGenerateTask(goyek.Define(goyek.Task{
		Name:  "generate-proto",
		Usage: "Generates protobuf code.",
		Action: func(a *goyek.A) {
			cmd.Exec(a, fmt.Sprintf("go run github.com/bufbuild/buf/cmd/buf@%s generate", verBuf))

			if err := os.MkdirAll("pb", 0o755); err != nil { //nolint:gosec
				a.Errorf("failed to create pb directory: %v", err)
			}
			cmd.Exec(a, runBuf+" build --as-file-descriptor-set -o ./descriptors/descriptorset.pb")
		},
	}))

	build.RegisterLintTask(goyek.Define(goyek.Task{
		Name:  "lint-proto",
		Usage: "Lints protobuf code.",
		Action: func(a *goyek.A) {
			cmd.Exec(a, runBuf+" lint")
		},
	}))
}
