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
	apiserver "github.com/vshn/appcat-apiserver/apiserver/command"
	controller "github.com/vshn/appcat-apiserver/controller/command"
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
			apiserver.Command(),
			controller.Command(),
		},
		Flags: []cli.Flag{
			NewLogLevelFlag(),
			NewLogFormatFlag(),
		},
	}
	return app
}

func setupLogging(ctx *cli.Context) error {
	err := SetupLogging(ctx, ctx.Args().First())
	if err != nil {
		return err
	}

	return LogMetadata(ctx)
}
