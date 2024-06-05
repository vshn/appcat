package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vshn/appcat/v4/cmd"
)

func init() {
	rootCmd.AddCommand(
		cmd.ControllerCMD,
		cmd.SLIProberCMD,
		cmd.FunctionCMD,
		cmd.MaintenanceCMD,
		cmd.SlareportCMD,
	)
}

func main() {

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var (
	logLevel  int
	logFormat string
	rootCmd   = &cobra.Command{
		Short:             "AppCat",
		Long:              "AppCat controllers, api servers, grpc servers and probers",
		PersistentPreRunE: setupLogging,
	}
)

func init() {
	rootCmd.PersistentFlags().IntVarP(&logLevel, "log-level", "v", 0, "Number of the log level verbosity.")
	rootCmd.PersistentFlags().StringVar(&logFormat, "log-format", "console", "Sets the log format (values: [json, console]).")
	viper.AutomaticEnv()
}

func setupLogging(cmd *cobra.Command, _ []string) error {
	err := SetupLogging(cmd, logLevel, logFormat)
	if err != nil {
		return err
	}

	return LogMetadata(cmd)
}
