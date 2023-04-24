package main

import (
	"fmt"
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	"runtime"
	"strings"
	"time"
)

// LogMetadata prints various metadata to the root logger.
// It prints version, architecture and current user ID and returns nil.
func LogMetadata(c *cli.Context) error {
	log := logr.FromContextOrDiscard(c.Context)
	log.WithValues(
		"version", "v0.0.1",
		"date", time.Now(),
		"go_os", runtime.GOOS,
		"go_arch", runtime.GOARCH,
		"go_version", runtime.Version(),
		"uid", os.Getuid(),
		"gid", os.Getgid(),
	).Info("Starting up " + "PostgreSQL Controller")
	return nil
}

func SetupLogging(c *cli.Context) error {
	log, err := newZapLogger("PostgreSQL Controller", "v0.0.1", c.Int(NewLogLevelFlag().Name), usesProductionLoggingConfig(c))
	c.Context = logr.NewContext(c.Context, log)
	return err
}

func usesProductionLoggingConfig(c *cli.Context) bool {
	return strings.EqualFold("JSON", c.String(NewLogFormatFlag().Name))
}

func newZapLogger(name, version string, verbosityLevel int, useProductionConfig bool) (logr.Logger, error) {
	cfg := zap.NewDevelopmentConfig()
	cfg.EncoderConfig.ConsoleSeparator = " | "
	if useProductionConfig {
		cfg = zap.NewProductionConfig()
	}
	// Zap's levels get more verbose as the number gets smaller,
	// bug logr's level increases with greater numbers.
	cfg.Level = zap.NewAtomicLevelAt(zapcore.Level(verbosityLevel * -1))
	z, err := cfg.Build()
	if err != nil {
		return logr.Discard(), fmt.Errorf("error configuring the logging stack: %w", err)
	}
	zap.ReplaceGlobals(z)
	zlog := zapr.NewLogger(z).WithName(name)
	if useProductionConfig {
		// Append the version to each log so that logging stacks like EFK/Loki can correlate errors with specific versions.
		return zlog.WithValues("version", version), nil
	}
	return zlog, nil
}

func NewLogLevelFlag() *cli.IntFlag {
	return &cli.IntFlag{
		Name: "log-level", Aliases: []string{"v"}, EnvVars: []string{"LOG_LEVEL"},
		Usage: "number of the log level verbosity",
		Value: 0,
	}
}

func NewLogFormatFlag() *cli.StringFlag {
	return &cli.StringFlag{
		Name: "log-format", EnvVars: []string{"LOG_FORMAT"},
		Usage: "sets the log format (values: [json, console])",
		Value: "console",
		Action: func(context *cli.Context, format string) error {
			if format == "console" || format == "json" {
				return nil
			}
			_ = cli.ShowAppHelp(context)
			return fmt.Errorf("unknown log format: %s", format)
		},
	}
}
