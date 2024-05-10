package config

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"

	"github.com/curioswitch/go-curiostack/config/testdata/allfiles"
)

type fullConfig struct {
	Common
}

func TestLoadDefaults(t *testing.T) {
	tests := []struct {
		name string
		fs   fs.FS
		env  map[string]string

		address string
	}{
		{
			name:    "no config files",
			fs:      fstest.MapFS{},
			address: ":8080",
		},
		{
			name:    "no config files",
			fs:      nil,
			address: ":8080",
		},
		{
			name:    "all files, no env",
			fs:      allfiles.FS,
			address: ":local",
		},
		{
			name:    "all files, env not present",
			fs:      allfiles.FS,
			env:     map[string]string{"CONFIG_ENV": "staging"},
			address: ":nonlocal",
		},
		{
			name:    "all files, dev",
			fs:      allfiles.FS,
			env:     map[string]string{"CONFIG_ENV": "dev"},
			address: ":dev",
		},
		{
			name:    "all files, prod",
			fs:      allfiles.FS,
			env:     map[string]string{"CONFIG_ENV": "prod"},
			address: ":prod",
		},
		{
			name:    "all files, prod, env override",
			fs:      allfiles.FS,
			env:     map[string]string{"CONFIG_ENV": "prod", "SERVER_ADDRESS": ":env"},
			address: ":env",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			var conf fullConfig

			require.NoError(t, Load(&conf, tc.fs))
			require.Equal(t, tc.address, conf.Server.Address)

			// From repo root
			require.Equal(t, "curioswitch-dev", conf.Google.Project)
		})
	}
}
