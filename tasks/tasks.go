package tasks

import (
	"github.com/curioswitch/go-build"
	"github.com/goyek/goyek/v2"
	"github.com/goyek/x/cmd"
)

func Define(opts ...build.Option) {
	build.DefineTasks(opts...)

	goyek.Define(goyek.Task{
		Name:  "start",
		Usage: "Starts the local server.",
		Action: func(a *goyek.A) {
			cmd.Exec(a, "go run .")
		},
	})
}
