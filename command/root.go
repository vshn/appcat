package command

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
)

var (
	logLevel  int
	logFormat string
	rootCmd   = &cobra.Command{
		Short:             "API Server for AppCat",
		PersistentPreRunE: setupLogging,
	}
)

func init() {
	rootCmd.PersistentFlags().IntVarP(&logLevel, "log-level", "v", 0, "Number of the log level verbosity.")
	rootCmd.PersistentFlags().StringVar(&logFormat, "log-format", "console", "Sets the log format (values: [json, console]).")
	viper.AutomaticEnv()
}

// Execute executes the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(controller, apiServerCmd)
}

func setupLogging(cmd *cobra.Command, _ []string) error {
	err := SetupLogging(cmd, logLevel, logFormat)
	if err != nil {
		return err
	}

	return LogMetadata(cmd)
}
