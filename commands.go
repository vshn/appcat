package main

import (
	xkube "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/urfave/cli/v2"
	appcatv1 "github.com/vshn/appcat-apiserver/apis/appcat/v1"
	"github.com/vshn/appcat-apiserver/apiserver/appcat"
	"github.com/vshn/appcat-apiserver/apiserver/vshn/postgres"
	"github.com/vshn/appcat-apiserver/controller"
	vshnv1 "github.com/vshn/component-appcat/apis/vshn/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog"
	"sigs.k8s.io/apiserver-runtime/pkg/builder"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
)

func runApiServer(ctx *cli.Context) error {
	appcatEnabled := ctx.Bool("appcat-handler")
	vshnBackupsEnabled := ctx.Bool("postgres-backup-handler")

	if !appcatEnabled && !vshnBackupsEnabled {
		klog.Fatal("Handlers are not enabled, please set at least one of APPCAT_HANDLER_ENABLED | VSHN_POSTGRES_BACKUP_HANDLER_ENABLED env variables to true")
	}

	b := builder.APIServer

	if appcatEnabled {
		b.WithResourceAndHandler(&appcatv1.AppCat{}, appcat.New())
	}

	if vshnBackupsEnabled {
		b.WithResourceAndHandler(&appcatv1.VSHNPostgresBackup{}, postgres.New())
	}

	b.WithoutEtcd().
		ExposeLoopbackAuthorizer().
		ExposeLoopbackMasterClientConfig()

	cmd, err := b.Build()
	if err != nil {
		klog.Fatal(err)
	}

	if err := cmd.Execute(); err != nil {
		klog.Fatal(err)
	}

	return nil
}

var s = runtime.NewScheme()

func init() {
	_ = corev1.SchemeBuilder.AddToScheme(s)
	_ = xkube.SchemeBuilder.AddToScheme(s)
	_ = vshnv1.SchemeBuilder.SchemeBuilder.AddToScheme(s)
}

// Run will run the controller mode of the composition function runner.
func runController(cli *cli.Context) error {

	log := logr.FromContextOrDiscard(cli.Context)

	ctrl.SetLogger(log)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 s,
		MetricsBindAddress:     cli.String("metrics-addr"),
		Port:                   9443,
		HealthProbeBindAddress: cli.String("health-addr"),
		LeaderElection:         cli.Bool("leader-elect"),
		LeaderElectionID:       "35t6u158.appcat.vshn.io",
	})
	if err != nil {
		return err
	}

	xpg := &controller.XPostgreSQLDeletionProtectionReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}

	err = xpg.SetupWithManager(mgr)
	if err != nil {
		return err
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return err
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return err
	}

	return mgr.Start(ctrl.SetupSignalHandler())
}
