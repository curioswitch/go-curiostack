package logging

import (
	"log/slog"
	"os"
	"strings"

	"github.com/curioswitch/go-usegcp/gcpslog"

	"github.com/curioswitch/go-curiostack/config"
)

// Initialize initalizes logging for the given configuration, setting the
// default slog handler. JSON should always be set to true in cloud
// deployments.
func Initialize(conf *config.Logging) {
	var level slog.Level
	switch strings.ToLower(conf.Level) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	case "info":
		fallthrough
	default:
		level = slog.LevelInfo
	}

	var h slog.Handler
	if conf.JSON {
		h = gcpslog.NewHandler(os.Stderr, gcpslog.Level(level))
	} else {
		h = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	}

	slog.SetDefault(slog.New(h))
}
