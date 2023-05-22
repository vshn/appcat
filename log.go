package main

import (
	"fmt"
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	"runtime"
	"strings"
	"time"
)

// LogMetadata prints various metadata to the root logger.
// It prints version, architecture and current user ID and returns nil.
func LogMetadata(cmd *cobra.Command) error {
	log := logr.FromContextOrDiscard(cmd.Context())
	log.WithValues(
		"version", "v0.0.1",
		"date", time.Now(),
		"go_os", runtime.GOOS,
		"go_arch", runtime.GOARCH,
		"go_version", runtime.Version(),
		"uid", os.Getuid(),
		"gid", os.Getgid(),
	).Info("Starting up " + cmd.Short)
	return nil
}

func SetupLogging(cmd *cobra.Command, logLevel int, logFormat string) error {
	isJson := strings.EqualFold("JSON", logFormat)
	log, err := newZapLogger(strings.ToUpper(cmd.Use), logLevel, isJson)
	cmd.SetContext(logr.NewContext(cmd.Context(), log))
	return err
}

func newZapLogger(name string, verbosityLevel int, useProductionConfig bool) (logr.Logger, error) {
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
	return zlog, nil
}
