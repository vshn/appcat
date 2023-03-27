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
	"os"
	"strconv"

	appcatv1 "github.com/vshn/appcat-apiserver/apis/appcat/v1"
	"github.com/vshn/appcat-apiserver/apiserver/appcat"
	"github.com/vshn/appcat-apiserver/apiserver/vshn/postgres"
	"k8s.io/klog"
	"sigs.k8s.io/apiserver-runtime/pkg/builder"
)

func main() {

	var appcatEnabled bool = true
	var vashnBackupsEnabled bool = false
	var err error

	appcatEnabled, err = strconv.ParseBool(os.Getenv("APPCAT_HANDLER_ENABLED"))
	if err != nil {
		klog.Fatal("Can't parse APPCAT_HANDLER_ENABLED env variable")
	}
	vashnBackupsEnabled, err = strconv.ParseBool(os.Getenv("VSHN_POSTGRES_BACKUP_HANDLER_ENABLED"))
	if err != nil {
		klog.Fatal("Can't parse APPCAT_HANDLER_ENABLED env variable")
	}

	if !appcatEnabled && !vashnBackupsEnabled {
		klog.Fatal("Handlers are not enabled, please set at least one of APPCAT_HANDLER_ENABLED | VSHN_POSTGRES_BACKUP_HANDLER_ENABLED env variables to True")
	}

	builder := builder.APIServer

	if appcatEnabled {
		builder.WithResourceAndHandler(&appcatv1.AppCat{}, appcat.New())
	}

	if vashnBackupsEnabled {
		builder.WithResourceAndHandler(&appcatv1.VSHNPostgresBackup{}, postgres.New())
	}

	builder.WithoutEtcd().
		ExposeLoopbackAuthorizer().
		ExposeLoopbackMasterClientConfig()

	cmd, err := builder.Build()
	if err != nil {
		klog.Fatal(err)
	}

	if err := cmd.Execute(); err != nil {
		klog.Fatal(err)
	}
}
