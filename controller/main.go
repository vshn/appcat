package main

import (
	"fmt"
	"github.com/vshn/appcat-apiserver/controller/controller"
	"os"

	"github.com/urfave/cli/v2"
)

func main() {
	app := newApp()
	err := app.Run(os.Args)
	// If required flags aren't set, it will return with error before we could set up logging
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func newApp() *cli.App {
	app := &cli.App{
		Name:   "Controller",
		Usage:  "A Controller for VSHN PostgreSQL services",
		Action: controller.RunController,
		Before: setupLogging,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "metrics-addr",
				Value: ":8080",
				Usage: "The address the metric endpoint binds to.",
			},
			&cli.StringFlag{
				Name:  "health-addr",
				Value: ":8081",
				Usage: "The address the probe endpoint binds to.",
			},
			&cli.BoolFlag{
				Name:  "leader-elect",
				Value: false,
				Usage: "Enable leader election for controller manager. " +
					"Enabling this will ensure there is only one active controller manager.",
			},
			NewLogLevelFlag(),
			NewLogFormatFlag(),
		},
	}
	return app
}

func setupLogging(ctx *cli.Context) error {
	err := SetupLogging(ctx)
	if err != nil {
		return err
	}

	return LogMetadata(ctx)
}
