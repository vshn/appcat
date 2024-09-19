package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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
		cmd.APIServerCMD,
	)
}

func main() {

	cmd, _, err := rootCmd.Find(os.Args[1:])

	// default to functions if no cmd is given
	// necessary to properly support `crank render`
	// from https://github.com/spf13/cobra/issues/823#issuecomment-870027246
	if err == nil && cmd.Use == rootCmd.Use && cmd.Flags().Parse(os.Args[1:]) != pflag.ErrHelp {
		args := append([]string{"functions"}, os.Args[1:]...)
		rootCmd.SetArgs(args)
	}

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
		Use:               "appcat",
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
