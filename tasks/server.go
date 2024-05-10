package tasks

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/goyek/goyek/v2"
	"github.com/goyek/x/cmd"

	"github.com/curioswitch/go-curiostack/config"
)

type curiostackConfig struct {
	config.Common
}

// DefineServer defines tasks for server projects.
func DefineServer(opts ...ServerOption) {
	dockerTags := flag.String("docker-tags", "dev", "Tags to add to add to built docker image.")
	dockerLabels := flag.String("docker-labels", "", "Labels to add to add to built docker image.")

	var conf serverConfig
	for _, o := range opts {
		o.apply(&conf)
	}
	if err := config.Load(&conf.curiostackConfig, nil); err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	goyek.Define(goyek.Task{
		Name:  "docker",
		Usage: "Builds the server docker image to local.",
		Action: func(a *goyek.A) {
			cmd.Exec(a, koCmd(*dockerTags, *dockerLabels), cmd.Env("KO_DOCKER_REPO", "ko.local"))
		},
	})

	goyek.Define(goyek.Task{
		Name:  "push",
		Usage: "Builds and pushes the server docker image.",
		Action: func(a *goyek.A) {
			cmd.Exec(a, koCmd(*dockerTags, *dockerLabels), cmd.Env("KO_DOCKER_REPO", fullDockerRepo(&conf)))
		},
	})

	goyek.Define(goyek.Task{
		Name:  "start",
		Usage: "Starts the local server.",
		Action: func(a *goyek.A) {
			cmd.Exec(a, "go run .")
		},
	})
}

type serverConfig struct {
	serviceName string
	dockerRepo  string

	curiostackConfig
}

// ServerOption is a configuration option for DefineServer.
type ServerOption interface {
	apply(conf *serverConfig)
}

// ServiceName returns a ServerOption to indicate the name of the service,
// used in places such as the name of the docker image.
//
// If not provided, an attempt will be made to infer by
//
//   - `parentDir-currentDir` if the current directory is named `server`
//   - `currentDir` otherwise
func ServiceName(serviceName string) ServerOption {
	return &serviceNameOption{serviceName: serviceName}
}

type serviceNameOption struct {
	serviceName string
}

func (o *serviceNameOption) apply(conf *serverConfig) {
	conf.serviceName = o.serviceName
}

// DockerRepo returns a ServerOption to indicate the docker repository to push to.
// If unset, the `docker` repository within the artifact registry of the configured
// Google project will be used.
func DockerRepo(dockerRepo string) ServerOption {
	return &dockerRepoOption{dockerRepo: dockerRepo}
}

type dockerRepoOption struct {
	dockerRepo string
}

func (o *dockerRepoOption) apply(conf *serverConfig) {
	conf.dockerRepo = o.dockerRepo
}

func serviceName(conf *serverConfig) string {
	if conf.serviceName != "" {
		return conf.serviceName
	}

	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("failed to get current directory: %v", err)
	}
	cur := filepath.Base(wd)
	if cur != "server" {
		return cur
	}

	parent := filepath.Base(filepath.Dir(wd))
	return fmt.Sprintf("%s-%s", parent, cur)
}

func fullDockerRepo(conf *serverConfig) string {
	repoBase := conf.dockerRepo
	if repoBase == "" {
		repoBase = fmt.Sprintf("%s-docker.pkg.dev/%s/docker", conf.Google.Region, conf.Google.Project)
	}
	svc := serviceName(conf)
	return fmt.Sprintf("%s/%s", repoBase, svc)
}

func koCmd(dockerTags string, dockerLabels string) string {
	labelsStr := ""
	if dockerLabels != "" {
		labelsStr = "--image-label " + dockerLabels
	}
	return fmt.Sprintf("go run github.com/google/ko@%s build --bare --tags %s %s .", verKo, dockerTags, labelsStr)
}
