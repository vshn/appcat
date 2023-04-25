/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"github.com/urfave/cli/v2"
	"os"
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
		Name:   "AppCat API Server",
		Usage:  "This AppCat API Server help improve AppCat services",
		Before: setupLogging,
		Commands: []*cli.Command{
			apiserverCommand(),
			controllerCommand(),
		},
		Flags: []cli.Flag{
			NewLogLevelFlag(),
			NewLogFormatFlag(),
		},
	}
	return app
}

func controllerCommand() *cli.Command {
	return &cli.Command{
		Name:   "controller",
		Usage:  "A controller to manage PostgreSQL instance deletion",
		Action: runController,
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
		},
	}
}

func apiserverCommand() *cli.Command {
	return &cli.Command{
		Name:   "api-server",
		Usage:  "Start api-server",
		Action: runApiServer,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "appcat-handler",
				Usage:   "Enables AppCat handler for this Api Server",
				Value:   true,
				Aliases: []string{"a"},
				EnvVars: []string{"APPCAT_HANDLER_ENABLED"},
			},
			&cli.BoolFlag{
				Name:    "postgres-backup-handler",
				Usage:   "Enables PostgreSQL backup handler for this Api Server",
				Value:   false,
				Aliases: []string{"pb"},
				EnvVars: []string{"VSHN_POSTGRES_BACKUP_HANDLER_ENABLED"},
			},
		},
	}
}

func setupLogging(ctx *cli.Context) error {
	err := SetupLogging(ctx)
	if err != nil {
		return err
	}

	return LogMetadata(ctx)
}
