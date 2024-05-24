package config

import (
	_ "embed"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/knadh/koanf/v2"
)

//go:embed config.yaml
var defaults []byte

// Server holds the configuration for the server.
type Server struct {
	// Address is the address the server will listen on, e.g. ":9080".
	// Defaults to ":8080".
	Address string `koanf:"address"`
}

// Google holds the configuration for using common GCP functionality.
type Google struct {
	// Project is the GCP project ID.
	Project string `koanf:"project"`

	// Region is the default region development resources are deployed to.
	Region string `koanf:"region"`
}

// Logging holds the configuration for logging.
type Logging struct {
	// Level is the [slog.Level] to use. Defaults to "info".
	Level string `koanf:"level"`

	// JSON indicates if logs should be output in JSON format.
	JSON bool `koanf:"json"`
}

// Common holds curiostack standard configuration objects. Server
// configuration objects should embed this and define their own
// fields on top of it.
type Common struct {
	// Server holds the configuration for the server.
	Server Server `koanf:"server"`

	// Google holds the configruation for using common GCP functionality.
	Google Google `koanf:"google"`

	// Logging holds the configuration for logging.
	Logging Logging `koanf:"logging"`
}

// Load resolves the configuration into the provided conf object. Config is merged
// in order from the following sources:
//
//  1. config.yaml embedded in this package. These are the curiostack defaults where applicable.
//  2. config.yaml at the base of the repository, identified by being next to go.work, if present.
//  3. config.yaml in the provided fs.FS if present.
//  4. config-local.yaml in the provided fs.FS if present and CONFIG_ENV is unset (local development).
//  5. config-nonlocal.yaml in the provided fs.FS if present and CONFIG_ENV is set.
//  6. config-${CONFIG_ENV}.yaml in the provided fs.FS if present and CONFIG_ENV is set.
//  7. Environment variables, where the config key is capitalized with '.' replaced with '_'.
func Load[T any](conf *T, confFiles fs.FS) error {
	k := koanf.NewWithConf(koanf.Conf{
		Delim:       ".",
		StrictMerge: true,
	})

	if err := k.Load(rawbytes.Provider(defaults), yaml.Parser()); err != nil {
		// Programming error, we are in control of the defaults.
		log.Fatalf("failed to load defaults: %v", err)
	}

	if goWorkDir := findGoWorkDir(); goWorkDir != "" {
		if err := loadIfPresent(k, os.DirFS(goWorkDir), ".curiostack.yaml"); err != nil {
			return err
		}
	}

	if confFiles != nil {
		if err := loadIfPresent(k, confFiles, "config.yaml"); err != nil {
			return err
		}

		confEnv := os.Getenv("CONFIG_ENV")
		if confEnv == "" {
			if err := loadIfPresent(k, confFiles, "config-local.yaml"); err != nil {
				return err
			}
		} else {
			if err := loadIfPresent(k, confFiles, "config-nonlocal.yaml"); err != nil {
				return err
			}
			if err := loadIfPresent(k, confFiles, fmt.Sprintf("config-%s.yaml", confEnv)); err != nil {
				return err
			}
		}
	}

	if err := k.Load(env.Provider("", ".", func(s string) string {
		return strings.ReplaceAll(strings.ToLower(s), "_", ".")
	}), nil); err != nil {
		return fmt.Errorf("config: failed to load env: %w", err)
	}

	if err := k.UnmarshalWithConf("", conf, koanf.UnmarshalConf{
		DecoderConfig: &mapstructure.DecoderConfig{
			Result:           conf,
			Squash:           true,
			WeaklyTypedInput: true,
		},
	}); err != nil {
		return fmt.Errorf("config: failed to unmarshal: %w", err)
	}

	return nil
}

func loadIfPresent(k *koanf.Koanf, confFiles fs.FS, name string) error {
	if _, err := fs.Stat(confFiles, name); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("config: failed to stat %s: %w", name, err)
	}

	b, err := fs.ReadFile(confFiles, name)
	if err != nil {
		return fmt.Errorf("config: failed to read %s: %w", name, err)
	}

	if err := k.Load(rawbytes.Provider(b), yaml.Parser()); err != nil {
		return fmt.Errorf("config: failed to load %s: %w", name, err)
	}

	return nil
}

func findGoWorkDir() string {
	dir, err := filepath.Abs(".")
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir || parent == "" {
			return ""
		}

		dir = parent
	}
}
